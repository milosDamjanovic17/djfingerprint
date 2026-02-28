package dsp

import (
	"math"
	"testing"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/audio"
)

const int16max = 32767

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makePCM builds a PCMData directly from a sample slice.
// SampleRate is always 11025 and Channels always 1 to match
// the guaranteed output of the audio pipeline.
func makePCM(samples []int16) *audio.PCMData {
	return &audio.PCMData{
		Samples:    samples,
		SampleRate: 11025,
		Channels:   1,
		Duration:   time.Duration(len(samples)) * time.Second / 11025,
	}
}

// makeSineWave generates a pure sine wave at the given frequency
// with the given number of samples, amplitude scaled to int16 range.
// Used to test that FFT peaks appear at the correct frequency bins.
func makeSineWave(freqHz float64, numSamples int) []int16 {
	samples := make([]int16, numSamples)
	for i := range samples {
		// sin oscillates -1.0 to +1.0, scale to 90% of int16max
		val := math.Sin(2 * math.Pi * freqHz * float64(i) / float64(SampleRate))
		samples[i] = int16(val * float64(int16max) * 0.9)
	}
	return samples
}

// dominantBin returns the index of the bin with the highest magnitude
// within a Frame's Magnitudes slice.
func dominantBin(f Frame) int {
	maxMag := -1.0
	maxIdx := 0
	for i, m := range f.Magnitudes {
		if m > maxMag {
			maxMag = m
			maxIdx = i
		}
	}
	return maxIdx
}

// -----------------------------------------------------------------------
// Compute — Nil and Invalid Input
// -----------------------------------------------------------------------

func TestCompute_NilInput(t *testing.T) {
	_, err := Compute(nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
}

func TestCompute_EmptySamples(t *testing.T) {
	_, err := Compute(makePCM([]int16{}))
	if err == nil {
		t.Fatal("expected error for empty samples, got nil")
	}
}

func TestCompute_TooFewSamples(t *testing.T) {
	// Fewer samples than one full window — cannot produce any frame
	samples := make([]int16, WindowSize-1)
	_, err := Compute(makePCM(samples))
	if err == nil {
		t.Fatal("expected error for samples fewer than WindowSize, got nil")
	}
}

func TestCompute_WrongSampleRate(t *testing.T) {
	pcm := &audio.PCMData{
		Samples:    make([]int16, WindowSize*4),
		SampleRate: 44100, // wrong — pipeline always outputs 11025
		Channels:   1,
	}
	_, err := Compute(pcm)
	if err == nil {
		t.Fatal("expected error for wrong sample rate, got nil")
	}
}

// -----------------------------------------------------------------------
// Compute — Frame Count
// -----------------------------------------------------------------------

func TestCompute_FrameCount_ExactWindow(t *testing.T) {
	// Exactly one window worth of samples produces exactly 1 frame
	samples := make([]int16, WindowSize)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spec.NumFrames != 1 {
		t.Errorf("expected 1 frame for exactly WindowSize samples, got %d", spec.NumFrames)
	}
}

func TestCompute_FrameCount_Formula(t *testing.T) {
	// Verify the frame count formula:
	// frames = floor((totalSamples - WindowSize) / HopSize) + 1
	totalSamples := 11025 * 3 // 3 seconds of audio
	expectedFrames := (totalSamples-WindowSize)/HopSize + 1

	samples := make([]int16, totalSamples)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.NumFrames != expectedFrames {
		t.Errorf(
			"expected %d frames for %d samples, got %d",
			expectedFrames, totalSamples, spec.NumFrames,
		)
	}
}

func TestCompute_NumFrames_MatchesSliceLength(t *testing.T) {
	samples := make([]int16, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if spec.NumFrames != len(spec.Frames) {
		t.Errorf(
			"NumFrames field %d does not match len(Frames) %d",
			spec.NumFrames, len(spec.Frames),
		)
	}
}

// -----------------------------------------------------------------------
// Compute — Frame Structure
// -----------------------------------------------------------------------

func TestCompute_BinsPerFrame(t *testing.T) {
	samples := make([]int16, WindowSize*4)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBins := BinEnd - BinStart
	if spec.BinsPerFrame != expectedBins {
		t.Errorf("expected BinsPerFrame=%d, got %d", expectedBins, spec.BinsPerFrame)
	}
}

func TestCompute_EachFrame_HasCorrectMagnitudeLength(t *testing.T) {
	samples := make([]int16, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedBins := BinEnd - BinStart
	for i, f := range spec.Frames {
		if len(f.Magnitudes) != expectedBins {
			t.Errorf(
				"frame %d: expected %d magnitudes, got %d",
				i, expectedBins, len(f.Magnitudes),
			)
			return
		}
	}
}

func TestCompute_FrameIndexSequential(t *testing.T) {
	samples := make([]int16, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, f := range spec.Frames {
		if f.Index != i {
			t.Errorf("frame at position %d has Index=%d, expected %d", i, f.Index, i)
			return
		}
	}
}

func TestCompute_FrameTimeSeconds_Increasing(t *testing.T) {
	samples := make([]int16, 11025*3)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i := 1; i < len(spec.Frames); i++ {
		if spec.Frames[i].TimeSeconds <= spec.Frames[i-1].TimeSeconds {
			t.Errorf(
				"frame %d TimeSeconds %f is not greater than frame %d TimeSeconds %f",
				i, spec.Frames[i].TimeSeconds,
				i-1, spec.Frames[i-1].TimeSeconds,
			)
			return
		}
	}
}

func TestCompute_FirstFrame_TimeSeconds(t *testing.T) {
	samples := make([]int16, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Frame 0 center = WindowSize/2 samples / SampleRate
	expected := float64(WindowSize/2) / float64(SampleRate)
	if math.Abs(spec.Frames[0].TimeSeconds-expected) > 0.001 {
		t.Errorf(
			"expected first frame TimeSeconds ~%f, got %f",
			expected, spec.Frames[0].TimeSeconds,
		)
	}
}

// -----------------------------------------------------------------------
// Hamming Window
// -----------------------------------------------------------------------

func TestHammingWindow_Size(t *testing.T) {
	w := buildHammingWindow(WindowSize)
	if len(w) != WindowSize {
		t.Errorf("expected Hamming window length %d, got %d", WindowSize, len(w))
	}
}

func TestHammingWindow_EdgeValuesNearZero(t *testing.T) {
	w := buildHammingWindow(WindowSize)

	// First and last coefficients must be near zero
	// For a Hamming window: w(0) = 0.54 - 0.46 = 0.08
	// This is the minimum value — edges are suppressed but not exactly zero
	const edgeThreshold = 0.09
	if w[0] > edgeThreshold {
		t.Errorf("expected first Hamming coefficient near zero, got %f", w[0])
	}
	if w[WindowSize-1] > edgeThreshold {
		t.Errorf("expected last Hamming coefficient near zero, got %f", w[WindowSize-1])
	}
}

func TestHammingWindow_CenterValueNearOne(t *testing.T) {
	w := buildHammingWindow(WindowSize)

	// Center coefficient must be near 1.0
	center := w[WindowSize/2]
	if math.Abs(center-1.0) > 0.01 {
		t.Errorf("expected center Hamming coefficient ~1.0, got %f", center)
	}
}

func TestHammingWindow_Symmetric(t *testing.T) {
	w := buildHammingWindow(WindowSize)

	// Hamming window is symmetric around its center
	for i := 0; i < WindowSize/2; i++ {
		diff := math.Abs(w[i] - w[WindowSize-1-i])
		if diff > 1e-10 {
			t.Errorf(
				"Hamming window not symmetric at index %d: w[%d]=%f w[%d]=%f",
				i, i, w[i], WindowSize-1-i, w[WindowSize-1-i],
			)
			return
		}
	}
}

func TestHammingWindow_AllValuesInRange(t *testing.T) {
	w := buildHammingWindow(WindowSize)

	// All Hamming coefficients must be in [0.0, 1.0]
	for i, v := range w {
		if v < 0.0 || v > 1.0 {
			t.Errorf("Hamming coefficient at index %d out of range [0,1]: %f", i, v)
			return
		}
	}
}

// -----------------------------------------------------------------------
// Frequency Bin Conversion
// -----------------------------------------------------------------------

func TestFreqToBin_20Hz(t *testing.T) {
	bin := freqToBin(20.0)
	if bin != BinStart {
		t.Errorf("expected freqToBin(20Hz)=%d (BinStart), got %d", BinStart, bin)
	}
}

func TestFreqToBin_5000Hz(t *testing.T) {
	bin := freqToBin(5000.0)
	if bin != BinEnd {
		t.Errorf("expected freqToBin(5000Hz)=%d (BinEnd), got %d", BinEnd, bin)
	}
}

func TestBinToFreq_RoundTrip(t *testing.T) {
	// Converting a frequency to a bin and back should recover
	// a frequency within one bin width of the original
	testFreqs := []float64{100.0, 440.0, 1000.0, 2500.0, 4000.0}
	binWidth := float64(SampleRate) / float64(WindowSize)

	for _, freq := range testFreqs {
		bin := freqToBin(freq)
		recovered := BinToFreq(bin)
		diff := math.Abs(freq - recovered)
		if diff > binWidth {
			t.Errorf(
				"freq %f Hz → bin %d → %f Hz: diff %f exceeds bin width %f",
				freq, bin, recovered, diff, binWidth,
			)
		}
	}
}

func TestBinStart_LessThan_BinEnd(t *testing.T) {
	if BinStart >= BinEnd {
		t.Errorf("BinStart (%d) must be less than BinEnd (%d)", BinStart, BinEnd)
	}
}

func TestBinEnd_WithinNumBins(t *testing.T) {
	if BinEnd >= NumBins {
		t.Errorf("BinEnd (%d) must be less than NumBins (%d)", BinEnd, NumBins)
	}
}

// -----------------------------------------------------------------------
// FFT Correctness — Sine Wave Peak Detection
// -----------------------------------------------------------------------

func TestCompute_SineWave_PeakAtCorrectBin(t *testing.T) {
	// Generate a pure 440Hz sine wave (concert A)
	// After FFT, the dominant bin in the first frame must correspond
	// to 440Hz within one bin width of tolerance
	const testFreq = 440.0
	samples := makeSineWave(testFreq, WindowSize*8)

	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(spec.Frames) == 0 {
		t.Fatal("expected at least one frame")
	}

	// Use the middle frame to avoid edge effects from the Hamming window
	midFrame := spec.Frames[len(spec.Frames)/2]
	dominantBinIdx := dominantBin(midFrame)

	// Convert the dominant bin index back to Hz
	// dominantBinIdx is relative to BinStart
	absoluteBin := BinStart + dominantBinIdx
	detectedFreq := BinToFreq(absoluteBin)

	binWidth := float64(SampleRate) / float64(WindowSize)
	diff := math.Abs(detectedFreq - testFreq)

	if diff > binWidth*2 {
		t.Errorf(
			"expected dominant frequency ~%f Hz, got %f Hz (diff=%f, tolerance=%f)",
			testFreq, detectedFreq, diff, binWidth*2,
		)
	}
}

func TestCompute_SilentAudio_MagnitudesNearZero(t *testing.T) {
	// All-zero input must produce all-zero (or near-zero) magnitudes
	samples := make([]int16, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	const tolerance = 1e-9
	for i, f := range spec.Frames {
		for j, m := range f.Magnitudes {
			if m > tolerance {
				t.Errorf(
					"frame %d bin %d: expected near-zero magnitude for silent audio, got %f",
					i, j, m,
				)
				return
			}
		}
	}
}

func TestCompute_MagnitudesNonNegative(t *testing.T) {
	// Magnitudes are absolute values — they must never be negative
	samples := makeSineWave(440.0, 11025*2)
	spec, err := Compute(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, f := range spec.Frames {
		for j, m := range f.Magnitudes {
			if m < 0 {
				t.Errorf("frame %d bin %d: negative magnitude %f", i, j, m)
				return
			}
		}
	}
}

func TestCompute_Deterministic(t *testing.T) {
	// Same input must always produce identical output
	samples := makeSineWave(440.0, 11025*3)
	pcm := makePCM(samples)

	spec1, err := Compute(pcm)
	if err != nil {
		t.Fatalf("first Compute failed: %v", err)
	}

	spec2, err := Compute(pcm)
	if err != nil {
		t.Fatalf("second Compute failed: %v", err)
	}

	if spec1.NumFrames != spec2.NumFrames {
		t.Fatalf("frame counts differ: %d vs %d", spec1.NumFrames, spec2.NumFrames)
	}

	for i := range spec1.Frames {
		for j, m := range spec1.Frames[i].Magnitudes {
			if m != spec2.Frames[i].Magnitudes[j] {
				t.Errorf(
					"frame %d bin %d: non-deterministic magnitude %f vs %f",
					i, j, m, spec2.Frames[i].Magnitudes[j],
				)
				return
			}
		}
	}
}

// -----------------------------------------------------------------------
// Benchmark
// -----------------------------------------------------------------------

func BenchmarkCompute_ThreeMinutes(b *testing.B) {
	// 3 minutes of audio at 11025 Hz = 1,984,500 samples
	totalSamples := 11025 * 180
	samples := makeSineWave(440.0, totalSamples)
	pcm := makePCM(samples)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Compute(pcm)
		if err != nil {
			b.Fatalf("Compute failed during benchmark: %v", err)
		}
	}
}