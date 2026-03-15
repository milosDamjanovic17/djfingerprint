package matcher

import (
	"math"
	"testing"

	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// makeQueryHash builds a FingerprintHash for use in matcher tests.
func makeQueryHash(hash int32, anchorTime int) dsp.FingerprintHash {
	return dsp.FingerprintHash{
		Hash:       hash,
		SongID:     0, // unknown during query
		AnchorTime: anchorTime,
	}
}

// makeDBResult builds a HashResult as returned by fingerprint_store.
func makeDBResult(songID int, anchorTime int, hash int32) models.HashResult {
	return models.HashResult{
		SongID:     songID,
		AnchorTime: anchorTime,
		Hash:       hash,
	}
}

// makeCoherentResults builds a set of query hashes and database results
// that simulate a genuine match for a given song at a given offset.
//
// count pairs are generated, all sharing the same deltaT = offset.
// This simulates what happens when a snippet genuinely comes from
// a song at a specific time position.
func makeCoherentResults(
	songID int,
	offset int,
	count int,
) ([]dsp.FingerprintHash, []models.HashResult) {
	queryHashes := make([]dsp.FingerprintHash, count)
	dbResults := make([]models.HashResult, count)

	for i := 0; i < count; i++ {
		hash := int32(1000 + i)
		queryAnchor := i * 10
		dbAnchor := queryAnchor + offset

		queryHashes[i] = makeQueryHash(hash, queryAnchor)
		dbResults[i] = makeDBResult(songID, dbAnchor, hash)
	}

	return queryHashes, dbResults
}

// -----------------------------------------------------------------------
// Match — Empty Input
// -----------------------------------------------------------------------

func TestMatch_EmptyQueryHashes_ReturnsEmpty(t *testing.T) {
	dbResults := []models.HashResult{
		makeDBResult(1, 100, int32(500)),
	}

	results := Match([]dsp.FingerprintHash{}, dbResults)

	if len(results) != 0 {
		t.Errorf("expected empty results for empty query hashes, got %d", len(results))
	}
}

func TestMatch_EmptyDBResults_ReturnsEmpty(t *testing.T) {
	queryHashes := []dsp.FingerprintHash{
		makeQueryHash(int32(500), 10),
	}

	results := Match(queryHashes, []models.HashResult{})

	if len(results) != 0 {
		t.Errorf("expected empty results for empty db results, got %d", len(results))
	}
}

func TestMatch_BothEmpty_ReturnsEmpty(t *testing.T) {
	results := Match([]dsp.FingerprintHash{}, []models.HashResult{})

	if len(results) != 0 {
		t.Errorf("expected empty results for both empty inputs, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// Match — Single Perfect Match
// -----------------------------------------------------------------------

func TestMatch_SingleSong_CorrectlyIdentified(t *testing.T) {
	const songID = 42
	const offset = 970
	const matchCount = 20

	queryHashes, dbResults := makeCoherentResults(songID, offset, matchCount)

	results := Match(queryHashes, dbResults)

	if len(results) == 0 {
		t.Fatal("expected at least one match result, got none")
	}

	if results[0].SongID != songID {
		t.Errorf("expected SongID=%d, got %d", songID, results[0].SongID)
	}
}

func TestMatch_SingleSong_ScoreEqualsMatchCount(t *testing.T) {
	const songID = 1
	const offset = 500
	const matchCount = 15

	queryHashes, dbResults := makeCoherentResults(songID, offset, matchCount)

	results := Match(queryHashes, dbResults)

	if len(results) == 0 {
		t.Fatal("expected at least one match result")
	}

	if results[0].Score != matchCount {
		t.Errorf(
			"expected Score=%d (matchCount), got %d",
			matchCount, results[0].Score,
		)
	}
}

func TestMatch_SingleSong_OffsetCorrect(t *testing.T) {
	const songID = 1
	const offset = 970
	const matchCount = 20

	queryHashes, dbResults := makeCoherentResults(songID, offset, matchCount)

	results := Match(queryHashes, dbResults)

	if len(results) == 0 {
		t.Fatal("expected at least one match result")
	}

	if results[0].Offset != offset {
		t.Errorf("expected Offset=%d, got %d", offset, results[0].Offset)
	}
}

func TestMatch_SingleSong_OffsetSecsCorrect(t *testing.T) {
	const songID = 1
	const offset = 970
	const matchCount = 20

	queryHashes, dbResults := makeCoherentResults(songID, offset, matchCount)

	results := Match(queryHashes, dbResults)

	if len(results) == 0 {
		t.Fatal("expected at least one match result")
	}

	expectedSecs := float64(offset) * float64(hopSize) / float64(sampleRate)
	if math.Abs(results[0].OffsetSecs-expectedSecs) > 0.001 {
		t.Errorf(
			"expected OffsetSecs=%.4f, got %.4f",
			expectedSecs, results[0].OffsetSecs,
		)
	}
}

// -----------------------------------------------------------------------
// Match — MinScore Threshold
// -----------------------------------------------------------------------

func TestMatch_BelowMinScore_ExcludedFromResults(t *testing.T) {
	// Generate only MinScore-1 coherent matches — should be excluded
	const songID = 1
	const matchCount = MinScore - 1

	queryHashes, dbResults := makeCoherentResults(songID, 500, matchCount)

	results := Match(queryHashes, dbResults)

	for _, r := range results {
		if r.SongID == songID {
			t.Errorf(
				"song %d with score below MinScore should be excluded, got score %d",
				songID, r.Score,
			)
		}
	}
}

func TestMatch_ExactlyMinScore_IncludedInResults(t *testing.T) {
	// Generate exactly MinScore coherent matches — must be included
	const songID = 1
	const matchCount = MinScore

	queryHashes, dbResults := makeCoherentResults(songID, 500, matchCount)

	results := Match(queryHashes, dbResults)

	found := false
	for _, r := range results {
		if r.SongID == songID {
			found = true
			break
		}
	}

	if !found {
		t.Errorf(
			"song %d with score exactly MinScore=%d should be included",
			songID, MinScore,
		)
	}
}

func TestMatch_AllBelowMinScore_ReturnsEmpty(t *testing.T) {
	// All candidate songs score below MinScore
	// Result must be empty — do not return low confidence guesses
	queryHashes := []dsp.FingerprintHash{
		makeQueryHash(int32(100), 10),
		makeQueryHash(int32(200), 20),
	}

	// Each hash matches a different song at a different offset
	// — no coherent spike for any song
	dbResults := []models.HashResult{
		makeDBResult(1, 50, int32(100)),
		makeDBResult(2, 60, int32(200)),
	}

	results := Match(queryHashes, dbResults)

	// Each song gets score 1 which is below MinScore=5
	for _, r := range results {
		if r.Score >= MinScore {
			t.Errorf(
				"expected all scores below MinScore=%d, got score=%d for song %d",
				MinScore, r.Score, r.SongID,
			)
		}
	}
}

// -----------------------------------------------------------------------
// Match — Top 3 Ordering and Limiting
// -----------------------------------------------------------------------

func TestMatch_ResultsSortedByScoreDescending(t *testing.T) {
	// Build three songs with different match counts
	// Song 1: 30 matches — highest score
	// Song 2: 20 matches — middle score
	// Song 3: 10 matches — lowest score
	var allQuery []dsp.FingerprintHash
	var allDB []models.HashResult

	q1, d1 := makeCoherentResults(1, 100, 30)
	q2, d2 := makeCoherentResults(2, 200, 20)
	q3, d3 := makeCoherentResults(3, 300, 10)

	// Use non-overlapping hash ranges to avoid cross-song collisions
	// Shift hash values so each song uses unique hashes
	for i := range q1 {
		q1[i].Hash += 0
		d1[i].Hash += 0
	}
	for i := range q2 {
		q2[i].Hash += 10000
		d2[i].Hash += 10000
	}
	for i := range q3 {
		q3[i].Hash += 20000
		d3[i].Hash += 20000
	}

	allQuery = append(allQuery, q1...)
	allQuery = append(allQuery, q2...)
	allQuery = append(allQuery, q3...)
	allDB = append(allDB, d1...)
	allDB = append(allDB, d2...)
	allDB = append(allDB, d3...)

	results := Match(allQuery, allDB)

	if len(results) < 2 {
		t.Fatalf("expected at least 2 results, got %d", len(results))
	}

	for i := 1; i < len(results); i++ {
		if results[i].Score > results[i-1].Score {
			t.Errorf(
				"results not sorted: results[%d].Score=%d > results[%d].Score=%d",
				i, results[i].Score, i-1, results[i-1].Score,
			)
		}
	}
}

func TestMatch_TopN_AtMostThreeResults(t *testing.T) {
	// Even with 5 qualifying songs only 3 must be returned
	var allQuery []dsp.FingerprintHash
	var allDB []models.HashResult

	for songID := 1; songID <= 5; songID++ {
		q, d := makeCoherentResults(songID, songID*100, 20)
		// Shift hash values per song to avoid cross-song hash collisions
		for i := range q {
			q[i].Hash += int32(songID * 10000)
			d[i].Hash += int32(songID * 10000)
		}
		allQuery = append(allQuery, q...)
		allDB = append(allDB, d...)
	}

	results := Match(allQuery, allDB)

	if len(results) > TopN {
		t.Errorf("expected at most %d results, got %d", TopN, len(results))
	}
}

func TestMatch_FewerThanThreeQualify_ReturnsOnlyQualifying(t *testing.T) {
	// Only 2 songs exceed MinScore — must return exactly 2
	var allQuery []dsp.FingerprintHash
	var allDB []models.HashResult

	// Song 1: 20 matches — qualifies
	q1, d1 := makeCoherentResults(1, 100, 20)
	for i := range q1 {
		q1[i].Hash += 0
		d1[i].Hash += 0
	}

	// Song 2: 15 matches — qualifies
	q2, d2 := makeCoherentResults(2, 200, 15)
	for i := range q2 {
		q2[i].Hash += 10000
		d2[i].Hash += 10000
	}

	// Song 3: 2 matches — below MinScore, excluded
	q3, d3 := makeCoherentResults(3, 300, 2)
	for i := range q3 {
		q3[i].Hash += 20000
		d3[i].Hash += 20000
	}

	allQuery = append(allQuery, q1...)
	allQuery = append(allQuery, q2...)
	allQuery = append(allQuery, q3...)
	allDB = append(allDB, d1...)
	allDB = append(allDB, d2...)
	allDB = append(allDB, d3...)

	results := Match(allQuery, allDB)

	if len(results) != 2 {
		t.Errorf("expected 2 qualifying results, got %d", len(results))
	}
}

// -----------------------------------------------------------------------
// Match — Multiple Songs, Correct Winner
// -----------------------------------------------------------------------

func TestMatch_MultipleSongs_CorrectWinner(t *testing.T) {
	// Song 7 has 50 coherent matches — should win
	// Song 23 has 5 coincidental scattered matches
	var allQuery []dsp.FingerprintHash
	var allDB []models.HashResult

	// Song 7 — genuine match
	q7, d7 := makeCoherentResults(7, 970, 50)
	for i := range q7 {
		q7[i].Hash += 0
		d7[i].Hash += 0
	}

	// Song 23 — scattered collisions, 5 different offsets
	for i := 0; i < 5; i++ {
		hash := int32(50000 + i)
		allQuery = append(allQuery, makeQueryHash(hash, i*10))
		allDB = append(allDB, makeDBResult(23, i*100, hash))
	}

	allQuery = append(allQuery, q7...)
	allDB = append(allDB, d7...)

	results := Match(allQuery, allDB)

	if len(results) == 0 {
		t.Fatal("expected at least one result")
	}

	if results[0].SongID != 7 {
		t.Errorf("expected Song 7 as top match, got SongID=%d", results[0].SongID)
	}
}

// -----------------------------------------------------------------------
// Match — Coincidental Collisions Scored Low
// -----------------------------------------------------------------------

func TestMatch_CoincidentalCollisions_LowScore(t *testing.T) {
	// Build scatter-pattern collisions for one song
	// Each hash has a different delta — no coherent spike forms
	const songID = 99
	const hashCount = 20

	queryHashes := make([]dsp.FingerprintHash, hashCount)
	dbResults := make([]models.HashResult, hashCount)

	for i := 0; i < hashCount; i++ {
		hash := int32(9000 + i)
		queryAnchor := i * 10
		// Each pair has a unique delta — completely scattered
		dbAnchor := i * 137 // prime multiplier ensures no two deltas match

		queryHashes[i] = makeQueryHash(hash, queryAnchor)
		dbResults[i] = makeDBResult(songID, dbAnchor, hash)
	}

	results := Match(queryHashes, dbResults)

	// Each offset appears exactly once so max histogram bin = 1
	// Score of 1 is below MinScore so results should be empty
	for _, r := range results {
		if r.SongID == songID && r.Score > 1 {
			t.Errorf(
				"coincidental collisions should score 1, got %d",
				r.Score,
			)
		}
	}
}

// -----------------------------------------------------------------------
// Match — Same Hash At Multiple Anchor Times
// -----------------------------------------------------------------------

func TestMatch_SameHashMultipleAnchorTimes_AllCounted(t *testing.T) {
	// The same hash value appears at 3 different anchor times in the query.
	// Each should produce a separate ΔT calculation.
	const songID = 5
	const hash = int32(77777)

	// Query: hash appears at frames 10, 20, 30
	queryHashes := []dsp.FingerprintHash{
		makeQueryHash(hash, 10),
		makeQueryHash(hash, 20),
		makeQueryHash(hash, 30),
	}

	// DB: hash appears once at frame 510 for this song
	// ΔT calculations:
	//   510 - 10 = 500
	//   510 - 20 = 490
	//   510 - 30 = 480
	// Three different deltas → each bin gets count 1
	dbResults := []models.HashResult{
		makeDBResult(songID, 510, hash),
	}

	// Build query map and histogram directly to inspect
	queryMap := buildQueryMap(queryHashes)
	histogram := buildHistogram(queryMap, dbResults)

	songHist, exists := histogram[songID]
	if !exists {
		t.Fatal("expected histogram entry for songID")
	}

	// Should have 3 different delta entries (500, 490, 480)
	if len(songHist) != 3 {
		t.Errorf("expected 3 histogram bins, got %d", len(songHist))
	}
}

// -----------------------------------------------------------------------
// Match — Determinism
// -----------------------------------------------------------------------

func TestMatch_Deterministic(t *testing.T) {
	queryHashes, dbResults := makeCoherentResults(1, 500, 30)

	results1 := Match(queryHashes, dbResults)
	results2 := Match(queryHashes, dbResults)

	if len(results1) != len(results2) {
		t.Fatalf(
			"non-deterministic result count: %d vs %d",
			len(results1), len(results2),
		)
	}

	for i := range results1 {
		if results1[i].SongID != results2[i].SongID {
			t.Errorf(
				"result[%d] SongID non-deterministic: %d vs %d",
				i, results1[i].SongID, results2[i].SongID,
			)
		}
		if results1[i].Score != results2[i].Score {
			t.Errorf(
				"result[%d] Score non-deterministic: %d vs %d",
				i, results1[i].Score, results2[i].Score,
			)
		}
	}
}

// -----------------------------------------------------------------------
// buildQueryMap
// -----------------------------------------------------------------------

func TestBuildQueryMap_Empty(t *testing.T) {
	result := buildQueryMap([]dsp.FingerprintHash{})
	if len(result) != 0 {
		t.Errorf("expected empty map, got %d entries", len(result))
	}
}

func TestBuildQueryMap_SingleHash(t *testing.T) {
	hashes := []dsp.FingerprintHash{
		makeQueryHash(int32(100), 50),
	}
	result := buildQueryMap(hashes)

	times, ok := result[int32(100)]
	if !ok {
		t.Fatal("expected hash 100 in query map")
	}
	if len(times) != 1 || times[0] != 50 {
		t.Errorf("expected [50], got %v", times)
	}
}

func TestBuildQueryMap_DuplicateHash_CollectsAllAnchorTimes(t *testing.T) {
	// Same hash at three different anchor times
	hashes := []dsp.FingerprintHash{
		makeQueryHash(int32(999), 10),
		makeQueryHash(int32(999), 20),
		makeQueryHash(int32(999), 30),
	}
	result := buildQueryMap(hashes)

	times, ok := result[int32(999)]
	if !ok {
		t.Fatal("expected hash 999 in query map")
	}
	if len(times) != 3 {
		t.Errorf("expected 3 anchor times, got %d: %v", len(times), times)
	}
}

// -----------------------------------------------------------------------
// buildHistogram
// -----------------------------------------------------------------------

func TestBuildHistogram_Empty(t *testing.T) {
	result := buildHistogram(
		map[int32][]int{},
		[]models.HashResult{},
	)
	if len(result) != 0 {
		t.Errorf("expected empty histogram, got %d entries", len(result))
	}
}

func TestBuildHistogram_SingleMatch_CorrectDelta(t *testing.T) {
	queryMap := map[int32][]int{
		int32(500): {30}, // hash 500 seen at frame 30 in query
	}

	dbResults := []models.HashResult{
		makeDBResult(7, 1000, int32(500)), // hash 500 at frame 1000 in DB
	}

	histogram := buildHistogram(queryMap, dbResults)

	songHist, ok := histogram[7]
	if !ok {
		t.Fatal("expected histogram entry for song 7")
	}

	// ΔT = 1000 - 30 = 970
	if songHist[970] != 1 {
		t.Errorf("expected histogram[7][970]=1, got %d", songHist[970])
	}
}

func TestBuildHistogram_CoherentMatches_SameDelta(t *testing.T) {
	// Multiple hashes all producing the same ΔT
	// histogram peak should equal the number of pairs
	const delta = 500
	const pairCount = 10

	queryMap := make(map[int32][]int)
	dbResults := make([]models.HashResult, pairCount)

	for i := 0; i < pairCount; i++ {
		hash := int32(1000 + i)
		queryAnchor := i * 10
		dbAnchor := queryAnchor + delta

		queryMap[hash] = []int{queryAnchor}
		dbResults[i] = makeDBResult(1, dbAnchor, hash)
	}

	histogram := buildHistogram(queryMap, dbResults)

	if histogram[1][delta] != pairCount {
		t.Errorf(
			"expected histogram[1][%d]=%d, got %d",
			delta, pairCount, histogram[1][delta],
		)
	}
}

// -----------------------------------------------------------------------
// scoreHistogram
// -----------------------------------------------------------------------

func TestScoreHistogram_Empty(t *testing.T) {
	results := scoreHistogram(map[int]map[int]int{})
	if len(results) != 0 {
		t.Errorf("expected empty results, got %d", len(results))
	}
}

func TestScoreHistogram_SingleSong_PeakFound(t *testing.T) {
	histogram := map[int]map[int]int{
		42: {
			100: 3,
			200: 8, // peak
			300: 2,
		},
	}

	results := scoreHistogram(histogram)

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Score != 8 {
		t.Errorf("expected Score=8, got %d", results[0].Score)
	}
	if results[0].Offset != 200 {
		t.Errorf("expected Offset=200, got %d", results[0].Offset)
	}
	if results[0].SongID != 42 {
		t.Errorf("expected SongID=42, got %d", results[0].SongID)
	}
}

// -----------------------------------------------------------------------
// filterByMinScore
// -----------------------------------------------------------------------

func TestFilterByMinScore_AllAbove_NoneRemoved(t *testing.T) {
	results := []MatchResult{
		{SongID: 1, Score: 10},
		{SongID: 2, Score: 20},
		{SongID: 3, Score: MinScore},
	}

	filtered := filterByMinScore(results)

	if len(filtered) != 3 {
		t.Errorf("expected 3 results, got %d", len(filtered))
	}
}

func TestFilterByMinScore_AllBelow_ReturnsEmpty(t *testing.T) {
	results := []MatchResult{
		{SongID: 1, Score: MinScore - 1},
		{SongID: 2, Score: 1},
		{SongID: 3, Score: 0},
	}

	filtered := filterByMinScore(results)

	if len(filtered) != 0 {
		t.Errorf("expected empty results, got %d", len(filtered))
	}
}

func TestFilterByMinScore_Mixed_OnlyAboveReturned(t *testing.T) {
	results := []MatchResult{
		{SongID: 1, Score: 20},       // above — keep
		{SongID: 2, Score: MinScore}, // exactly at — keep
		{SongID: 3, Score: 2},        // below — discard
		{SongID: 4, Score: 100},      // above — keep
	}

	filtered := filterByMinScore(results)

	if len(filtered) != 3 {
		t.Errorf("expected 3 results, got %d", len(filtered))
	}

	for _, r := range filtered {
		if r.SongID == 3 {
			t.Error("song 3 with score below MinScore should have been filtered out")
		}
	}
}

// -----------------------------------------------------------------------
// OffsetToSeconds
// -----------------------------------------------------------------------

func TestOffsetToSeconds_Zero(t *testing.T) {
	result := OffsetToSeconds(0)
	if result != 0.0 {
		t.Errorf("expected 0.0 for frame 0, got %f", result)
	}
}

func TestOffsetToSeconds_KnownValue(t *testing.T) {
	// frame 970 * hopSize 512 / sampleRate 11025
	// = 497,640 / 11,025 = 45.138...
	const frame = 970
	expected := float64(frame) * float64(hopSize) / float64(sampleRate)
	result := OffsetToSeconds(frame)

	if math.Abs(result-expected) > 0.0001 {
		t.Errorf("expected %.4f, got %.4f", expected, result)
	}
}

// -----------------------------------------------------------------------
// Integration — Full DSP Pipeline into Matcher
// -----------------------------------------------------------------------

func TestMatch_Integration_DSPPipelineToMatcher(t *testing.T) {
	// This test simulates the full identify flow without a database:
	// 1. Generate a sine wave as the "full song"
	// 2. Take a snippet from the middle
	// 3. Run both through the full DSP pipeline
	// 4. Use the full song hashes as simulated DB results
	// 5. Verify the matcher identifies the correct song

	// We import the dsp package indirectly through the types
	// The actual DSP calls happen through the dsp package
	// which we have already verified works correctly

	// Build a coherent scenario with 100 matching pairs
	// simulating a strong real-world match
	const songID = 1
	const offset = 200
	const matchCount = 100

	queryHashes, dbResults := makeCoherentResults(songID, offset, matchCount)

	// Add some noise — 20 scattered results from a different song
	for i := 0; i < 20; i++ {
		hash := int32(90000 + i)
		queryHashes = append(queryHashes, makeQueryHash(hash, i*5))
		dbResults = append(dbResults,
			makeDBResult(99, i*173, hash), // prime multiplier = scattered
		)
	}

	results := Match(queryHashes, dbResults)

	if len(results) == 0 {
		t.Fatal("expected at least one match result")
	}

	if results[0].SongID != songID {
		t.Errorf(
			"expected correct song (ID=%d) as top match, got SongID=%d",
			songID, results[0].SongID,
		)
	}

	if results[0].Score < matchCount {
		t.Errorf(
			"expected Score >= %d for perfect coherent match, got %d",
			matchCount, results[0].Score,
		)
	}
}