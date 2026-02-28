package audio

import (
	"math"
	"os/exec"
	"path/filepath"
	"testing"
	"os"
)

// readFile is a test helper to read a file into bytes
// os.ReadFile is available in Go 1.16+ but we wrap it
// here to keep test helper functions consistent
func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makePCM is a test helper that builds a PCMData from a raw int16 slice
// without needing FFmpeg. Used for pure unit tests of normalization logic.
func makePCM(samples []int16) *PCMData {
	return &PCMData{
		Samples:    samples,
		SampleRate: 11025,
		Channels:   1,
	}
}

// peakOf returns the maximum absolute value in a sample slice
// Mirrors findPeak logic, used to verify normalization output in tests
func peakOf(samples []int16) int16 {
	var maxAbs int32
	for _, s := range samples {
		abs := int32(s)
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}
	if maxAbs > int16max {
		return int16max
	}
	return int16(maxAbs)
}

// -----------------------------------------------------------------------
// Normalize — Nil and Empty Input
// -----------------------------------------------------------------------

func TestNormalize_NilInput(t *testing.T) {
	_, err := Normalize(nil)
	if err == nil {
		t.Fatal("expected error for nil input, got nil")
	}
}

func TestNormalize_EmptySamples(t *testing.T) {
	_, err := Normalize(makePCM([]int16{}))
	if err == nil {
		t.Fatal("expected error for empty samples slice, got nil")
	}
}

// -----------------------------------------------------------------------
// Normalize — Silent Audio
// -----------------------------------------------------------------------

func TestNormalize_SilentAudio_WasSilentTrue(t *testing.T) {
	// All zeros — perfectly silent
	samples := make([]int16, 11025)
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error for silent audio: %v", err)
	}
	if !result.WasSilent {
		t.Error("expected WasSilent=true for all-zero samples")
	}
}

func TestNormalize_SilentAudio_ScaleFactorOne(t *testing.T) {
	samples := make([]int16, 11025)
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ScaleFactor != 1.0 {
		t.Errorf("expected ScaleFactor=1.0 for silent audio, got %f", result.ScaleFactor)
	}
}

func TestNormalize_SilentAudio_SamplesUnchanged(t *testing.T) {
	samples := make([]int16, 1000)
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for i, s := range result.PCM.Samples {
		if s != 0 {
			t.Errorf("expected sample[%d]=0 for silent audio, got %d", i, s)
			return
		}
	}
}

func TestNormalize_BelowThreshold_WasSilentTrue(t *testing.T) {
	// Peak of 5 is below silenceThreshold of 10
	samples := []int16{1, -3, 5, -2, 4, 0, -1, 3}
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.WasSilent {
		t.Error("expected WasSilent=true for audio below silence threshold")
	}
}

// -----------------------------------------------------------------------
// Normalize — Peak Amplitude After Normalization
// -----------------------------------------------------------------------

func TestNormalize_PeakReachesCeiling(t *testing.T) {
	// Create audio with a known peak well below the ceiling
	// Peak = 10000, target ceiling = 0.95 * 32767 = 31128
	samples := []int16{0, 5000, 10000, -8000, 3000, -10000, 0}
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	actualPeak := peakOf(result.PCM.Samples)
	expectedPeak := int16(math.Round(float64(int16max) * normalizationCeiling))

	// Allow ±2 counts for floating point rounding
	diff := int(actualPeak) - int(expectedPeak)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Errorf(
			"expected peak ~%d after normalization, got %d (diff=%d)",
			expectedPeak, actualPeak, diff,
		)
	}
}

func TestNormalize_AlreadyAtCeiling_ScaleNearOne(t *testing.T) {
	// Audio whose peak is already at the target ceiling
	// Scale factor should be very close to 1.0
	targetPeak := int16(math.Round(float64(int16max) * normalizationCeiling))
	samples := []int16{0, targetPeak, -targetPeak, targetPeak / 2}

	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Scale factor should be within 1% of 1.0
	if math.Abs(result.ScaleFactor-1.0) > 0.01 {
		t.Errorf(
			"expected ScaleFactor ~1.0 for already-normalized audio, got %f",
			result.ScaleFactor,
		)
	}
}

func TestNormalize_QuietAudio_AmplifiedCorrectly(t *testing.T) {
	// Very quiet audio — peak at 100 out of 32767
	// Scale factor should be approximately 31128 / 100 = 311.28
	samples := []int16{50, -100, 75, -80, 100}
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expectedScale := (float64(int16max) * normalizationCeiling) / 100.0
	tolerance := expectedScale * 0.01 // 1% tolerance

	diff := math.Abs(result.ScaleFactor - expectedScale)
	if diff > tolerance {
		t.Errorf(
			"expected ScaleFactor ~%f, got %f",
			expectedScale, result.ScaleFactor,
		)
	}
}

// -----------------------------------------------------------------------
// Normalize — Non-Destructive (Original Unchanged)
// -----------------------------------------------------------------------

func TestNormalize_OriginalPCMUnchanged(t *testing.T) {
	// The original samples must never be modified
	original := []int16{1000, -2000, 3000, -4000, 5000}
	backup := make([]int16, len(original))
	copy(backup, original)

	input := makePCM(original)
	_, err := Normalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify original slice is unchanged
	for i, s := range input.Samples {
		if s != backup[i] {
			t.Errorf(
				"original sample[%d] was modified: expected %d, got %d",
				i, backup[i], s,
			)
		}
	}
}

func TestNormalize_ReturnsSeparateSlice(t *testing.T) {
	samples := []int16{1000, -2000, 3000}
	input := makePCM(samples)
	result, err := Normalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Modifying the output must not affect the input
	result.PCM.Samples[0] = 0

	if input.Samples[0] == 0 {
		t.Error("modifying normalized output affected the original input samples")
	}
}

// -----------------------------------------------------------------------
// Normalize — Metadata Preserved
// -----------------------------------------------------------------------

func TestNormalize_SampleRatePreserved(t *testing.T) {
	input := makePCM([]int16{1000, -2000, 3000, -4000})
	result, err := Normalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PCM.SampleRate != 11025 {
		t.Errorf("expected SampleRate=11025, got %d", result.PCM.SampleRate)
	}
}

func TestNormalize_ChannelsPreserved(t *testing.T) {
	input := makePCM([]int16{1000, -2000, 3000, -4000})
	result, err := Normalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.PCM.Channels != 1 {
		t.Errorf("expected Channels=1, got %d", result.PCM.Channels)
	}
}

func TestNormalize_SampleCountUnchanged(t *testing.T) {
	samples := make([]int16, 5000)
	for i := range samples {
		samples[i] = int16(i % 1000)
	}
	input := makePCM(samples)
	result, err := Normalize(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.PCM.Samples) != len(samples) {
		t.Errorf(
			"expected %d samples after normalization, got %d",
			len(samples), len(result.PCM.Samples),
		)
	}
}

// -----------------------------------------------------------------------
// Normalize — NormalizeResult Fields
// -----------------------------------------------------------------------

func TestNormalize_OriginalPeakReported(t *testing.T) {
	samples := []int16{100, -200, 150, -180, 200}
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.OriginalPeak != 200 {
		t.Errorf("expected OriginalPeak=200, got %d", result.OriginalPeak)
	}
}

func TestNormalize_WasSilentFalse_ForAudibleAudio(t *testing.T) {
	samples := []int16{100, -500, 800, -1000, 600}
	result, err := Normalize(makePCM(samples))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WasSilent {
		t.Error("expected WasSilent=false for audible audio")
	}
}

// -----------------------------------------------------------------------
// findPeak — Edge Cases
// -----------------------------------------------------------------------

func TestFindPeak_AllPositive(t *testing.T) {
	samples := []int16{100, 200, 300, 150, 50}
	peak := findPeak(samples)
	if peak != 300 {
		t.Errorf("expected peak=300, got %d", peak)
	}
}

func TestFindPeak_AllNegative(t *testing.T) {
	samples := []int16{-100, -200, -300, -150, -50}
	peak := findPeak(samples)
	if peak != 300 {
		t.Errorf("expected peak=300 for all-negative samples, got %d", peak)
	}
}

func TestFindPeak_Mixed(t *testing.T) {
	samples := []int16{-500, 300, -800, 200, 100}
	peak := findPeak(samples)
	if peak != 800 {
		t.Errorf("expected peak=800, got %d", peak)
	}
}

func TestFindPeak_Int16Min_NoOverflow(t *testing.T) {
	// math.MinInt16 = -32768
	// abs(-32768) = 32768 which overflows int16
	// findPeak must handle this without panicking or wrapping
	samples := []int16{-32768, 100, -200}
	peak := findPeak(samples)

	// We clamp to int16max (32767) when the abs would overflow
	if peak != int16max {
		t.Errorf("expected peak=32767 for MinInt16 input, got %d", peak)
	}
}

func TestFindPeak_SingleSample(t *testing.T) {
	samples := []int16{42}
	peak := findPeak(samples)
	if peak != 42 {
		t.Errorf("expected peak=42, got %d", peak)
	}
}

func TestFindPeak_AllZeros(t *testing.T) {
	samples := make([]int16, 100)
	peak := findPeak(samples)
	if peak != 0 {
		t.Errorf("expected peak=0 for all-zero samples, got %d", peak)
	}
}

// -----------------------------------------------------------------------
// DecodeAndNormalize — Integration with Real Audio
// -----------------------------------------------------------------------

func TestDecodeAndNormalize_ValidFile(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_norm.wav")
	generateSineWAV(t, wavPath, 440, 3.0)

	result, err := DecodeAndNormalize(wavPath, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeAndNormalize failed: %v", err)
	}

	// Must not be marked silent — a 440Hz sine wave is audible
	if result.WasSilent {
		t.Error("expected WasSilent=false for a 440Hz sine wave")
	}

	// Peak must be near the ceiling
	actualPeak := peakOf(result.PCM.Samples)
	expectedPeak := int16(math.Round(float64(int16max) * normalizationCeiling))

	diff := int(actualPeak) - int(expectedPeak)
	if diff < 0 {
		diff = -diff
	}
	if diff > 2 {
		t.Errorf(
			"expected peak ~%d after DecodeAndNormalize, got %d",
			expectedPeak, actualPeak,
		)
	}

	// Sample rate must still be 11025 after the full pipeline
	if result.PCM.SampleRate != 11025 {
		t.Errorf("expected SampleRate=11025, got %d", result.PCM.SampleRate)
	}
}

func TestDecodeAndNormalize_FileNotFound(t *testing.T) {
	_, err := DecodeAndNormalize("/nonexistent/file.mp3", "ffmpeg")
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

// -----------------------------------------------------------------------
// DecodeBytesAndNormalize
// -----------------------------------------------------------------------

func TestDecodeBytesAndNormalize_EmptyBytes(t *testing.T) {
	_, err := DecodeBytesAndNormalize([]byte{}, "ffmpeg")
	if err == nil {
		t.Fatal("expected error for empty bytes, got nil")
	}
}

func TestDecodeBytesAndNormalize_ValidBytes(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("FFmpeg not available")
	}

	dir := t.TempDir()
	wavPath := filepath.Join(dir, "test_bytes_norm.wav")
	generateSineWAV(t, wavPath, 440, 2.0)

	data, err := readFile(wavPath)
	if err != nil {
		t.Fatalf("failed to read test WAV: %v", err)
	}

	result, err := DecodeBytesAndNormalize(data, "ffmpeg")
	if err != nil {
		t.Fatalf("DecodeBytesAndNormalize failed: %v", err)
	}

	if result.PCM.SampleRate != 11025 {
		t.Errorf("expected SampleRate=11025, got %d", result.PCM.SampleRate)
	}

	if len(result.PCM.Samples) == 0 {
		t.Error("expected non-empty samples from DecodeBytesAndNormalize")
	}
}

// -----------------------------------------------------------------------
// Benchmark
// -----------------------------------------------------------------------

func BenchmarkNormalize_LargeSlice(b *testing.B) {
	// Simulate 30 seconds of audio at 11025 Hz = 330750 samples
	samples := make([]int16, 11025*30)
	for i := range samples {
		samples[i] = int16((i % 20000) - 10000)
	}
	input := makePCM(samples)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := Normalize(input)
		if err != nil {
			b.Fatalf("Normalize failed during benchmark: %v", err)
		}
	}
}