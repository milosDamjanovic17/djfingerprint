package audio

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"os/exec"
	"time"
)

// PCMData holds the decoded audio in a normalized, consistent format.
// Regardless of the input file format (MP3, WAV, OGG, WebM, FLAC),
// the output is always:
//   - Mono (1 channel)
//   - 11025 Hz sample rate
//   - Signed 16-bit little-endian PCM samples

type PCMData struct {
	Samples []int16
	SampleRate int
	Channels int
	Duration time.Duration
}

// SampleCount returns the total number of samples in the PCM data
func (p *PCMData) SampleCount() int {
	return len(p.Samples)
}

// DurationSeconds returns the audio duration as a float64 in seconds
func (p *PCMData) DurationSeconds() float64 {
	return float64(len(p.Samples)) / float64(p.SampleRate)
}

// DecodeFile takes a path to any audio file supported by FFmpeg and
// returns decoded PCM data ready for the fingerprinting pipeline.
//
// The file is decoded to:
//   - 1 channel (mono)
//   - 11025 Hz sample rate
//   - Signed 16-bit little-endian PCM
//
// FFmpeg must be installed and accessible via PATH or the ffmpegPath
// argument must point to the binary directly.
func DecodeFile(filePath string, ffmpegPath string) (*PCMData, error) {
	// verify that the file actually exists before invoking FFmpeg
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file does not exist: %s", filePath)
	}

	// If ffmpegPath is empty, assume "ffmpeg" is in the system PATH
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg" // assume it's in PATH
	}

	// Build the FFmpeg command
	// Flags explained:
	//   -i filePath         input file
	//   -ac 1               mix down to mono (1 audio channel)
	//   -ar 11025           resample to 11025 Hz
	//   -f s16le            output format: signed 16-bit little-endian PCM
	//   -acodec pcm_s16le   explicitly set the audio codec to match
	//   -vn                 disable any video stream (some MP4/WebM have video)
	//   -loglevel error     suppress FFmpeg's verbose output, only show errors
	//   pipe:1              write output to stdout instead of a file
	cmd := exec.Command(
		ffmpegPath,
		"-i", filePath,
		"-ac", "1",
		"-ar", "11025",
		"-f", "s16le",
		"-acodec", "pcm_s16le",
		"-vn",
		"-loglevel", "error",
		"pipe:1",
	)

	// capture the stdout (raw PCM data) and stderr (for error messages)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create get stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// start the FFmpeg process
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start FFmpeg: %w", err)
	}

	// read all raw PCM bytes from stdout
	rawBytes, err := io.ReadAll(stdout)
	if err != nil {
		return nil, fmt.Errorf("failed to read ffmpeg output: %w", err)
	}

	// Read any error output from FFmpeg
	stderrBytes, err := io.ReadAll(stderr)
	if err != nil {
		return nil, fmt.Errorf("failed to read ffmpeg : %w", err)
	}

	// wait for FFmpeg to finish anc check for exit status
	if err := cmd.Wait(); err != nil {
		// Include FFmpeg's stderr output in the error message for better debugging
		if len(stderrBytes) > 0 {
			return nil, fmt.Errorf("ffmpeg failed: %w\nffmpeg error output: %s", err, string(stderrBytes))
		}
		return nil, fmt.Errorf("ffmpeg process failed: %w", err)
	}

	// if FFmpeg succeeded but produced no output, the file is likely silent, empty, or in format FFmpeg can't decode
	if len(rawBytes) == 0 {
		return nil, fmt.Errorf("ffmpeg produced no output for file: %s", filePath)
	}

	// s16le format: each sample is exactly 2 bytes
	// total sample count = total bytes / 2
	if len(rawBytes)%2 != 0 {
		return nil, fmt.Errorf("unexpected PCM byte count %d: not a multiple of 2", len(rawBytes))
	}

	// Convert the raw []byte to []int16
	// Each pair of bytes is one 16-bit sample in little-endian order
	sampleCount := len(rawBytes) / 2
	samples := make([]int16, sampleCount)

	for i := range samples {
		// binary.LittleEndian.Uint16 reads 2 bytes as an unsigned 16-bit integer
		// We must cast it to int16 to get the signed value FFmpeg produced
		samples[i] = int16(binary.LittleEndian.Uint16(rawBytes[i*2 : i*2+2]))
	}

	const sampleRate = 11025
	const channels = 1

	// Calculate duration from the number of samples and the sample rate
	// Duration = samples / samples_per_second
	durationSeconds := float64(sampleCount) / float64(sampleRate)
	duration := time.Duration(durationSeconds * float64(time.Second))

	return &PCMData {
		Samples: samples,
		SampleRate: sampleRate,
		Channels: channels,
		Duration: duration,
	}, nil
}

	// DecodeBytes takes raw audio data as a byte slice (e.g. from a browser
	// upload that was read into memory) writes it to a temporary file,
	// decodes it via DecodeFile, and cleans up the temp file.
	//
	// This is used by the HTTP identify endpoint where the audio arrives
	// as a multipart upload body rather than an on-disk file.
func DecodeBytes(data []byte, ffmpegPath string) (*PCMData, error) {
	if len(data) == 0 {
		return nil, fmt.Errorf("audio data is empty")
	}

	// Write to a temp file so FFmpeg can read it
	// FFmpeg needs a seekable input for most formats
	tmpFile, err := os.CreateTemp("", "djfp-audio-*.bin")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp file: %w", err)
	}

	// temp file cleanup when the function returns
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, fmt.Errorf("failed to write audio data to temp file: %w", err)
	}

	// Close before passing path to FFmpeg
	// FFmpeg will open it independently
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("failed to close temp file: %w", err)
	}

	return DecodeFile(tmpFile.Name(), ffmpegPath)
}

