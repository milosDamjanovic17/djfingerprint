package dsp

import (
	"math"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makeFrame builds a Frame with a flat magnitude slice of the correct
// length for use in unit tests. All magnitudes default to zero.
func makeFrame(frameIndex int, timeSeconds float64) Frame {
	mags := make([]float64, BinEnd-BinStart)
	return Frame{
		Magnitudes:  mags,
		Index:       frameIndex,
		TimeSeconds: timeSeconds,
	}
}

// setMagnitude sets the magnitude at an absolute bin index in a Frame.
// Converts from absolute bin to the frame's relative magnitude index.
func setMagnitude(f *Frame, absoluteBin int, magnitude float64) {
	idx := absoluteBin - BinStart
	if idx >= 0 && idx < len(f.Magnitudes) {
		f.Magnitudes[idx] = magnitude
	}
}

// makeSpectrogram wraps a slice of Frames into a *Spectrogram.
func makeSpectrogram(frames []Frame) *Spectrogram {
	return &Spectrogram{
		Frames:       frames,
		NumFrames:    len(frames),
		BinsPerFrame: BinEnd - BinStart,
	}
}

// -----------------------------------------------------------------------
// ExtractPeaks — Nil and Empty Input
// -----------------------------------------------------------------------

func TestExtractPeaks_NilSpectrogram(t *testing.T) {
	_, err := ExtractPeaks(nil)
	if err == nil {
		t.Fatal("expected error for nil spectrogram, got nil")
	}
}

func TestExtractPeaks_NoFrames(t *testing.T) {
	spec := makeSpectrogram([]Frame{})
	_, err := ExtractPeaks(spec)
	if err == nil {
		t.Fatal("expected error for spectrogram with no frames, got nil")
	}
}

// -----------------------------------------------------------------------
// ExtractPeaks — Output Structure
// -----------------------------------------------------------------------

func TestExtractPeaks_OutputLength_MatchesFrameCount(t *testing.T) {
	frames := []Frame{
		makeFrame(0, 0.0),
		makeFrame(1, 0.046),
		makeFrame(2, 0.093),
	}
	spec := makeSpectrogram(frames)

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peakLists) != len(frames) {
		t.Errorf(
			"expected %d PeakLists, got %d",
			len(frames), len(peakLists),
		)
	}
}

func TestExtractPeaks_FrameIndex_Preserved(t *testing.T) {
	frames := []Frame{
		makeFrame(0, 0.0),
		makeFrame(1, 0.046),
		makeFrame(2, 0.093),
	}
	spec := makeSpectrogram(frames)

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, pl := range peakLists {
		if pl.FrameIndex != i {
			t.Errorf(
				"PeakList[%d].FrameIndex = %d, expected %d",
				i, pl.FrameIndex, i,
			)
		}
	}
}

func TestExtractPeaks_TimeSeconds_Preserved(t *testing.T) {
	times := []float64{0.0, 0.046, 0.093}
	frames := []Frame{
		makeFrame(0, times[0]),
		makeFrame(1, times[1]),
		makeFrame(2, times[2]),
	}
	spec := makeSpectrogram(frames)

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, pl := range peakLists {
		if pl.TimeSeconds != times[i] {
			t.Errorf(
				"PeakList[%d].TimeSeconds = %f, expected %f",
				i, pl.TimeSeconds, times[i],
			)
		}
	}
}

// -----------------------------------------------------------------------
// ExtractPeaks — Silent Frame
// -----------------------------------------------------------------------

func TestExtractPeaks_SilentFrame_EmptyPeakList(t *testing.T) {
	// A frame with all-zero magnitudes should produce an empty PeakList
	frame := makeFrame(0, 0.0)
	spec := makeSpectrogram([]Frame{frame})

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peakLists[0].Peaks) != 0 {
		t.Errorf(
			"expected 0 peaks for silent frame, got %d",
			len(peakLists[0].Peaks),
		)
	}
}

func TestExtractPeaks_ZeroMagnitude_NotACandidate(t *testing.T) {
	// Explicitly verify that bins with magnitude exactly 0.0
	// are never selected as peak candidates.
	// This guards against the threshold collapsing to 0.0
	// which would pass all zero-magnitude bins as ghost peaks.
	frame := makeFrame(0, 0.0)

	// Leave all magnitudes at their default zero value
	// Do not call setMagnitude at all

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peakLists[0].Peaks) != 0 {
		t.Errorf(
			"expected 0 peaks when all magnitudes are zero, got %d",
			len(peakLists[0].Peaks),
		)
	}
}

// -----------------------------------------------------------------------
// Dynamic Threshold
// -----------------------------------------------------------------------

func TestDynamicThreshold_Empty(t *testing.T) {
	result := dynamicThreshold([]Peak{})
	if result != 0.0 {
		t.Errorf("expected threshold=0.0 for empty candidates, got %f", result)
	}
}

func TestDynamicThreshold_SingleCandidate(t *testing.T) {
	candidates := []Peak{{Magnitude: 100.0}}
	result := dynamicThreshold(candidates)
	if result != 100.0 {
		t.Errorf("expected threshold=100.0 for single candidate, got %f", result)
	}
}

func TestDynamicThreshold_Average(t *testing.T) {
	// Average of 100, 200, 300 = 200
	candidates := []Peak{
		{Magnitude: 100.0},
		{Magnitude: 200.0},
		{Magnitude: 300.0},
	}
	result := dynamicThreshold(candidates)
	if math.Abs(result-200.0) > 1e-9 {
		t.Errorf("expected threshold=200.0, got %f", result)
	}
}

func TestDynamicThreshold_AllEqual(t *testing.T) {
	// When all magnitudes are equal, threshold equals that value
	// All 6 are equal → threshold = that value → all pass
	candidates := []Peak{
		{Magnitude: 50.0},
		{Magnitude: 50.0},
		{Magnitude: 50.0},
		{Magnitude: 50.0},
		{Magnitude: 50.0},
		{Magnitude: 50.0},
	}
	result := dynamicThreshold(candidates)
	if math.Abs(result-50.0) > 1e-9 {
		t.Errorf("expected threshold=50.0 for equal candidates, got %f", result)
	}
}

// -----------------------------------------------------------------------
// Threshold Filtering — Peaks Above and Below
// -----------------------------------------------------------------------

func TestExtractPeaks_WeakPeaks_Discarded(t *testing.T) {
	// Set up a frame where one band has a very strong peak
	// and all other bands have very weak peaks.
	// Only the strong peak should survive the threshold.
	frame := makeFrame(0, 0.0)

	// Place one very strong peak in the Mid band (bin ~111)
	midBin := freqToBin(1500.0) // solidly in Mid band
	setMagnitude(&frame, midBin, 1000.0)

	// Place weak peaks in all other bands
	setMagnitude(&frame, freqToBin(50.0), 1.0)   // VeryLow
	setMagnitude(&frame, freqToBin(400.0), 1.0)  // Low
	setMagnitude(&frame, freqToBin(800.0), 1.0)  // LowMid
	setMagnitude(&frame, freqToBin(3000.0), 1.0) // HighMid
	setMagnitude(&frame, freqToBin(4500.0), 1.0) // High

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Threshold ≈ (1000 + 1 + 1 + 1 + 1 + 1) / 6 ≈ 167.5
	// Only the 1000.0 magnitude peak survives
	peaks := peakLists[0].Peaks
	if len(peaks) != 1 {
		t.Errorf("expected 1 peak to survive threshold, got %d", len(peaks))
	}

	if len(peaks) == 1 && math.Abs(peaks[0].Magnitude-1000.0) > 1e-9 {
		t.Errorf("expected surviving peak magnitude=1000.0, got %f", peaks[0].Magnitude)
	}
}

func TestExtractPeaks_AllEqualMagnitude_AllSurvive(t *testing.T) {
	// When all 6 bands have equal magnitude peaks, all survive
	// threshold = average = the same value → all pass (>=)
	frame := makeFrame(0, 0.0)

	setMagnitude(&frame, freqToBin(50.0), 500.0)   // VeryLow
	setMagnitude(&frame, freqToBin(400.0), 500.0)  // Low
	setMagnitude(&frame, freqToBin(800.0), 500.0)  // LowMid
	setMagnitude(&frame, freqToBin(1500.0), 500.0) // Mid
	setMagnitude(&frame, freqToBin(3000.0), 500.0) // HighMid
	setMagnitude(&frame, freqToBin(4500.0), 500.0) // High

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	peaks := peakLists[0].Peaks
	if len(peaks) != 6 {
		t.Errorf("expected all 6 peaks to survive equal threshold, got %d", len(peaks))
	}
}

func TestExtractPeaks_MaxSixPeaksPerFrame(t *testing.T) {
	// No frame should ever produce more than 6 peaks
	// since there are only 6 bands — one candidate per band
	frame := makeFrame(0, 0.0)

	setMagnitude(&frame, freqToBin(50.0), 900.0)
	setMagnitude(&frame, freqToBin(400.0), 800.0)
	setMagnitude(&frame, freqToBin(800.0), 700.0)
	setMagnitude(&frame, freqToBin(1500.0), 600.0)
	setMagnitude(&frame, freqToBin(3000.0), 500.0)
	setMagnitude(&frame, freqToBin(4500.0), 400.0)

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peakLists[0].Peaks) > 6 {
		t.Errorf(
			"expected at most 6 peaks per frame, got %d",
			len(peakLists[0].Peaks),
		)
	}
}

// -----------------------------------------------------------------------
// Peak Fields
// -----------------------------------------------------------------------

func TestExtractPeaks_Peak_BandIndex_Correct(t *testing.T) {
	// Place one strong peak in the Mid band (bandIdx=3)
	// and weak peaks in all others
	// The surviving peak should have BandIndex=3
	frame := makeFrame(0, 0.0)

	setMagnitude(&frame, freqToBin(50.0), 1.0)    // VeryLow  band 0
	setMagnitude(&frame, freqToBin(400.0), 1.0)   // Low      band 1
	setMagnitude(&frame, freqToBin(800.0), 1.0)   // LowMid   band 2
	setMagnitude(&frame, freqToBin(1500.0), 900.0) // Mid     band 3 ← strong
	setMagnitude(&frame, freqToBin(3000.0), 1.0)  // HighMid  band 4
	setMagnitude(&frame, freqToBin(4500.0), 1.0)  // High     band 5

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(peakLists[0].Peaks) != 1 {
		t.Fatalf("expected 1 surviving peak, got %d", len(peakLists[0].Peaks))
	}

	if peakLists[0].Peaks[0].BandIndex != 3 {
		t.Errorf(
			"expected BandIndex=3 (Mid band), got %d",
			peakLists[0].Peaks[0].BandIndex,
		)
	}
}

func TestExtractPeaks_Peak_FrequencyHz_InBandRange(t *testing.T) {
	// Every peak's FrequencyHz must fall within its band's Hz range
	frame := makeFrame(0, 0.0)

	setMagnitude(&frame, freqToBin(50.0), 500.0)
	setMagnitude(&frame, freqToBin(400.0), 500.0)
	setMagnitude(&frame, freqToBin(800.0), 500.0)
	setMagnitude(&frame, freqToBin(1500.0), 500.0)
	setMagnitude(&frame, freqToBin(3000.0), 500.0)
	setMagnitude(&frame, freqToBin(4500.0), 500.0)

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, peak := range peakLists[0].Peaks {
		band := FrequencyBands[peak.BandIndex]
		if peak.FrequencyHz < band.MinHz || peak.FrequencyHz > band.MaxHz {
			t.Errorf(
				"peak FrequencyHz %f is outside band %s range [%f, %f]",
				peak.FrequencyHz, band.Name, band.MinHz, band.MaxHz,
			)
		}
	}
}

func TestExtractPeaks_Peak_TimeSeconds_MatchesFrame(t *testing.T) {
	const expectedTime = 1.234
	frame := makeFrame(0, expectedTime)
	setMagnitude(&frame, freqToBin(1500.0), 500.0)

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, peak := range peakLists[0].Peaks {
		if peak.TimeSeconds != expectedTime {
			t.Errorf(
				"expected peak TimeSeconds=%f, got %f",
				expectedTime, peak.TimeSeconds,
			)
		}
	}
}

func TestExtractPeaks_Peak_MagnitudeNonNegative(t *testing.T) {
	frame := makeFrame(0, 0.0)
	setMagnitude(&frame, freqToBin(50.0), 200.0)
	setMagnitude(&frame, freqToBin(400.0), 300.0)
	setMagnitude(&frame, freqToBin(800.0), 400.0)
	setMagnitude(&frame, freqToBin(1500.0), 500.0)
	setMagnitude(&frame, freqToBin(3000.0), 600.0)
	setMagnitude(&frame, freqToBin(4500.0), 700.0)

	spec := makeSpectrogram([]Frame{frame})
	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, peak := range peakLists[0].Peaks {
		if peak.Magnitude < 0 {
			t.Errorf("peak has negative magnitude: %f", peak.Magnitude)
		}
	}
}

// -----------------------------------------------------------------------
// FrequencyBands Initialisation
// -----------------------------------------------------------------------

func TestFrequencyBands_Count(t *testing.T) {
	if len(FrequencyBands) != 6 {
		t.Errorf("expected 6 frequency bands, got %d", len(FrequencyBands))
	}
}

func TestFrequencyBands_BinStart_LessThan_BinEnd(t *testing.T) {
	for _, band := range FrequencyBands {
		if band.binStart >= band.binEnd {
			t.Errorf(
				"band %s: binStart (%d) must be less than binEnd (%d)",
				band.Name, band.binStart, band.binEnd,
			)
		}
	}
}

func TestFrequencyBands_NoGaps(t *testing.T) {
	// Each band's MinHz must equal the previous band's MaxHz
	for i := 1; i < len(FrequencyBands); i++ {
		prev := FrequencyBands[i-1]
		curr := FrequencyBands[i]
		if curr.MinHz != prev.MaxHz {
			t.Errorf(
				"gap between band %s (MaxHz=%f) and band %s (MinHz=%f)",
				prev.Name, prev.MaxHz, curr.Name, curr.MinHz,
			)
		}
	}
}

func TestFrequencyBands_CoversFullRange(t *testing.T) {
	first := FrequencyBands[0]
	last := FrequencyBands[len(FrequencyBands)-1]

	if first.MinHz != FreqMin {
		t.Errorf("first band MinHz=%f, expected FreqMin=%v", first.MinHz, FreqMin)
	}
	if last.MaxHz != FreqMax {
		t.Errorf("last band MaxHz=%f, expected FreqMax=%v", last.MaxHz, FreqMax)
	}
}

// -----------------------------------------------------------------------
// PeakCount Utility
// -----------------------------------------------------------------------

func TestPeakCount_Empty(t *testing.T) {
	result := PeakCount([]PeakList{})
	if result != 0 {
		t.Errorf("expected PeakCount=0 for empty list, got %d", result)
	}
}

func TestPeakCount_MultipleLists(t *testing.T) {
	lists := []PeakList{
		{Peaks: []Peak{{}, {}, {}}},
		{Peaks: []Peak{{}}},
		{Peaks: []Peak{{}, {}}},
	}
	result := PeakCount(lists)
	if result != 6 {
		t.Errorf("expected PeakCount=6, got %d", result)
	}
}

// -----------------------------------------------------------------------
// Integration with Spectrogram
// -----------------------------------------------------------------------

func TestExtractPeaks_Integration_WithRealSpectrogram(t *testing.T) {
	// Generate a real spectrogram from a 440Hz sine wave
	// and verify that peak extraction completes without error
	// and produces a reasonable number of peaks
	samples := makeSineWave(440.0, 11025*3)
	pcm := makePCM(samples)

	spec, err := Compute(pcm)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("ExtractPeaks failed: %v", err)
	}

	// Must produce one PeakList per frame
	if len(peakLists) != spec.NumFrames {
		t.Errorf(
			"expected %d PeakLists, got %d",
			spec.NumFrames, len(peakLists),
		)
	}

	totalPeaks := PeakCount(peakLists)

	// A 440Hz sine wave has energy concentrated in one band
	// so many frames will have just 1 peak that passes threshold
	// We expect at least 1 peak per frame on average
	if totalPeaks < spec.NumFrames {
		t.Errorf(
			"expected at least %d total peaks for a sine wave, got %d",
			spec.NumFrames, totalPeaks,
		)
	}
}

// -----------------------------------------------------------------------
// Benchmark
// -----------------------------------------------------------------------

func BenchmarkExtractPeaks_ThreeMinutes(b *testing.B) {
	totalSamples := 11025 * 180
	samples := makeSineWave(440.0, totalSamples)
	pcm := makePCM(samples)

	spec, err := Compute(pcm)
	if err != nil {
		b.Fatalf("Compute failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := ExtractPeaks(spec)
		if err != nil {
			b.Fatalf("ExtractPeaks failed during benchmark: %v", err)
		}
	}
}