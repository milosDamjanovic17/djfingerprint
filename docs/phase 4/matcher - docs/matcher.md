# `matcher.go` — Introduction Document

---

## What `matcher.go` Does

```
Receives  → []models.HashResult from fingerprint_store.QueryByHashes
            []dsp.FingerprintHash from the query snippet DSP pipeline
Does      → groups results by song ID
            for each song builds a time-offset histogram
            calculates a coherence score per song
            selects the Top 3 songs by coherence score
Returns   → []MatchResult sorted by score descending
```

## What `matcher.go` Does NOT Do

```
Does not generate hashes          ← dsp/fingerprint.go
Does not query the database       ← store/fingerprint_store.go
Does not decode audio             ← audio/decoder.go
Does not know about HTTP          ← api/handlers/identify.go
Does not write to the database    ← store layer
Does not return more than Top 3   ← by design
```

---

## The Core Problem Being Solved

A raw hash lookup returns too much data to be useful on its own. Consider what `QueryByHashes` returns for a 15-second snippet against a library of 500 songs:

```
Query hashes:  ~1,500 hashes from the snippet
Result rows:   potentially thousands of matching rows
               spread across dozens of songs
               many are coincidental collisions
```

The matcher's job is to **separate genuine matches from coincidental collisions** and produce a ranked confidence score for each candidate song.

---

## Why Raw Hash Count Is Not Enough

The naive approach — count how many query hashes match each song, rank by count — fails for two reasons:

### Reason 1 — Hash Collisions Are Common

A 32-bit hash has $2^{32} = 4{,}294{,}967{,}296$ possible values. With 58,000 hashes per song and 500 songs in the library that is 29 million stored hashes. The probability of random collisions between a query snippet and unrelated songs is non-trivial.

### Reason 2 — The Same Song Played at Different Speeds

DJ use cases include tracks played at slightly different BPMs (tempo-stretched). Raw hash counts would fail here because pitch-shifted audio produces slightly different hashes.

---

## The Solution — Coherence Scoring

A genuine match has a property that coincidental collisions do not:

> **All matching hashes between a snippet and its source song share the same time offset.**

If the snippet starts at second 45 of the full song, then every hash that matches between the snippet and the full song will have an anchor time difference of exactly $\Delta T = 45 \text{ seconds}$ (in frame units).

Coincidental collisions between the snippet and unrelated songs will have **random, scattered time offsets** — no consistent $\Delta T$.

This is the coherence test.

---

## The Time Offset Histogram

For each candidate song, the matcher builds a histogram of time offsets:

$$\Delta T = \text{dbAnchorTime} - \text{queryAnchorTime}$$

Where:
- `dbAnchorTime` is the `AnchorTime` from the database result (the stored song)
- `queryAnchorTime` is the `AnchorTime` from the query hash (the snippet)

```
Genuine match (snippet starts at frame 970 of the song):
  Hash 1: dbAnchor=1000, queryAnchor=30  → ΔT = 970
  Hash 2: dbAnchor=1050, queryAnchor=80  → ΔT = 970
  Hash 3: dbAnchor=1100, queryAnchor=130 → ΔT = 970
  Histogram: { 970: 3 }   ← strong peak at one offset

Coincidental collision (unrelated song):
  Hash 1: dbAnchor=200,  queryAnchor=30  → ΔT = 170
  Hash 2: dbAnchor=1300, queryAnchor=80  → ΔT = 1220
  Hash 3: dbAnchor=500,  queryAnchor=130 → ΔT = 370
  Histogram: { 170:1, 1220:1, 370:1 }   ← scattered, no peak
```

The **coherence score** for each song is the **maximum bin count** in its histogram — the height of the tallest spike:

$$\text{score} = \max(\text{histogram bins})$$

---

## Worked Example

```
Query snippet: 15 seconds from track "Acid Rain" by Objekt
Library: 500 songs

QueryByHashes returns 4,200 matching rows across many songs

Matcher builds histograms:

Song ID 7  (Acid Rain — Objekt):
  Histogram: { 970: 847, 971: 12, 969: 8, ... }
  Score: 847  ← massive peak at one offset

Song ID 23 (unrelated track):
  Histogram: { 100:3, 450:2, 780:1, 1200:2, ... }
  Score: 3    ← scattered, no peak

Song ID 41 (unrelated track):
  Histogram: { 200:1, 650:4, 900:2, ... }
  Score: 4    ← scattered

Top 3 by score:
  1. Song ID 7  — score 847  ← correct match
  2. Song ID 41 — score 4
  3. Song ID 23 — score 3
```

---

## The `MatchResult` Struct

```
MatchResult
  ├── SongID      int     — the candidate song's database ID
  ├── Score       int     — coherence score (max histogram bin)
  ├── Offset      int     — the frame offset at which the match peaks
  └── OffsetSecs  float64 — Offset converted to seconds for the response
```

`OffsetSecs` tells the frontend approximately where in the full song the snippet came from. Useful for DJ context — "this snippet is from 1:37 into the track".

$$\text{OffsetSecs} = \frac{\text{Offset} \times \text{HopSize}}{\text{SampleRate}} = \frac{\text{Offset} \times 512}{11025}$$

---

## The Minimum Score Threshold

A match is only included in the Top 3 if its score exceeds a minimum threshold:

```go
const MinScore = 5
```

Songs with a coherence score below 5 are discarded entirely — they are almost certainly coincidental collisions rather than genuine matches. This prevents the API from returning low-confidence garbage results when the audio is unrecognised.

If no song exceeds the threshold the API returns an empty result set with a clear "no match found" response rather than returning the three least-wrong guesses.

---

## The Lookup Map — How Query Hashes Are Indexed

To calculate $\Delta T$ efficiently, the matcher needs to look up the `queryAnchorTime` for a given hash value in O(1) time.

Before processing the database results, the query hashes are indexed into a map:

```go
// key: hash value
// value: list of anchor times in the snippet that produced this hash
queryMap := map[int32][]int{}
for _, qh := range queryHashes {
    queryMap[qh.Hash] = append(queryMap[qh.Hash], qh.AnchorTime)
}
```

Then for each database result:

```go
for _, result := range dbResults {
    queryAnchorTimes := queryMap[result.Hash]
    for _, queryAnchor := range queryAnchorTimes {
        delta := result.AnchorTime - queryAnchor
        histogram[result.SongID][delta]++
    }
}
```

---

## Why Multiple Query Anchor Times Per Hash

The same hash value can appear multiple times in the query snippet at different anchor times. This happens legitimately when:
- The same frequency pattern repeats in the audio (e.g. a repeating bassline)
- The fingerprint fan-out creates the same hash from different anchor positions

The `queryMap` stores all anchor times for a given hash value so none are missed.

---

## What the Tests Will Cover

```
[ ] Empty query hashes returns empty result
[ ] Empty database results returns empty result
[ ] Single perfect match identified correctly
[ ] Score reflects histogram peak count accurately
[ ] OffsetSecs calculated correctly from frame offset
[ ] Below MinScore results excluded from Top 3
[ ] Top 3 ordering — highest score first
[ ] When fewer than 3 matches exceed threshold, return only those
[ ] When zero matches exceed threshold, return empty slice
[ ] Multiple songs — correct song ranked first
[ ] Coincidental collisions scored correctly (low score)
[ ] Same hash appearing at multiple anchor times handled correctly
[ ] Deterministic — same input always produces same output
```

---

## Where `matcher.go` Sits in the Full Identify Flow

```
POST /api/v1/identify
        │
        ▼
handlers/identify.go
  Decode audio bytes → PCM
  Normalize → Spectrogram → Peaks → GenerateHashes
        │
        ▼ queryHashes []dsp.FingerprintHash
        │
        ▼
fingerprint_store.QueryByHashes(queryHashes)
        │
        ▼ dbResults []models.HashResult
        │
        ▼
matcher.Match(queryHashes, dbResults)
        │
        ▼ []MatchResult (Top 3, sorted by score)
        │
        ▼
handlers/identify.go
  Build JSON response → return to client
```

---

## File Location and Package

```
internal/matcher/matcher.go
internal/matcher/matcher_test.go

package matcher
```

Imports needed:
```
github.com/milosDamjanovic17/djfingerprint/internal/dsp
github.com/milosDamjanovic17/djfingerprint/internal/models
```

---

# `matcher.go` & `matcher_test.go` — Build Summary

---

## What We Built

### The Purpose
`matcher.go` is the **intelligence layer of the identification system**. It receives raw hash lookup results from the database and transforms them into a ranked, confidence-scored list of candidate songs. Without the matcher, a hash lookup returns thousands of rows spread across dozens of songs with no way to distinguish a genuine match from a coincidental collision. The matcher solves this using coherence scoring — the same fundamental approach used in the original Shazam algorithm.

---

## `matcher.go` — What Was Built and Why

### Constants Defined

```go
const (
    MinScore   = 5
    TopN       = 3
    hopSize    = 512
    sampleRate = 11025
)
```

| Constant | Value | Reason |
|----------|-------|--------|
| `MinScore` | `5` | Minimum coherence score to be included in results. Songs below this are almost certainly coincidental hash collisions |
| `TopN` | `3` | Maximum results returned. The API always returns at most 3 candidates |
| `hopSize` | `512` | Mirrors `dsp/spectrogram.go` — used to convert frame offsets to seconds without importing DSP internals |
| `sampleRate` | `11025` | Mirrors `dsp/spectrogram.go` — same reason |

**Why `hopSize` and `sampleRate` are redefined locally rather than imported:**
The matcher is a scoring algorithm — it should not depend on DSP implementation details. If the spectrogram parameters ever change, the matcher constants are updated explicitly and intentionally rather than silently inheriting a change from another package.

---

### The `MatchResult` Struct

```go
type MatchResult struct {
    SongID     int
    Score      int
    Offset     int
    OffsetSecs float64
}
```

| Field | Content |
|-------|---------|
| `SongID` | Database ID of the candidate song |
| `Score` | Coherence score — height of the tallest histogram spike |
| `Offset` | Frame index at which the histogram peaks — where in the song the snippet came from |
| `OffsetSecs` | `Offset` converted to seconds for the API response |

`OffsetSecs` is the user-facing value — telling the frontend "this snippet came from approximately 45 seconds into the track" is meaningful context for a DJ application.

$$\text{OffsetSecs} = \frac{\text{Offset} \times 512}{11025}$$

---

### `Match(queryHashes, dbResults) []MatchResult`

The primary function. Six steps from raw database results to ranked Top 3.

**Step by step:**

```
1. Guard: empty queryHashes or dbResults → return empty slice immediately
        ↓
2. buildQueryMap(queryHashes)
   hash value → []anchorTime
   O(1) lookup during histogram construction
        ↓
3. buildHistogram(queryMap, dbResults)
   For each DB result:
     look up query anchor times for that hash
     for each query anchor time:
       ΔT = dbAnchorTime - queryAnchorTime
       histogram[songID][ΔT]++
        ↓
4. scoreHistogram(histogram)
   For each song find the peak bin count
   Record the ΔT at the peak as the Offset
   Convert Offset to OffsetSecs
        ↓
5. filterByMinScore(results)
   Discard results where Score < MinScore
        ↓
6. sort.Slice by Score descending
   Truncate to TopN
   Return
```

---

### Why Coherence Scoring Works

The mathematical insight that makes the algorithm reliable:

**Genuine match — all pairs share the same $\Delta T$:**

If a snippet starts at frame 970 of the full song, then for every matching hash pair:

$$\Delta T = \text{dbAnchorTime} - \text{queryAnchorTime} = 970$$

The histogram has one dominant spike at bin 970.

**Coincidental collision — pairs have random $\Delta T$ values:**

An unrelated song may share some hash values with the snippet by chance, but the anchor times are unrelated so:

$$\Delta T_1 = 170, \quad \Delta T_2 = 1220, \quad \Delta T_3 = 370, \quad \ldots$$

The histogram is flat — no spike forms.

$$\text{coherence score} = \max(\text{histogram bins})$$

A genuine match scores in the hundreds. A coincidental collision scores 1–3.

---

### `buildQueryMap(queryHashes) map[int32][]int`

Indexes query hashes by hash value before the main scoring loop. This is a critical performance optimisation — without it, finding the query anchor time for each database result would require a linear scan through the query hashes for every database row.

**Why values are `[]int` not `int`:**
The same hash value can appear at multiple anchor times in the query snippet — a repeating bassline or rhythmic pattern generates the same hash at each repetition. All anchor times must be collected and processed:

```go
queryMap[qh.Hash] = append(queryMap[qh.Hash], qh.AnchorTime)
```

Missing any of these would undercount coherent matches and reduce the score.

---

### `buildHistogram(queryMap, dbResults) map[int]map[int]int`

The core computation. Processes every database result and increments the appropriate histogram bin.

```go
for _, result := range dbResults {
    queryAnchorTimes := queryMap[result.Hash]
    for _, queryAnchor := range queryAnchorTimes {
        delta := result.AnchorTime - queryAnchor
        histogram[result.SongID][delta]++
    }
}
```

**Two-level map structure:**
```
histogram
  └── songID (int)
        └── deltaT (int) → count (int)
```

Each song maintains its own independent histogram. Songs never interfere with each other's counts.

**Lazy initialisation:**
```go
if _, exists := histogram[result.SongID]; !exists {
    histogram[result.SongID] = make(map[int]int)
}
```

The inner map is only created when a song is first encountered — no pre-allocation needed because the set of matching song IDs is not known until the results are processed.

---

### `scoreHistogram(histogram) []MatchResult`

Converts the histogram into `[]MatchResult` by finding the peak bin for each song:

```go
for songID, offsets := range histogram {
    bestScore := 0
    bestOffset := 0

    for delta, count := range offsets {
        if count > bestScore {
            bestScore = count
            bestOffset = delta
        }
    }
    // ...
}
```

The peak bin count becomes `Score`. The peak bin key becomes `Offset`. Both are stored in the `MatchResult` along with the calculated `OffsetSecs`.

---

### `filterByMinScore(results) []MatchResult`

Removes results below `MinScore`. Returns a new slice — the original is not modified.

**Why `MinScore=5` specifically:**
A score of 5 means 5 hash pairs all agreed on exactly the same time offset. The probability of 5 coincidental collisions all producing the same $\Delta T$ is extremely low. A score of 1–4 is almost certainly noise. A score of 5+ indicates at least a weak genuine signal.

For a strong match (full song in a quiet environment) the score is typically 200–800. For a degraded match (background noise, crowd noise) the score is typically 20–100. MinScore=5 is deliberately permissive — it excludes pure noise while capturing degraded matches.

---

### `OffsetToSeconds(frameOffset int) float64`

Exported utility function. Converts a frame offset to seconds:

$$\text{seconds} = \frac{\text{frameOffset} \times 512}{11025}$$

Used by handlers when building the API response.

---

## The Four Private Functions — Why Separated

The `Match` function is decomposed into four private functions rather than written as one large function. Each step is independently testable:

| Function | Tests Written |
|----------|--------------|
| `buildQueryMap` | 3 unit tests including duplicate hash handling |
| `buildHistogram` | 3 unit tests including coherent delta accumulation |
| `scoreHistogram` | 2 unit tests including peak detection |
| `filterByMinScore` | 3 unit tests including boundary at exactly MinScore |

This decomposition meant that when writing the integration test, each component's correctness was already individually verified. The integration test only needed to verify that the components work together correctly.

---

## `matcher_test.go` — What Was Tested and Why

### Test Helper Functions

Three helpers established for all matcher tests:

**`makeQueryHash(hash int32, anchorTime int) dsp.FingerprintHash`**
Builds a query-mode fingerprint hash with `SongID=0` (unknown). Used to build query snippets without needing the DSP pipeline.

**`makeDBResult(songID int, anchorTime int, hash int32) models.HashResult`**
Builds a database result row as `fingerprint_store.QueryByHashes` would return. Used to simulate database responses without needing PostgreSQL.

**`makeCoherentResults(songID int, offset int, count int) ([]FingerprintHash, []HashResult)`**
The most important helper. Generates `count` matching pairs where every pair shares exactly `offset` as their $\Delta T$:

```go
queryAnchor := i * 10
dbAnchor := queryAnchor + offset
// ΔT = dbAnchor - queryAnchor = offset (always)
```

This simulates a genuine match with a known score and offset. Used across 12 tests.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestMatch_EmptyQueryHashes_ReturnsEmpty` | Empty query returns empty slice |
| `TestMatch_EmptyDBResults_ReturnsEmpty` | Empty DB results returns empty slice |
| `TestMatch_BothEmpty_ReturnsEmpty` | Both empty returns empty slice |
| `TestMatch_SingleSong_CorrectlyIdentified` | Correct SongID in top result |
| `TestMatch_SingleSong_ScoreEqualsMatchCount` | Score equals number of coherent pairs |
| `TestMatch_SingleSong_OffsetCorrect` | Offset field matches the known time offset |
| `TestMatch_SingleSong_OffsetSecsCorrect` | OffsetSecs calculated correctly from frame offset |
| `TestMatch_BelowMinScore_ExcludedFromResults` | Score < MinScore excluded |
| `TestMatch_ExactlyMinScore_IncludedInResults` | Score == MinScore included |
| `TestMatch_AllBelowMinScore_ReturnsEmpty` | All below threshold returns empty |
| `TestMatch_ResultsSortedByScoreDescending` | Results ordered highest score first |
| `TestMatch_TopN_AtMostThreeResults` | 5 qualifying songs capped at 3 results |
| `TestMatch_FewerThanThreeQualify_ReturnsOnlyQualifying` | 2 qualifying songs returns exactly 2 |
| `TestMatch_MultipleSongs_CorrectWinner` | Genuine match outscores scattered collisions |
| `TestMatch_CoincidentalCollisions_LowScore` | Scattered matches score 1, below MinScore |
| `TestMatch_SameHashMultipleAnchorTimes_AllCounted` | Duplicate hash values produce separate ΔT entries |
| `TestMatch_Deterministic` | Same input always produces same output |
| `TestBuildQueryMap_Empty` | Empty input returns empty map |
| `TestBuildQueryMap_SingleHash` | Single hash stored with correct anchor time |
| `TestBuildQueryMap_DuplicateHash_CollectsAllAnchorTimes` | Three identical hashes collect all three anchor times |
| `TestBuildHistogram_Empty` | Empty inputs return empty histogram |
| `TestBuildHistogram_SingleMatch_CorrectDelta` | Single pair produces correct ΔT bin |
| `TestBuildHistogram_CoherentMatches_SameDelta` | 10 coherent pairs produce bin count of 10 |
| `TestScoreHistogram_Empty` | Empty histogram returns empty results |
| `TestScoreHistogram_SingleSong_PeakFound` | Peak bin identified correctly from multi-bin histogram |
| `TestFilterByMinScore_AllAbove_NoneRemoved` | All above threshold unchanged |
| `TestFilterByMinScore_AllBelow_ReturnsEmpty` | All below threshold returns empty |
| `TestFilterByMinScore_Mixed_OnlyAboveReturned` | Mixed scores — only above-threshold returned |
| `TestOffsetToSeconds_Zero` | Frame 0 returns 0.0 seconds |
| `TestOffsetToSeconds_KnownValue` | Frame 970 returns correct seconds value |
| `TestMatch_Integration_DSPPipelineToMatcher` | 100 coherent pairs + 20 noise pairs — correct song wins |

---

## Boundaries Established

`matcher.go` closes out the identification logic. After this point the data is a clean ranked result ready for HTTP serialisation:

```
BEFORE matcher.go                    AFTER matcher.go
─────────────────                    ────────────────
[]models.HashResult            →     []MatchResult
Thousands of raw DB rows             At most 3 ranked candidates
No confidence signal                 Coherence score per candidate
No time offset information           OffsetSecs per candidate
Genuine matches mixed with noise     Noise filtered by MinScore
```

---

## What `matcher.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Hash generation | `dsp/fingerprint.go` |
| Database queries | `store/fingerprint_store.go` |
| Song metadata retrieval | `store/song_store.go` |
| HTTP response building | `api/handlers/identify.go` |
| Audio decoding | `audio/decoder.go` |
| Spectrogram computation | `dsp/spectrogram.go` |

---

## Complete Identification Pipeline — Current State

```
POST /api/v1/identify (audio snippet)
        │
        ▼
  audio/decoder.go + audio/normalizer.go
  Decode → Normalize → *PCMData
        │
        ▼
  dsp/spectrogram.go → dsp/peaks.go → dsp/fingerprint.go
  PCMData → Spectrogram → Peaks → []FingerprintHash
        │
        ▼
  store/fingerprint_store.go — QueryByHashes()
  []FingerprintHash → []models.HashResult (DB lookup)
        │
        ▼
  matcher/matcher.go — Match()
  []HashResult → coherence scoring → []MatchResult (Top 3)
        │
        ▼
  api/handlers/identify.go  ← NEXT
  []MatchResult → JSON response → client
```

---

## Test Results Achieved

```
All tests  → PASS
Integration test match rate → 100% correct winner identified
go vet     → clean
go build   → clean
```

---

## What Comes Next — Step 4: `api/`

With models, store, and matcher all complete, the API layer has everything it needs to wire together:

```
Phase 4 remaining files:

  internal/api/middleware/
    logging.go     ← request/response logging
    recovery.go    ← panic recovery → HTTP 500
    cors.go        ← CORS headers for browser requests

  internal/api/dto/
    request.go     ← upload and identify request shapes
    response.go    ← song, match, and error response shapes

  internal/api/handlers/
    upload.go      ← POST /api/v1/upload
    identify.go    ← POST /api/v1/identify
    songs.go       ← GET/DELETE /api/v1/songs

  internal/api/routes.go   ← wires handlers and middleware

  cmd/api/main.go          ← server entry point
```

> This summary reflects the state of `matcher.go` and `matcher_test.go` as committed. Move forward to `api/` for further development explanation.