package dsp

import (
	"testing"
	"fmt"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makePeak builds a Peak with the given bin and frame index.
// All other fields are set to sensible defaults for testing.
func makePeak(bin int, frameIndex int) Peak {
	return Peak{
		Bin:         bin,
		Magnitude:   500.0,
		FrequencyHz: BinToFreq(bin),
		FrameIndex:  frameIndex,
		TimeSeconds: float64(frameIndex) * 0.046,
		BandIndex:   0,
	}
}

// makePeakList builds a PeakList containing the provided peaks.
func makePeakList(frameIndex int, peaks []Peak) PeakList {
	return PeakList{
		Peaks:       peaks,
		FrameIndex:  frameIndex,
		TimeSeconds: float64(frameIndex) * 0.046,
	}
}

// -----------------------------------------------------------------------
// encodeHash / DecodeHash — Round Trip
// -----------------------------------------------------------------------

func TestEncodeHash_Deterministic(t *testing.T) {
	// Same inputs must always produce the same hash
	h1 := encodeHash(100, 200, 50)
	h2 := encodeHash(100, 200, 50)

	if h1 != h2 {
		t.Errorf("encodeHash is not deterministic: got %d and %d", h1, h2)
	}
}

func TestDecodeHash_RoundTrip(t *testing.T) {
	cases := []struct {
		anchorBin   int
		targetBin   int
		deltaFrames int
	}{
		{0, 0, 0},
		{100, 200, 50},
		{511, 511, 16383}, // maximum values for each field
		{1, 1, 1},
		{255, 300, 100},
		{464, 100, 99}, // realistic bin values from our frequency range
	}

	for _, tc := range cases {
		hash := encodeHash(tc.anchorBin, tc.targetBin, tc.deltaFrames)
		decoded := DecodeHash(hash)

		if decoded.AnchorFreq != tc.anchorBin {
			t.Errorf(
				"anchorBin: encoded %d, decoded %d",
				tc.anchorBin, decoded.AnchorFreq,
			)
		}

		if decoded.TargetFreq != tc.targetBin {
			t.Errorf(
				"targetBin: encoded %d, decoded %d",
				tc.targetBin, decoded.TargetFreq,
			)
		}

		if decoded.DeltaTime != tc.deltaFrames {
			t.Errorf(
				"deltaFrames: encoded %d, decoded %d",
				tc.deltaFrames, decoded.DeltaTime,
			)
		}
	}
}

func TestEncodeHash_FieldsDoNotBleed(t *testing.T) {
	// Verify that setting one field to max does not affect the others
	// anchorBin=511 (all 9 bits set), targetBin=0, deltaFrames=0
	hash := encodeHash(511, 0, 0)
	decoded := DecodeHash(hash)

	if decoded.AnchorFreq != 511 {
		t.Errorf("expected AnchorFreq=511, got %d", decoded.AnchorFreq)
	}
	if decoded.TargetFreq != 0 {
		t.Errorf("expected TargetFreq=0, got %d", decoded.TargetFreq)
	}
	if decoded.DeltaTime != 0 {
		t.Errorf("expected DeltaTime=0, got %d", decoded.DeltaTime)
	}

	// targetBin=511, anchorBin=0, deltaFrames=0
	hash = encodeHash(0, 511, 0)
	decoded = DecodeHash(hash)

	if decoded.AnchorFreq != 0 {
		t.Errorf("expected AnchorFreq=0, got %d", decoded.AnchorFreq)
	}
	if decoded.TargetFreq != 511 {
		t.Errorf("expected TargetFreq=511, got %d", decoded.TargetFreq)
	}
	if decoded.DeltaTime != 0 {
		t.Errorf("expected DeltaTime=0, got %d", decoded.DeltaTime)
	}

	// deltaFrames=16383 (all 14 bits set), anchorBin=0, targetBin=0
	hash = encodeHash(0, 0, 16383)
	decoded = DecodeHash(hash)

	if decoded.AnchorFreq != 0 {
		t.Errorf("expected AnchorFreq=0, got %d", decoded.AnchorFreq)
	}
	if decoded.TargetFreq != 0 {
		t.Errorf("expected TargetFreq=0, got %d", decoded.TargetFreq)
	}
	if decoded.DeltaTime != 16383 {
		t.Errorf("expected DeltaTime=16383, got %d", decoded.DeltaTime)
	}
}

func TestEncodeHash_MaskOverflow(t *testing.T) {
	// Values larger than their field size must be masked correctly
	// anchorBin=512 overflows 9 bits (512 & 0x1FF = 0)
	hash := encodeHash(512, 0, 0)
	decoded := DecodeHash(hash)

	if decoded.AnchorFreq != 0 {
		t.Errorf(
			"expected AnchorFreq=0 after masking 512, got %d",
			decoded.AnchorFreq,
		)
	}
}

// -----------------------------------------------------------------------
// flattenPeaks
// -----------------------------------------------------------------------

func TestFlattenPeaks_Empty(t *testing.T) {
	result := flattenPeaks([]PeakList{})
	if len(result) != 0 {
		t.Errorf("expected empty slice for empty input, got %d", len(result))
	}
}

func TestFlattenPeaks_SingleList(t *testing.T) {
	peaks := []Peak{
		makePeak(100, 0),
		makePeak(200, 0),
	}
	pl := makePeakList(0, peaks)
	result := flattenPeaks([]PeakList{pl})

	if len(result) != 2 {
		t.Errorf("expected 2 peaks, got %d", len(result))
	}
}

func TestFlattenPeaks_MultipleLists(t *testing.T) {
	pl0 := makePeakList(0, []Peak{makePeak(100, 0), makePeak(200, 0)})
	pl1 := makePeakList(1, []Peak{makePeak(300, 1)})
	pl2 := makePeakList(2, []Peak{makePeak(400, 2), makePeak(50, 2), makePeak(150, 2)})

	result := flattenPeaks([]PeakList{pl0, pl1, pl2})

	if len(result) != 6 {
		t.Errorf("expected 6 peaks total, got %d", len(result))
	}
}

func TestFlattenPeaks_OrderPreserved(t *testing.T) {
	// Peaks must appear in frame order after flattening
	pl0 := makePeakList(0, []Peak{makePeak(100, 0)})
	pl1 := makePeakList(1, []Peak{makePeak(200, 1)})
	pl2 := makePeakList(2, []Peak{makePeak(300, 2)})

	result := flattenPeaks([]PeakList{pl0, pl1, pl2})

	for i := 1; i < len(result); i++ {
		if result[i].FrameIndex < result[i-1].FrameIndex {
			t.Errorf(
				"peaks not in frame order: result[%d].FrameIndex=%d < result[%d].FrameIndex=%d",
				i, result[i].FrameIndex, i-1, result[i-1].FrameIndex,
			)
		}
	}
}

func TestFlattenPeaks_EmptyPeakLists_Skipped(t *testing.T) {
	// PeakLists with empty Peaks slices (silent frames)
	// must not contribute any peaks to the flat result
	pl0 := makePeakList(0, []Peak{makePeak(100, 0)})
	pl1 := makePeakList(1, []Peak{}) // silent frame
	pl2 := makePeakList(2, []Peak{makePeak(300, 2)})

	result := flattenPeaks([]PeakList{pl0, pl1, pl2})

	if len(result) != 2 {
		t.Errorf("expected 2 peaks (silent frame skipped), got %d", len(result))
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Input Validation
// -----------------------------------------------------------------------

func TestGenerateHashes_EmptyPeakLists(t *testing.T) {
	_, err := GenerateHashes([]PeakList{}, 1)
	if err == nil {
		t.Fatal("expected error for empty peakLists, got nil")
	}
}

func TestGenerateHashes_AllSilentFrames(t *testing.T) {
	// PeakLists where every frame has zero peaks should return an error
	// since no hashes can be generated from empty peaks
	peakLists := []PeakList{
		makePeakList(0, []Peak{}),
		makePeakList(1, []Peak{}),
		makePeakList(2, []Peak{}),
	}
	_, err := GenerateHashes(peakLists, 1)
	if err == nil {
		t.Fatal("expected error when all frames are silent, got nil")
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Output Structure
// -----------------------------------------------------------------------

func TestGenerateHashes_SongID_Propagated(t *testing.T) {
	// Every hash in the output must carry the songID passed in
	const songID = 42

	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0), makePeak(200, 0)}),
		makePeakList(1, []Peak{makePeak(150, 1), makePeak(250, 1)}),
		makePeakList(2, []Peak{makePeak(300, 2)}),
	}

	hashes, err := GenerateHashes(peakLists, songID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, h := range hashes {
		if h.SongID != songID {
			t.Errorf("hash[%d].SongID = %d, expected %d", i, h.SongID, songID)
			return
		}
	}
}

func TestGenerateHashes_ZeroSongID_ForQuery(t *testing.T) {
	// When identifying a snippet, songID = 0
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0), makePeak(200, 0)}),
		makePeakList(1, []Peak{makePeak(150, 1)}),
	}

	hashes, err := GenerateHashes(peakLists, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, h := range hashes {
		if h.SongID != 0 {
			t.Errorf("expected SongID=0 for query mode, got %d", h.SongID)
			return
		}
	}
}

func TestGenerateHashes_AnchorTime_IsFrameIndex(t *testing.T) {
	// AnchorTime must equal the FrameIndex of the anchor peak
	peakLists := []PeakList{
		makePeakList(5, []Peak{makePeak(100, 5)}),
		makePeakList(6, []Peak{makePeak(200, 6)}),
		makePeakList(7, []Peak{makePeak(300, 7)}),
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, h := range hashes {
		// AnchorTime must be one of our frame indices (5, 6, or 7)
		if h.AnchorTime < 5 || h.AnchorTime > 7 {
			t.Errorf(
				"AnchorTime %d is not a valid frame index (5-7)",
				h.AnchorTime,
			)
			return
		}
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Fan-out Behaviour
// -----------------------------------------------------------------------

func TestGenerateHashes_FanOut_AtMostFivePerAnchor(t *testing.T) {
	// Build enough targets in the zone so that fan-out is the limiting
	// factor. Place 10 peaks in frames 1-10, one anchor in frame 0.
	peaks0 := []Peak{makePeak(100, 0)} // anchor
	peakLists := []PeakList{makePeakList(0, peaks0)}

	// Add 10 target frames each with one peak
	for i := 1; i <= 10; i++ {
		peakLists = append(peakLists,
			makePeakList(i, []Peak{makePeak(200+i, i)}),
		)
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Count hashes where AnchorTime == 0 (from the anchor peak)
	anchorHashes := 0
	for _, h := range hashes {
		if h.AnchorTime == 0 {
			anchorHashes++
		}
	}

	if anchorHashes > FanOut {
		t.Errorf(
			"expected at most %d hashes per anchor, got %d",
			FanOut, anchorHashes,
		)
	}
}

func TestGenerateHashes_FanOut_FewerTargetsThanFanOut(t *testing.T) {
	// When fewer targets are available than FanOut, all available
	// targets should be used — no error, no padding
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0)}), // anchor
		makePeakList(1, []Peak{makePeak(200, 1)}), // only 1 target
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Anchor at frame 0 paired with target at frame 1 = 1 hash
	// Target at frame 1 has no forward peaks = 0 hashes
	if len(hashes) != 1 {
		t.Errorf("expected 1 hash for 1 anchor + 1 target, got %d", len(hashes))
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Target Zone Boundary
// -----------------------------------------------------------------------

func TestGenerateHashes_TargetOutsideZone_NotPaired(t *testing.T) {
	// A peak beyond TargetZoneFrames must not be paired with the anchor
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0)}),
		// This peak is beyond the target zone
		makePeakList(TargetZoneFrames+1,
			[]Peak{makePeak(200, TargetZoneFrames+1)}),
	}

	hashes, err := GenerateHashes(peakLists, 1)
	// With only one anchor and no valid targets we expect an error
	if err == nil {
		// If no error, verify no hashes were produced with
		// a delta time beyond the zone
		for _, h := range hashes {
			decoded := DecodeHash(h.Hash)
			if decoded.DeltaTime > TargetZoneFrames {
				t.Errorf(
					"hash has deltaTime %d which exceeds TargetZoneFrames %d",
					decoded.DeltaTime, TargetZoneFrames,
				)
			}
		}
	}
}

func TestGenerateHashes_TargetInsideZone_IsPaired(t *testing.T) {
	// A peak exactly at the boundary of the target zone must be paired
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0)}),
		makePeakList(TargetZoneFrames, []Peak{makePeak(200, TargetZoneFrames)}),
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(hashes) == 0 {
		t.Error("expected at least one hash for target at zone boundary")
	}

	decoded := DecodeHash(hashes[0].Hash)
	if decoded.DeltaTime != TargetZoneFrames {
		t.Errorf(
			"expected DeltaTime=%d, got %d",
			TargetZoneFrames, decoded.DeltaTime,
		)
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Hash Content
// -----------------------------------------------------------------------

func TestGenerateHashes_DeltaTime_AlwaysPositive(t *testing.T) {
	// Since targets are always ahead of anchors in time,
	// every decoded delta time must be >= 0
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0), makePeak(200, 0)}),
		makePeakList(1, []Peak{makePeak(150, 1), makePeak(250, 1)}),
		makePeakList(5, []Peak{makePeak(300, 5)}),
		makePeakList(10, []Peak{makePeak(400, 10)}),
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, h := range hashes {
		decoded := DecodeHash(h.Hash)
		if decoded.DeltaTime < 0 {
			t.Errorf("hash[%d] has negative DeltaTime: %d", i, decoded.DeltaTime)
			return
		}
	}
}

func TestGenerateHashes_HashValues_NonZero(t *testing.T) {
	// Hashes from real peaks with different bins and delta times
	// should not all be zero
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0)}),
		makePeakList(1, []Peak{makePeak(200, 1)}),
		makePeakList(2, []Peak{makePeak(300, 2)}),
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	allZero := true
	for _, h := range hashes {
		if h.Hash != 0 {
			allZero = false
			break
		}
	}

	if allZero {
		t.Error("all hashes are zero — encoding is likely broken")
	}
}

// -----------------------------------------------------------------------
// GenerateHashes — Determinism
// -----------------------------------------------------------------------

func TestGenerateHashes_Deterministic(t *testing.T) {
	// Same peak lists must always produce identical hash slices
	peakLists := []PeakList{
		makePeakList(0, []Peak{makePeak(100, 0), makePeak(200, 0)}),
		makePeakList(1, []Peak{makePeak(150, 1)}),
		makePeakList(3, []Peak{makePeak(300, 3), makePeak(400, 3)}),
		makePeakList(5, []Peak{makePeak(50, 5)}),
	}

	hashes1, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("first call failed: %v", err)
	}

	hashes2, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("second call failed: %v", err)
	}

	if len(hashes1) != len(hashes2) {
		t.Fatalf(
			"hash count differs between calls: %d vs %d",
			len(hashes1), len(hashes2),
		)
	}

	for i := range hashes1 {
		if hashes1[i].Hash != hashes2[i].Hash {
			t.Errorf(
				"hash[%d] differs: %d vs %d",
				i, hashes1[i].Hash, hashes2[i].Hash,
			)
			return
		}
		if hashes1[i].AnchorTime != hashes2[i].AnchorTime {
			t.Errorf(
				"hash[%d] AnchorTime differs: %d vs %d",
				i, hashes1[i].AnchorTime, hashes2[i].AnchorTime,
			)
			return
		}
	}
}

// -----------------------------------------------------------------------
// HashCount
// -----------------------------------------------------------------------

func TestHashCount_Empty(t *testing.T) {
	result := HashCount([]FingerprintHash{})
	if result != 0 {
		t.Errorf("expected HashCount=0, got %d", result)
	}
}

func TestHashCount_NonEmpty(t *testing.T) {
	hashes := []FingerprintHash{{Hash: 1}, {Hash: 2}, {Hash: 3}}
	result := HashCount(hashes)
	if result != 3 {
		t.Errorf("expected HashCount=3, got %d", result)
	}
}

// -----------------------------------------------------------------------
// Integration — Full Pipeline
// -----------------------------------------------------------------------

func TestGenerateHashes_Integration_FullPipeline(t *testing.T) {
	// Run the full DSP pipeline:
	// sine wave → spectrogram → peaks → hashes
	// Verify that hashes are produced and are deterministic
	samples := makeSineWave(440.0, 11025*10) // 10 seconds
	pcm := makePCM(samples)

	spec, err := Compute(pcm)
	if err != nil {
		t.Fatalf("Compute failed: %v", err)
	}

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		t.Fatalf("ExtractPeaks failed: %v", err)
	}

	hashes, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("GenerateHashes failed: %v", err)
	}

	if len(hashes) == 0 {
		t.Fatal("expected non-empty hashes from full pipeline")
	}

	// All hashes must have the correct song ID
	for i, h := range hashes {
		if h.SongID != 1 {
			t.Errorf("hash[%d].SongID = %d, expected 1", i, h.SongID)
			return
		}
	}

	// Run again — must produce identical hashes (determinism)
	hashes2, err := GenerateHashes(peakLists, 1)
	if err != nil {
		t.Fatalf("second GenerateHashes failed: %v", err)
	}

	if len(hashes) != len(hashes2) {
		t.Fatalf(
			"hash count not deterministic: %d vs %d",
			len(hashes), len(hashes2),
		)
	}

	for i := range hashes {
		if hashes[i].Hash != hashes2[i].Hash {
			t.Errorf(
				"hash[%d] not deterministic: %d vs %d",
				i, hashes[i].Hash, hashes2[i].Hash,
			)
			return
		}
	}
}

func TestGenerateHashes_Integration_SnippetMatchesSong(t *testing.T) {
	// A 10-second snippet taken from the middle of a 3-minute song
	// must share a significant number of identical hashes with the
	// full song fingerprint.
	// This is the fundamental correctness test for the fingerprinter.
	fullSamples := makeSineWave(440.0, 11025*30) // 30 seconds

	// Snippet: seconds 10-20 of the full audio
	snippetStart := 11025 * 10
	snippetEnd := 11025 * 20
	snippetSamples := fullSamples[snippetStart:snippetEnd]

	fullPCM := makePCM(fullSamples)
	snippetPCM := makePCM(snippetSamples)

	fullSpec, err := Compute(fullPCM)
	if err != nil {
		t.Fatalf("Compute (full) failed: %v", err)
	}

	snippetSpec, err := Compute(snippetPCM)
	if err != nil {
		t.Fatalf("Compute (snippet) failed: %v", err)
	}

	fullPeaks, err := ExtractPeaks(fullSpec)
	if err != nil {
		t.Fatalf("ExtractPeaks (full) failed: %v", err)
	}

	snippetPeaks, err := ExtractPeaks(snippetSpec)
	if err != nil {
		t.Fatalf("ExtractPeaks (snippet) failed: %v", err)
	}

	fullHashes, err := GenerateHashes(fullPeaks, 1)
	if err != nil {
		t.Fatalf("GenerateHashes (full) failed: %v", err)
	}

	snippetHashes, err := GenerateHashes(snippetPeaks, 0)
	if err != nil {
		t.Fatalf("GenerateHashes (snippet) failed: %v", err)
	}

	// Build a set of all hash values from the full song
	fullHashSet := make(map[int32]struct{}, len(fullHashes))
	for _, h := range fullHashes {
		fullHashSet[h.Hash] = struct{}{}
	}

	// Count how many snippet hashes appear in the full song hash set
	matches := 0
	for _, h := range snippetHashes {
		if _, ok := fullHashSet[h.Hash]; ok {
			matches++
		}
	}

	// At least 30% of snippet hashes must match the full song
	// A pure sine wave will have highly consistent peaks so
	// the match rate should be well above this minimum
	matchRate := float64(matches) / float64(len(snippetHashes))
	fmt.Printf("Match rate: %.1f%% (%d/%d hashes matched)\n", matchRate*100, matches, len(snippetHashes))
	if matchRate < 0.30 {
		t.Errorf(
			"snippet match rate too low: %.1f%% (%d/%d hashes matched)",
			matchRate*100, matches, len(snippetHashes),
		)
	}
}

// -----------------------------------------------------------------------
// Benchmark
// -----------------------------------------------------------------------

func BenchmarkGenerateHashes_ThreeMinutes(b *testing.B) {
	totalSamples := 11025 * 180
	samples := makeSineWave(440.0, totalSamples)
	pcm := makePCM(samples)

	spec, err := Compute(pcm)
	if err != nil {
		b.Fatalf("Compute failed: %v", err)
	}

	peakLists, err := ExtractPeaks(spec)
	if err != nil {
		b.Fatalf("ExtractPeaks failed: %v", err)
	}

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, err := GenerateHashes(peakLists, 1)
		if err != nil {
			b.Fatalf("GenerateHashes failed: %v", err)
		}
	}
}