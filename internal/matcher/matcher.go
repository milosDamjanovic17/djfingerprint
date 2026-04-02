package matcher

import (
	"sort"

	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

const (
	// MinScore is the minimum absolute coherence score a song must
	// achieve to be included in the Top 3 results.
	// A genuine 10-second match typically produces 50-200+ coherent
	// hashes at the same time offset. Random collisions between
	// unrelated songs rarely exceed 10-15.
	// Set to 20 to cleanly separate genuine matches from noise.
	MinScore = 20

	// MinScoreRatio is the minimum fraction of query hashes that must
	// match coherently. This prevents weak matches when the absolute
	// score technically exceeds MinScore but represents a tiny fraction
	// of the total hashes — likely noise rather than a genuine match.
	// 0.3% of query hashes (e.g. 18 out of 6000 for a 10s snippet).
	MinScoreRatio = 0.003

	// TopN is the maximum number of results returned by Match.
	// The API always returns at most 3 candidates.
	TopN = 3

	// HopSize mirrors the value from dsp/spectrogram.go.
	// Used to convert frame offsets to seconds.
	// Defined here to avoid importing the dsp package constants
	// directly — the matcher should not depend on DSP internals.
	hopSize = 512

	// sampleRate mirrors the value from dsp/spectrogram.go.
	sampleRate = 11025
)

// MatchResult represents a single candidate song identified by
// the coherence scoring algorithm.
// Results are sorted by Score descending — index 0 is the best match.
type MatchResult struct {
	// SongID is the database ID of the candidate song
	SongID int

	// Score is the coherence score — the height of the tallest spike
	// in the time-offset histogram for this song.
	// Higher is more confident.
	Score int

	// Offset is the frame index at which the histogram peaks.
	// Represents how far into the full song the snippet came from.
	Offset int

	// OffsetSecs is Offset converted to seconds.
	// Offset * HopSize / SampleRate
	OffsetSecs float64
}

// Match is the core identification function.
// It takes the hashes generated from the query snippet and the
// matching rows returned from the database, runs coherence scoring,
// and returns the Top 3 candidate songs sorted by confidence.
//
// Returns an empty slice (not an error) when:
//   - queryHashes is empty
//   - dbResults is empty
//   - no song achieves a score >= MinScore
//
// This is intentional — "no match found" is a valid outcome.
func Match(
	queryHashes []dsp.FingerprintHash,
	dbResults []models.HashResult,
) []MatchResult {
	if len(queryHashes) == 0 || len(dbResults) == 0 {
		return []MatchResult{}
	}

	// Step 1: Build a lookup map from hash value to all anchor times
	// in the query snippet that produced that hash.
	//
	// The same hash value can appear multiple times in the snippet
	// at different anchor times — a repeating bassline for example.
	// We store all of them so none are missed during scoring.
	//
	// key:   hash value (int32)
	// value: slice of anchor frame indices from the query snippet
	queryMap := buildQueryMap(queryHashes)

	// Step 2: Build a per-song time-offset histogram.
	//
	// For each database result, look up the corresponding query
	// anchor times and calculate the time offset:
	//
	//   ΔT = dbAnchorTime - queryAnchorTime
	//
	// A genuine match produces many results at the same ΔT.
	// Coincidental collisions produce results scattered across
	// many different ΔT values with no dominant spike.
	//
	// histogram[songID][deltaT] = count of matches at that offset
	histogram := buildHistogram(queryMap, dbResults)

	// Step 3: For each song find the peak of its histogram.
	// The peak count is the coherence score.
	// Record which ΔT produced the peak — that is the time offset.
	results := scoreHistogram(histogram)

	// Step 4: Filter out results below MinScore and MinScoreRatio.
	// Both thresholds must be met — this catches coincidental
	// collisions that pass one check but not the other.
	results = filterByMinScore(results, len(queryHashes))

	// Step 5: Sort by score descending — best match first.
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// Step 6: Return at most TopN results.
	if len(results) > TopN {
		results = results[:TopN]
	}

	return results
}

// buildQueryMap indexes the query hashes by hash value.
// For each hash value it collects all anchor times in the snippet
// that produced that hash.
//
// This allows O(1) lookup of query anchor times when processing
// each database result row.
func buildQueryMap(queryHashes []dsp.FingerprintHash) map[int32][]int {
	queryMap := make(map[int32][]int, len(queryHashes))

	for _, qh := range queryHashes {
		queryMap[qh.Hash] = append(queryMap[qh.Hash], qh.AnchorTime)
	}

	return queryMap
}

// buildHistogram constructs the per-song time-offset histogram.
//
// For each database result:
//  1. Look up all query anchor times for the matching hash value
//  2. For each query anchor time calculate ΔT = dbAnchor - queryAnchor
//  3. Increment histogram[songID][ΔT]
//
// The result is a nested map:
//
//	songID → { deltaT → count }
func buildHistogram(
	queryMap map[int32][]int,
	dbResults []models.HashResult,
) map[int]map[int]int {
	// histogram[songID][deltaT] = match count
	histogram := make(map[int]map[int]int)

	for _, result := range dbResults {
		queryAnchorTimes, found := queryMap[result.Hash]
		if !found {
			// This hash is in the database but not in the query map.
			// Should not happen since we queried by these hashes,
			// but guard explicitly.
			continue
		}

		// Ensure the song has a histogram entry
		if _, exists := histogram[result.SongID]; !exists {
			histogram[result.SongID] = make(map[int]int)
		}

		for _, queryAnchor := range queryAnchorTimes {
			delta := result.AnchorTime - queryAnchor
			histogram[result.SongID][delta]++
		}
	}

	return histogram
}

// scoreHistogram finds the peak bin in each song's histogram
// and converts it to a MatchResult.
//
// The peak bin count is the coherence score.
// The bin key (ΔT) at the peak is the time offset.
func scoreHistogram(histogram map[int]map[int]int) []MatchResult {
	results := make([]MatchResult, 0, len(histogram))

	for songID, offsets := range histogram {
		bestScore := 0
		bestOffset := 0

		for delta, count := range offsets {
			if count > bestScore {
				bestScore = count
				bestOffset = delta
			}
		}

		// Convert frame offset to seconds
		// OffsetSecs = Offset * HopSize / SampleRate
		offsetSecs := float64(bestOffset) * float64(hopSize) / float64(sampleRate)

		results = append(results, MatchResult{
			SongID:     songID,
			Score:      bestScore,
			Offset:     bestOffset,
			OffsetSecs: offsetSecs,
		})
	}

	return results
}

// filterByMinScore removes results that don't meet both thresholds:
//  1. Absolute: score must be >= MinScore (20)
//  2. Relative: score must be >= MinScoreRatio * totalQueryHashes
//
// The relative check prevents weak matches where the absolute score
// technically passes but represents a negligible fraction of query hashes.
func filterByMinScore(results []MatchResult, totalQueryHashes int) []MatchResult {
	ratioThreshold := int(float64(totalQueryHashes) * MinScoreRatio)

	// Use whichever threshold is higher
	threshold := MinScore
	if ratioThreshold > threshold {
		threshold = ratioThreshold
	}

	filtered := make([]MatchResult, 0, len(results))
	for _, r := range results {
		if r.Score >= threshold {
			filtered = append(filtered, r)
		}
	}
	return filtered
}

// OffsetToSeconds converts a frame offset to seconds.
// Exported for use in handlers and tests.
func OffsetToSeconds(frameOffset int) float64 {
	return float64(frameOffset) * float64(hopSize) / float64(sampleRate)
}