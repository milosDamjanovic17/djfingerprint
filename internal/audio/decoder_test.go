package audio

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
	"fmt"
)

// ffmpegAvailable checks whether FFmpeg is installed and accessible.
// Tests that require FFmpeg are skipped if it is not found.
// This prevents CI failures on machines without FFmpeg installed.
func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// generateSineWAV creates a real WAV file containing a pure sine wave
// at the given frequency and duration using FFmpeg.
// This gives us a real audio file to test against without needing
// to ship binary test fixtures in the repository.
func generateSineWAV(t *testing.T, path string, freqHz int, durationSec float64) {
	t.Helper()

	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available, skipping test")
	}

	cmd := exec.Command(
		"ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("sine=frequency=%d:duration=%f", freqHz, durationSec),
		"-ar", "44100",
		"-ac", "1",
		"-y",         // overwrite if exists
		"-loglevel", "error",
		path,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate test WAV: %v\noutput: %s", err, out)
	}
}

// generateSilentWAV creates a WAV file containing only silence
func generateSilentWAV(t *testing.T, path string, durationSec float64) {
	t.Helper()

	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available, skipping test")
	}

	cmd := exec.Command(
		"ffmpeg",
		"-f", "lavfi",
		"-i", fmt.Sprintf("anullsrc=r=44100:cl=mono:d=%f", durationSec),
		"-t", fmt.Sprintf("%f", durationSec),
		"-y",
		"-loglevel", "error",
		path,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("failed to generate silent WAV: %v\noutput: %s", err, out)
	}
}

// -----------------------------------------------------------------------
// DecodeFile Tests
// -----------------------------------------------------------------------

func TestDecodeFile_FileNotFound(t *testing.T) {
	_, err := DecodeFile("/nonexistent/path/audio.mp3", "ffmpeg")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestDecodeFile_EmptyPath(t *testing.T) {
	_, err := DecodeFile("", "ffmpeg")
	if err == nil {
		t.Fatal("expected error for empty file path, got nil")
	}
}

func TestDecodeFile_FFmpegNotFound(t *testing.T) {
	// Create a real temp file so the file-exists check passes
	tmpFile, err := os.CreateTemp("", "test-audio-*.wav")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	_, err = DecodeFile(tmpFile.Name(), "/nonexistent/ffmpeg")
	if err == nil {
		t.Fatal("expected error when ffmpeg binary does not exist, got nil")
	}
}

func TestDecodeFile_WAV_SampleRate(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_440hz.wav")
	generateSineWAV(t, wavPath, 440, 3.0)

	pcm, err := DecodeFile(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	// Output sample rate must always be 11025 regardless of input rate
	if pcm.SampleRate != 11025 {
		t.Errorf("expected SampleRate=11025, got %d", pcm.SampleRate)
	}
}

func TestDecodeFile_WAV_Mono(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_mono.wav")
	generateSineWAV(t, wavPath, 440, 3.0)

	pcm, err := DecodeFile(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	// Output must always be mono regardless of input channel count
	if pcm.Channels != 1 {
		t.Errorf("expected Channels=1, got %d", pcm.Channels)
	}
}

func TestDecodeFile_WAV_SampleCount(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_duration.wav")

	const durationSec = 5.0
	generateSineWAV(t, wavPath, 440, durationSec)

	pcm, err := DecodeFile(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	// Expected samples = 11025 * 5 = 55125
	// Allow 5% tolerance for resampling rounding
	expectedSamples := 11025 * durationSec
	tolerance := expectedSamples * 0.05

	actual := float64(pcm.SampleCount())
	if actual < expectedSamples-tolerance || actual > expectedSamples+tolerance {
		t.Errorf(
			"expected ~%.0f samples (±5%%), got %d",
			expectedSamples, pcm.SampleCount(),
		)
	}
}

func TestDecodeFile_WAV_Duration(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_dur.wav")

	const durationSec = 4.0
	generateSineWAV(t, wavPath, 440, durationSec)

	pcm, err := DecodeFile(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	expectedDur := time.Duration(durationSec * float64(time.Second))
	tolerance := 200 * time.Millisecond

	diff := pcm.Duration - expectedDur
	if diff < 0 {
		diff = -diff
	}

	if diff > tolerance {
		t.Errorf(
			"expected duration ~%v (±200ms), got %v",
			expectedDur, pcm.Duration,
		)
	}
}

func TestDecodeFile_SilentAudio_NoError(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	silentPath := filepath.Join(dir, "silent.wav")
	generateSilentWAV(t, silentPath, 3.0)

	// Silent audio must decode without errors
	// The normalizer (not the decoder) handles the all-zeros case
	pcm, err := DecodeFile(silentPath, "ffmpeg")
	if err != nil {
		t.Fatalf("silent audio should decode without error, got: %v", err)
	}

	if pcm.SampleCount() == 0 {
		t.Error("expected non-zero sample count for silent audio")
	}
}

func TestDecodeFile_CorruptFile_ReturnsError(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	corruptPath := filepath.Join(dir, "corrupt.mp3")

	// Write random non-audio bytes to simulate a corrupt file
	err := os.WriteFile(corruptPath, []byte("this is not audio data at all"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	_, err = DecodeFile(corruptPath, "ffmpeg")
	if err == nil {
		t.Fatal("expected error for corrupt audio file, got nil")
	}
}

func TestDecodeFile_SamplesNotEmpty(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_samples.wav")
	generateSineWAV(t, wavPath, 440, 2.0)

	pcm, err := DecodeFile(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeFile failed: %v", err)
	}

	if len(pcm.Samples) == 0 {
		t.Error("expected non-empty samples slice")
	}

	// For a 440Hz sine wave, not all samples should be zero
	allZero := true
	for _, s := range pcm.Samples {
		if s != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		t.Error("all samples are zero for a non-silent sine wave input")
	}
}

// -----------------------------------------------------------------------
// DecodeBytes Tests
// -----------------------------------------------------------------------

func TestDecodeBytes_EmptyInput(t *testing.T) {
	_, err := DecodeBytes([]byte{}, "ffmpeg")
	if err == nil {
		t.Fatal("expected error for empty byte slice, got nil")
	}
}

func TestDecodeBytes_ValidWAVBytes(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_bytes.wav")
	generateSineWAV(t, wavPath, 440, 2.0)

	// Read the file into bytes to simulate an in-memory upload
	data, err := os.ReadFile(wavPath)
	if err != nil {
		t.Fatal(err)
	}

	pcm, err := DecodeBytes(data, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeBytes failed: %v", err)
	}

	if pcm.SampleRate != 11025 {
		t.Errorf("expected SampleRate=11025, got %d", pcm.SampleRate)
	}

	if pcm.Channels != 1 {
		t.Errorf("expected Channels=1, got %d", pcm.Channels)
	}

	if len(pcm.Samples) == 0 {
		t.Error("expected non-empty samples from DecodeBytes")
	}
}

func TestDecodeBytes_CorruptBytes(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available")
	}

	_, err := DecodeBytes([]byte("not audio at all"), "ffmpeg")
	if err == nil {
		t.Fatal("expected error for corrupt bytes, got nil")
	}
}

// -----------------------------------------------------------------------
// PCMData Method Tests
// -----------------------------------------------------------------------

func TestPCMData_SampleCount(t *testing.T) {
	pcm := &PCMData{
		Samples:    make([]int16, 11025),
		SampleRate: 11025,
		Channels:   1,
	}

	if pcm.SampleCount() != 11025 {
		t.Errorf("expected SampleCount=11025, got %d", pcm.SampleCount())
	}
}

func TestPCMData_DurationSeconds(t *testing.T) {
	pcm := &PCMData{
		Samples:    make([]int16, 11025),
		SampleRate: 11025,
		Channels:   1,
	}

	// 11025 samples at 11025 Hz = exactly 1 second
	dur := pcm.DurationSeconds()
	if dur != 1.0 {
		t.Errorf("expected DurationSeconds=1.0, got %f", dur)
	}
}

// -----------------------------------------------------------------------
// Benchmark
// -----------------------------------------------------------------------

func BenchmarkDecodeFile_30Seconds(b *testing.B) {
	if !ffmpegAvailable() {
		b.Skip("FFmpeg not available")
	}

	dir := b.TempDir()
	wavPath := filepath.Join(dir, "bench_30s.wav")

	cmd := exec.Command(
		"ffmpeg",
		"-f", "lavfi",
		"-i", "sine=frequency=440:duration=30",
		"-ar", "44100",
		"-ac", "2",   // stereo input to test downmix
		"-y",
		"-loglevel", "error",
		wavPath,
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		b.Fatalf("failed to generate benchmark WAV: %v\n%s", err, out)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := DecodeFile(wavPath, "ffmpeg")
		if err != nil {
			b.Fatalf("DecodeFile failed during benchmark: %v", err)
		}
	}
}