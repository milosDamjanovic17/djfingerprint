package dsp

import (
	"fmt"
	"math"
	"math/cmplx"

	"github.com/milosDamjanovic17/djfingerprint/internal/audio"
	"gonum.org/v1/gonum/dsp/fourier"
)

const (

	// WindowSize is the number of samples per FFT window, Must be power of 2 for FFT efficiency.
	WindowSize = 1024

	// Hopsize is the number of samples between successive windows, 512 = 50% overlap - standard for continuity between frames.
	HopSize = 512

	// SampleRate is the expected input sample rate from the audio pipeline
	// Matches the rate from normalizer.go
	SampleRate = 11025

	// FreqMin is the lower bound of the frequency range we care about in Hz
	FreqMin = 20

	// FreqMax is the upper bound of the frequency range we car about in Hz
	FreqMax = 5000

	// NumBins is the number of usable FFT output bins
	// For a real-valued FFT of size N, usable bins = N/2 + 1 (DC to Nyquist)
	NumBins = WindowSize/2 + 1

)

var (
	// BinStart is the first FFT bin index that falls within FreqMin.
	// Calculated once at package init and reused across all calls.
	// bin = floor(freq * WindowSize / SampleRate)
	BinStart = freqToBin(FreqMin)

	// BinEnd is the last FFT bin index that falls within FreqMax.
	BinEnd = freqToBin(FreqMax)
)

// Frame represents one time slice of the spectrogram.
// It contains the magnitude of each frequency bin within the
// restricted range [BinStart, BinEnd] for a single window of audio.
type Frame struct {
	// Magnitudes holds the FFT magnitude for each bin in [BinStart, BinEnd].
	// Index 0 corresponds to BinStart, index len-1 corresponds to BinEnd.
	Magnitudes []float64

	// Index is the position of this frame in the full spectrogram sequence.
	// Frame 0 is the first window, Frame 1 is the second, and so on.
	Index int

	// TimeSeconds is the center time of this frame in seconds,
	// calculated from the frame index, hop size, and sample rate.
	TimeSeconds float64
}

// Spectrogram is the full 2D time-frequency representation of the audio.
// It is a slice of Frames — one Frame per sliding window position.
type Spectrogram struct {
	// Frames holds one Frame per sliding window position
	Frames []Frame

	// NumFrames is the total number of frames in the spectrogram
	NumFrames int

	// BinsPerFrame is the number of frequency bins per frame
	// Equal to BinEnd - BinStart
	BinsPerFrame int
}

// Compute takes normalized PCM audio and produces a Spectrogram.
//
// The process for each sliding window position:
//  1. Extract a WindowSize slice of samples
//  2. Apply the Hamming window function to reduce spectral leakage
//  3. Run FFT to convert from time domain to frequency domain
//  4. Calculate magnitude of each complex FFT output bin
//  5. Store only the bins within [FreqMin, FreqMax]
//
// The input PCMData must have SampleRate == 11025 and Channels == 1.
// These are guaranteed by the audio pipeline (decoder + normalizer).
func Compute(pcm *audio.PCMData) (*Spectrogram, error) {
	if pcm == nil {
		return nil, fmt.Errorf("spectrogram: input PCMData is nil")
	}

	if len(pcm.Samples) == 0 {
		return nil, fmt.Errorf("spectrogram: input PCMData has no samples")
	}

	if len(pcm.Samples) < WindowSize {
		return nil, fmt.Errorf(
			"spectrogram: audio too short — need at least %d samples, got %d",
			WindowSize, len(pcm.Samples),
		)
	}

	if pcm.SampleRate != SampleRate {
		return nil, fmt.Errorf(
			"spectrogram: unexpected sample rate %d, expected %d",
			pcm.SampleRate, SampleRate,
		)
	}

	// Pre-compute the Hamming window coefficients once.
	// These are the same for every frame so we calculate them once
	// and reuse them rather than recalculating per frame.
	hammingCoeffs := buildHammingWindow(WindowSize)

	// Calculate how many frames the sliding window will produce.
	// frames = floor((totalSamples - WindowSize) / HopSize) + 1
	numFrames := (len(pcm.Samples)-WindowSize)/HopSize + 1
	binsPerFrame := BinEnd - BinStart

	frames := make([]Frame, 0, numFrames)

	// Initialise the gonum FFT plan once and reuse it across all frames.
	// Creating a new FFT plan per frame would be expensive.
	fft := fourier.NewFFT(WindowSize)

	// windowed holds the float64 samples for the current window
	// after the Hamming coefficients have been applied.
	// Allocated once and reused each iteration.
	windowed := make([]float64, WindowSize)

	for i := 0; i < numFrames; i++ {
		// Calculate the start sample index for this window
		start := i * HopSize
		end := start + WindowSize
		_ = end // guard: ensure we do not read past the end of the sample slice & ignore unused variable warning

		// Apply the Hamming window to this segment of samples.
		// Each sample is multiplied by its corresponding Hamming coefficient,
		// scaling the edges toward zero and preserving the center.
		for j := 0; j < WindowSize; j++ {
			windowed[j] = float64(pcm.Samples[start+j]) * hammingCoeffs[j]
		}

		// Run the FFT on the windowed segment.
		// Coefficients returns the complex FFT output.
		// For real input of size N, output has N/2+1 complex values.
		complexOut := fft.Coefficients(nil, windowed)

		// Extract the magnitude of each bin in our frequency range.
		// magnitude = sqrt(real² + imag²)
		magnitudes := make([]float64, binsPerFrame)
		for binIdx := 0; binIdx < binsPerFrame; binIdx++ {
			bin := BinStart + binIdx
			// Guard: ensure we do not read past the FFT output slice
			if bin >= len(complexOut) {
				break
			}
			magnitudes[binIdx] = cmplx.Abs(complexOut[bin])
		}

		// Calculate the center time of this frame in seconds.
		// The center sample of window i is at: i*HopSize + WindowSize/2
		centerSample := float64(i*HopSize) + float64(WindowSize)/2.0
		timeSeconds := centerSample / float64(SampleRate)

		frames = append(frames, Frame{
			Magnitudes:  magnitudes,
			Index:       i,
			TimeSeconds: timeSeconds,
		})
	}

	return &Spectrogram{
		Frames:       frames,
		NumFrames:    len(frames),
		BinsPerFrame: binsPerFrame,
	}, nil
}

// buildHammingWindow returns a slice of WindowSize Hamming coefficients.
//
// The Hamming window function is:
//
//	w(n) = 0.54 - 0.46 * cos(2π * n / (N - 1))
//
// Where n is the sample index and N is the window size.
// The result is a smooth bell curve — near zero at both edges,
// near 1.0 at the center — that eliminates the hard cut artifacts
// that cause spectral leakage in FFT analysis.
func buildHammingWindow(size int) []float64 {
	window := make([]float64, size)
	for n := 0; n < size; n++ {
		window[n] = 0.54 - 0.46*math.Cos(2*math.Pi*float64(n)/float64(size-1))
	}
	return window
}

// freqToBin converts a frequency in Hz to the nearest FFT bin index.
//
//	bin = floor(freq * WindowSize / SampleRate)
func freqToBin(freqHz float64) int {
	return int(math.Floor(freqHz * float64(WindowSize) / float64(SampleRate)))
}

// BinToFreq converts an FFT bin index back to its centre frequency in Hz.
// This is the inverse of freqToBin and is used in tests and diagnostics.
//
//	freq = bin * SampleRate / WindowSize
func BinToFreq(bin int) float64 {
	return float64(bin) * float64(SampleRate) / float64(WindowSize)
}