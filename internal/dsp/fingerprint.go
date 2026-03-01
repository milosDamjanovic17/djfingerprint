package dsp

import (
	"fmt"
)

const (
	// FanOut is the maximum number of target peaks paired with each anchor.
	// 5 targets per anchor balances fingerprint uniqueness against
	// database size. Increasing this improves match accuracy but
	// grows the fingerprints table significantly.
	FanOut = 5

	// TargetZoneFrames is how many frames ahead of an anchor we search
	// for target peaks. At 46.4ms per frame, 100 frames = ~4.6 seconds.
	TargetZoneFrames = 100

	// anchorFreqBits is the number of bits allocated to the anchor
	// frequency field in the 32-bit hash.
	anchorFreqBits = 9

	// targetFreqBits is the number of bits allocated to the target
	// frequency field in the 32-bit hash.
	targetFreqBits = 9

	// deltaTimeBits is the number of bits allocated to the time
	// difference field in the 32-bit hash.
	deltaTimeBits = 14

	// anchorFreqMask masks the anchor frequency to 9 bits (0-511)
	anchorFreqMask = (1 << anchorFreqBits) - 1 // 0x1FF

	// targetFreqMask masks the target frequency to 9 bits (0-511)
	targetFreqMask = (1 << targetFreqBits) - 1 // 0x1FF

	// deltaTimeMask masks the delta time to 14 bits (0-16383)
	deltaTimeMask = (1 << deltaTimeBits) - 1 // 0x3FFF

	// anchorFreqShift is how many bits the anchor frequency is shifted
	// left in the 32-bit hash layout.
	// Layout: [anchorFreq:9][targetFreq:9][deltaTime:14]
	// anchorFreq occupies bits 31-23 → shift = 9 + 14 = 23
	anchorFreqShift = targetFreqBits + deltaTimeBits // 23

	// targetFreqShift is how many bits the target frequency is shifted
	// left in the 32-bit hash layout.
	// targetFreq occupies bits 22-14 → shift = 14
	targetFreqShift = deltaTimeBits // 14
)

// FingerprintHash represents a single fingerprint hash and its
// associated timing metadata.
//
// During indexing (song upload):
//   - Hash is the encoded 32-bit value
//   - SongID is the database ID of the song being indexed
//   - AnchorTime is the frame index of the anchor peak
//
// During querying (snippet identification):
//   - Hash is the encoded 32-bit value
//   - SongID is 0 (unknown — we are trying to find it)
//   - AnchorTime is the frame index of the anchor peak
type FingerprintHash struct {
	Hash       int32
	SongID     int
	AnchorTime int
}

// DecodedHash holds the three component values extracted from a
// 32-bit fingerprint hash. Used for testing and diagnostics.
type DecodedHash struct {
	AnchorFreq int
	TargetFreq int
	DeltaTime  int
}

// GenerateHashes takes the peak lists produced by ExtractPeaks and
// generates fingerprint hashes by pairing each anchor peak with up
// to FanOut target peaks ahead of it in time.
//
// songID should be set to the database ID of the song when indexing.
// songID should be 0 when generating hashes for a query snippet.
//
// The returned slice contains one FingerprintHash per anchor-target
// pair. For a typical 3-minute song this will be in the range of
// 50,000 to 100,000 hashes.
func GenerateHashes(peakLists []PeakList, songID int) ([]FingerprintHash, error) {
	if len(peakLists) == 0 {
		return nil, fmt.Errorf("generatehashes: peakLists is empty")
	}

	// Flatten all peaks from all frames into a single ordered slice.
	// This makes the target zone search a simple forward index scan
	// rather than a nested frame-by-frame search.
	allPeaks := flattenPeaks(peakLists)

	if len(allPeaks) == 0 {
		return nil, fmt.Errorf("generatehashes: no peaks found in peakLists")
	}

	hashes := make([]FingerprintHash, 0, len(allPeaks)*FanOut)

	for i, anchor := range allPeaks {
		// Collect up to FanOut targets from the peaks ahead of this anchor.
		// Targets must be:
		//   1. Later in time than the anchor (forward only)
		//   2. Within TargetZoneFrames frames of the anchor
		targetsFound := 0

		for j := i + 1; j < len(allPeaks) && targetsFound < FanOut; j++ {
			target := allPeaks[j]

			// Calculate the frame distance between anchor and target
			deltaFrames := target.FrameIndex - anchor.FrameIndex

			// Target is beyond the time window — stop searching
			// Since allPeaks is ordered by frame index, no further
			// targets will be within the window either
			if deltaFrames > TargetZoneFrames {
				break
			}

			// deltaFrames should always be >= 0 since j > i and
			// peaks are ordered by frame. Guard explicitly.
			if deltaFrames < 0 {
				continue
			}

			hash := encodeHash(anchor.Bin, target.Bin, deltaFrames)

			hashes = append(hashes, FingerprintHash{
				Hash:       hash,
				SongID:     songID,
				AnchorTime: anchor.FrameIndex,
			})

			targetsFound++
		}
	}

	if len(hashes) == 0 {
		return nil, fmt.Errorf(
			"generatehashes: no hashes produced — audio may be too short or too sparse",
		)
	}

	return hashes, nil
}

// encodeHash packs the anchor frequency bin, target frequency bin,
// and frame delta into a single 32-bit signed integer.
//
// Bit layout:
//
//	[31 ── 23]  anchorFreq  (9 bits)
//	[22 ── 14]  targetFreq  (9 bits)
//	[13 ──  0]  deltaTime   (14 bits)
//
// Each field is masked before shifting to ensure no field bleeds
// into adjacent bits regardless of the input values.
func encodeHash(anchorBin, targetBin, deltaFrames int) int32 {
	anchor := (anchorBin & anchorFreqMask) << anchorFreqShift
	target := (targetBin & targetFreqMask) << targetFreqShift
	delta := deltaFrames & deltaTimeMask

	return int32(anchor | target | delta)
}

// DecodeHash extracts the three component values from a 32-bit hash.
// This is the inverse of encodeHash.
//
// Used in tests to verify round-trip encoding correctness and in
// diagnostics to inspect what a hash represents.
func DecodeHash(hash int32) DecodedHash {
	h := int(hash)
	return DecodedHash{
		AnchorFreq: (h >> anchorFreqShift) & anchorFreqMask,
		TargetFreq: (h >> targetFreqShift) & targetFreqMask,
		DeltaTime:  h & deltaTimeMask,
	}
}

// flattenPeaks converts a []PeakList into a flat []Peak ordered
// by frame index. This simplifies the target zone search in
// GenerateHashes to a single forward pass over a contiguous slice.
//
// The ordering is guaranteed because ExtractPeaks produces PeakLists
// in frame order and peaks within each frame share the same FrameIndex.
func flattenPeaks(peakLists []PeakList) []Peak {
	// Pre-calculate total peak count for a single allocation
	total := 0
	for _, pl := range peakLists {
		total += len(pl.Peaks)
	}

	flat := make([]Peak, 0, total)
	for _, pl := range peakLists {
		flat = append(flat, pl.Peaks...)
	}

	return flat
}

// HashCount returns the total number of hashes in a slice.
// Convenience function for logging and benchmarking.
func HashCount(hashes []FingerprintHash) int {
	return len(hashes)
}