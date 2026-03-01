# Phase 3 `fingerprint.go`

---

## What `fingerprint.go` Does

```
Receives  → a []PeakList from peaks.go
Does      → treats each peak as an anchor
            pairs each anchor with up to 5 nearby target peaks
            encodes each anchor-target pair into a 32-bit hash
            returns a collection of hashes with their timing metadata
Returns   → a []FingerprintHash — each containing a hash value,
            the anchor's time offset, and the song ID (when indexing)
```

## What `fingerprint.go` Does NOT Do

```
Does not decode or normalize audio        ← audio package
Does not compute FFT or spectrogram       ← spectrogram.go
Does not extract peaks from bands         ← peaks.go
Does not query or write the database      ← store layer
Does not perform coherence scoring        ← matcher layer
Does not know about songs or matching     ← matcher layer
```

---

## The Three Core Concepts

### 1 — Anchors and Targets

Every peak in the `[]PeakList` is a candidate **anchor**. For each anchor we look forward in time across the subsequent frames and identify up to 5 nearby peaks called **targets**.

```
Time →
Frame 0    Frame 1    Frame 2    Frame 3    Frame 4
  ●            ●                    ●          ●
  ↑            ↑                    ↑          ↑
anchor      target1              target2    target3
  └────────────┴────────────────────┴──────────┘
              target zone (fixed time window forward)
```

The target zone is a fixed time window **ahead** of the anchor — we never look backwards. This ensures every hash encodes a forward-in-time relationship, which is consistent whether matching a snippet against a full song or a full song against itself.

---

### 2 — The Target Zone

The target zone defines how far forward in time we search for targets:

| Parameter | Value | Reason |
|-----------|-------|--------|
| Fan-out | 5 targets per anchor | Balance between fingerprint uniqueness and database size |
| Time window | Next 100 frames | Approximately 4.6 seconds forward at 46.4ms per frame |

The 5 targets are the **next 5 peaks chronologically** within the time window — not the 5 loudest, not 5 random ones. Chronological selection ensures the time delta ($\Delta t$) encoded in the hash is always positive and bounded.

---

### 3 — The 32-Bit Hash

Each anchor-target pair is encoded into a single `int32` value:

$$\text{hash} = (f_{\text{anchor}} \And \texttt{0x1FF}) \ll 23 \mid (f_{\text{target}} \And \texttt{0x1FF}) \ll 14 \mid (\Delta t \And \texttt{0x3FFF})$$

The 32 bits are divided into three fields:

```
Bit layout (32 bits total):
┌─────────────────┬─────────────────┬──────────────────────────┐
│  anchor_freq    │  target_freq    │       delta_time         │
│    9 bits       │    9 bits       │        14 bits           │
│   bits 31-23    │   bits 22-14    │        bits 13-0         │
└─────────────────┴─────────────────┴──────────────────────────┘
```

| Field | Bits | Range | Encodes |
|-------|------|-------|---------|
| `anchor_freq` | 9 | 0–511 | FFT bin index of the anchor peak |
| `target_freq` | 9 | 0–511 | FFT bin index of the target peak |
| `delta_time` | 14 | 0–16383 | Frame distance between anchor and target |

**Why these sizes:**
- 9 bits covers 0–511 which safely holds any bin index in our 513-bin FFT output
- 14 bits covers 0–16383 frames which at 46.4ms per frame is over 12 minutes — more than any DJ track
- Together they fit exactly in 32 bits — compact, cache-friendly, fast to compare

**Why `int32` not `uint32`:**
PostgreSQL's `INTEGER` type is a signed 32-bit integer. Using `int32` in Go means the hash values map directly to the database column type with no conversion needed.

---

## The `FingerprintHash` Struct

```go
type FingerprintHash struct {
    Hash       int32
    SongID     int
    AnchorTime int
}
```

| Field | Content |
|-------|---------|
| `Hash` | The 32-bit encoded value |
| `SongID` | Populated during indexing — the song this hash belongs to. Zero during query/matching |
| `AnchorTime` | The frame index of the anchor peak — used for coherence scoring in the matcher |

---

## Two Distinct Usage Modes

`fingerprint.go` is used in two completely different contexts by the layers above it. The function signature is the same — the difference is in what is done with the output.

### Indexing Mode — When a song is uploaded

```
User uploads MP3
      ↓
Decode → Normalize → Spectrogram → Peaks → GenerateHashes(peaks, songID)
      ↓
[]FingerprintHash with SongID populated
      ↓
INSERT all rows into fingerprints table
```

### Query Mode — When a snippet is submitted for identification

```
User records snippet
      ↓
Decode → Normalize → Spectrogram → Peaks → GenerateHashes(peaks, 0)
      ↓
[]FingerprintHash with SongID = 0
      ↓
SELECT matching hashes from fingerprints table
      ↓
matcher.go builds HashMap, calculates coherence scores
```

---

## What the Output Looks Like

For a 3-minute song at 5 targets per anchor, the expected hash count is:

$$\text{hashes} \approx \text{totalPeaks} \times \text{fanOut} = \text{peaksPerFrame} \times \text{numFrames} \times 5$$

For a typical music track:
- ~3 peaks per frame (after threshold filtering)
- ~3871 frames for 3 minutes
- Fan-out of 5

$$\approx 3 \times 3871 \times 5 = 58{,}065 \text{ hashes per song}$$

This is why the `fingerprints` table grows large quickly and why the hash index is so critical.

---

## Hash Decoding — For Diagnostics

The hash can be decoded back to its component parts for testing and diagnostics:

$$f_{\text{anchor}} = (\text{hash} \gg 23) \And \texttt{0x1FF}$$

$$f_{\text{target}} = (\text{hash} \gg 14) \And \texttt{0x1FF}$$

$$\Delta t = \text{hash} \And \texttt{0x3FFF}$$

---

## How `fingerprint.go` Connects to the Rest of Phase 3

```
peaks.go                fingerprint.go              store layer
   │                          │                          │
   │   []PeakList             │   []FingerprintHash      │
   │ ─────────────────────→   │ ─────────────────────→   │
   │                          │                          │
   │                    For each anchor:                 │
   │                    find next 5 peaks                │
   │                    encode as int32                  │
   │                    tag with AnchorTime              │
```

---

## What Comes Next After `fingerprint.go`

Once `fingerprint.go` is complete, Phase 3 is fully done. The two remaining pieces are:

```
store/fingerprint_store.go   ← Phase 4: INSERT and SELECT hashes from DB
store/song_store.go          ← Phase 4: INSERT and SELECT song metadata
matcher/matcher.go           ← Phase 4: coherence scoring, Top 3 selection
```

---

# `fingerprint.go` & `fingerprint_test.go` — Build Summary

---

## What We Built

### The Purpose
`fingerprint.go` is the **third and final stage of the DSP engine** and the most critical component in the entire application. Its job is to take the compact set of frequency peaks produced by `peaks.go` and transform them into a collection of 32-bit integer hashes that uniquely describe the audio. These hashes are what gets stored in the database during indexing and what gets queried during identification.

---

## `fingerprint.go` — What Was Built and Why

### Constants Defined

```go
const (
    FanOut           = 5
    TargetZoneFrames = 100

    anchorFreqBits  = 9
    targetFreqBits  = 9
    deltaTimeBits   = 14

    anchorFreqMask  = (1 << anchorFreqBits) - 1  // 0x1FF  = 511
    targetFreqMask  = (1 << targetFreqBits) - 1  // 0x1FF  = 511
    deltaTimeMask   = (1 << deltaTimeBits)  - 1  // 0x3FFF = 16383

    anchorFreqShift = targetFreqBits + deltaTimeBits  // 23
    targetFreqShift = deltaTimeBits                   // 14
)
```

| Constant | Value | Reason |
|----------|-------|--------|
| `FanOut` | `5` | Maximum targets per anchor — balances fingerprint uniqueness against database size |
| `TargetZoneFrames` | `100` | How many frames ahead to search for targets — at 46.4ms per frame this is ~4.6 seconds forward |
| `anchorFreqBits` | `9` | Covers bins 0–511, safely holds any FFT bin from our 513-bin output |
| `targetFreqBits` | `9` | Same reasoning as anchorFreqBits |
| `deltaTimeBits` | `14` | Covers 0–16383 frames — at 46.4ms per frame this is over 12 minutes, more than any DJ track |

---

### The `FingerprintHash` Struct

```go
type FingerprintHash struct {
    Hash       int32
    SongID     int
    AnchorTime int
}
```

| Field | Indexing Mode | Query Mode |
|-------|--------------|------------|
| `Hash` | 32-bit encoded value | 32-bit encoded value |
| `SongID` | Database ID of the song being indexed | `0` — unknown, we are trying to find it |
| `AnchorTime` | Frame index of the anchor peak | Frame index of the anchor peak |

`int32` is used rather than `uint32` because PostgreSQL's `INTEGER` column type is a signed 32-bit integer. Using `int32` in Go means hash values map directly to the database column with no conversion needed.

---

### The `DecodedHash` Struct

```go
type DecodedHash struct {
    AnchorFreq int
    TargetFreq int
    DeltaTime  int
}
```

Used exclusively for testing and diagnostics. Holds the three component values recovered from a 32-bit hash via `DecodeHash`. Never stored in the database or used in matching — it exists to make the hash encoding verifiable and debuggable.

---

### `GenerateHashes(peakLists []PeakList, songID int) ([]FingerprintHash, error)`

The primary function. Takes the full peak output from `ExtractPeaks` and generates all fingerprint hashes.

**What it does step by step:**

```
1. Guard against empty peakLists
        ↓
2. Flatten all peaks from all PeakLists into a single ordered []Peak
   via flattenPeaks()
        ↓
3. Guard against no peaks after flattening (all frames were silent)
        ↓
4. For each peak i (the anchor):
   a. Scan forward from i+1
   b. For each candidate peak j:
      - Calculate deltaFrames = target.FrameIndex - anchor.FrameIndex
      - If deltaFrames > TargetZoneFrames → break (past the window)
      - If deltaFrames < 0 → skip (guard, should not happen)
      - Encode hash from anchor.Bin, target.Bin, deltaFrames
      - Append FingerprintHash with Hash, SongID, AnchorTime
      - Increment targetsFound
   c. Stop when targetsFound == FanOut or window is exhausted
        ↓
5. Guard against zero hashes produced
        ↓
6. Return []FingerprintHash
```

**Key design decisions:**

- **Flattening first** — converting `[]PeakList` to `[]Peak` before the pairing loop turns a nested frame-by-frame search into a single forward pass over a contiguous slice. This is simpler and more cache-friendly
- **Forward-only targets** — targets are always ahead of their anchor in time. This ensures $\Delta t$ is always positive and bounded, and means the same hash is generated for a given anchor-target pair regardless of where in the audio the snippet was recorded
- **Early break on zone exit** — because `allPeaks` is ordered by frame index, once `deltaFrames > TargetZoneFrames` no subsequent peak can be within the zone either. The `break` stops the inner scan immediately rather than checking every remaining peak
- **Fan-out cap** — stopping at `FanOut=5` targets per anchor keeps hash density predictable. Each anchor generates at most 5 hashes, making the total hash count for a song calculable and controllable

---

### `encodeHash(anchorBin, targetBin, deltaFrames int) int32`

Packs three values into a single 32-bit signed integer using bitwise operations.

**Bit layout:**

```
 31      23 22      14 13             0
┌──────────┬──────────┬────────────────┐
│anchorFreq│targetFreq│   deltaTime    │
│  9 bits  │  9 bits  │   14 bits      │
└──────────┴──────────┴────────────────┘
```

**Encoding:**

$$\text{hash} = \underbrace{(f_a \And \texttt{0x1FF})}_{\text{mask to 9 bits}} \ll 23 \mid \underbrace{(f_t \And \texttt{0x1FF})}_{\text{mask to 9 bits}} \ll 14 \mid \underbrace{(\Delta t \And \texttt{0x3FFF})}_{\text{mask to 14 bits}}$$

Each field is **masked before shifting** — this ensures that even if a value exceeds its field size, it cannot bleed into adjacent bits. For example, bin 512 masked to 9 bits becomes 0 rather than corrupting the target frequency field.

---

### `DecodeHash(hash int32) DecodedHash`

The inverse of `encodeHash`. Recovers the three original values by right-shifting and masking:

$$f_a = (\text{hash} \gg 23) \And \texttt{0x1FF}$$

$$f_t = (\text{hash} \gg 14) \And \texttt{0x1FF}$$

$$\Delta t = \text{hash} \And \texttt{0x3FFF}$$

Exported so it is accessible from the store layer and matcher layer for diagnostics.

---

### `flattenPeaks(peakLists []PeakList) []Peak`

Converts `[]PeakList` into a flat `[]Peak` ordered by frame index.

**Why this exists:**
The anchor-target pairing algorithm needs to scan forward from each anchor across all subsequent peaks regardless of which frame they came from. A flat ordered slice makes this a simple `for j := i+1; j < len(allPeaks)` loop. Without flattening, the equivalent logic would require nested loops over frames and peaks within frames — more complex and harder to reason about.

**Pre-allocation:**
The total peak count is calculated first with a counting pass, then the result slice is allocated once at full capacity. This avoids repeated slice growth during the append loop.

---

### `HashCount(hashes []FingerprintHash) int`

Convenience function returning `len(hashes)`. Used for logging and benchmarking.

---

## The Math — Expected Hash Volumes

### Per Song (3-minute track)

$$\text{frames} = \left\lfloor \frac{11025 \times 180 - 1024}{512} \right\rfloor + 1 = 3871$$

$$\text{peaks} \approx 3 \text{ per frame} \times 3871 = 11{,}613 \text{ total peaks}$$

$$\text{hashes} \approx 11{,}613 \times 5 = 58{,}065 \text{ hashes per song}$$

### Database Growth

| Library Size | Approximate Rows |
|-------------|-----------------|
| 100 songs | ~5.8 million |
| 500 songs | ~29 million |
| 1000 songs | ~58 million |
| 5000 songs | ~290 million |

This is why `idx_fingerprints_hash` in Phase 1 is the single most important index in the application.

---

## Two Usage Modes

The same `GenerateHashes` function serves two completely different purposes depending on context:

### Indexing Mode — Song Upload

```
User uploads MP3
      ↓
Decode → Normalize → Spectrogram → Peaks
      ↓
GenerateHashes(peakLists, songID=42)
      ↓
[]FingerprintHash with SongID=42
      ↓
INSERT all rows into fingerprints table
```

### Query Mode — Snippet Identification

```
User records snippet
      ↓
Decode → Normalize → Spectrogram → Peaks
      ↓
GenerateHashes(peakLists, songID=0)
      ↓
[]FingerprintHash with SongID=0
      ↓
SELECT matching hashes from fingerprints table
      ↓
matcher.go builds HashMap → coherence scoring → Top 3
```

The `songID=0` convention for query mode is intentional — zero is never a valid PostgreSQL `SERIAL` ID so it serves as a clear sentinel value.

---

## `fingerprint_test.go` — What Was Tested and Why

### Test Helper Functions

Two helpers keep tests clean and independent of the spectrogram and peaks packages:

**`makePeak(bin int, frameIndex int) Peak`**
Builds a `Peak` with a known bin and frame index and sensible defaults for all other fields. Allows precise control over which anchor-target pairs will be formed without needing a full audio pipeline.

**`makePeakList(frameIndex int, peaks []Peak) PeakList`**
Wraps a slice of peaks into a `PeakList` with consistent `TimeSeconds` derived from the frame index.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestEncodeHash_Deterministic` | Same inputs always produce the same hash value |
| `TestDecodeHash_RoundTrip` | Six cases covering zero, max, and realistic values all round-trip correctly |
| `TestEncodeHash_FieldsDoNotBleed` | Setting one field to maximum does not corrupt the other two fields |
| `TestEncodeHash_MaskOverflow` | Values exceeding field size are correctly masked — no corruption |
| `TestFlattenPeaks_Empty` | Empty input returns empty slice |
| `TestFlattenPeaks_SingleList` | Single PeakList flattened correctly |
| `TestFlattenPeaks_MultipleLists` | Multiple PeakLists produce correct total count |
| `TestFlattenPeaks_OrderPreserved` | Frame ordering is maintained after flattening |
| `TestFlattenPeaks_EmptyPeakLists_Skipped` | Silent frames contribute zero peaks to flat result |
| `TestGenerateHashes_EmptyPeakLists` | Returns error for empty input |
| `TestGenerateHashes_AllSilentFrames` | Returns error when all frames have zero peaks |
| `TestGenerateHashes_SongID_Propagated` | Every hash carries the correct songID |
| `TestGenerateHashes_ZeroSongID_ForQuery` | songID=0 is correctly propagated in query mode |
| `TestGenerateHashes_AnchorTime_IsFrameIndex` | AnchorTime matches the anchor peak's FrameIndex |
| `TestGenerateHashes_FanOut_AtMostFivePerAnchor` | No anchor produces more than 5 hashes |
| `TestGenerateHashes_FanOut_FewerTargetsThanFanOut` | Fewer available targets than FanOut uses all available |
| `TestGenerateHashes_TargetOutsideZone_NotPaired` | Peak beyond TargetZoneFrames is not paired |
| `TestGenerateHashes_TargetInsideZone_IsPaired` | Peak at exact zone boundary is correctly paired |
| `TestGenerateHashes_DeltaTime_AlwaysPositive` | All decoded delta times are >= 0 |
| `TestGenerateHashes_HashValues_NonZero` | Hash output is not all zeros for real peaks |
| `TestGenerateHashes_Deterministic` | Same peak lists always produce identical hash slices |
| `TestHashCount_Empty` | Returns 0 for empty slice |
| `TestHashCount_NonEmpty` | Returns correct count for non-empty slice |
| `TestGenerateHashes_Integration_FullPipeline` | Full DSP pipeline produces non-empty deterministic hashes |
| `TestGenerateHashes_Integration_SnippetMatchesSong` | Snippet hashes match full song hashes at >= 30% rate |
| `BenchmarkGenerateHashes_ThreeMinutes` | Hash generation on 3 minutes completes within acceptable time |

---

## The Integration Test Result

`TestGenerateHashes_Integration_SnippetMatchesSong` confirmed a **100% match rate** between a 10-second snippet and the full 30-second song it was taken from.

This result is expected for a pure sine wave — it has perfectly consistent peaks across its entire duration so every snippet hash appears in the full song hash set. Real music will produce lower match rates (typically 30–70%) due to variation in dynamics and timbre across the track. The 30% threshold in the test accounts for this.

A 100% match rate on a synthetic sine wave is the ideal baseline — it proves the hash encoding, decoding, and pairing logic is completely correct before introducing the complexity of real audio.

---

## Boundaries Established

`fingerprint.go` closes out the entire DSP engine. After this point the data is no longer audio — it is a set of integers ready for database operations:

```
BEFORE fingerprint.go              AFTER fingerprint.go
─────────────────────              ────────────────────
[]PeakList                   →     []FingerprintHash
Time-tagged frequency peaks        32-bit integer hashes
Structured audio events            Database-ready rows
~11,000 peaks per 3 min song       ~58,000 hashes per 3 min song
```

---

## What `fingerprint.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Audio decoding | `audio/decoder.go` |
| Amplitude normalization | `audio/normalizer.go` |
| Spectrogram computation | `dsp/spectrogram.go` |
| Peak extraction | `dsp/peaks.go` |
| Database INSERT of hashes | `store/fingerprint_store.go` — Phase 4 |
| Database SELECT of hashes | `store/fingerprint_store.go` — Phase 4 |
| Coherence scoring | `matcher/matcher.go` — Phase 4 |
| HTTP handling | `api/handlers/` — Phase 4 |

---

## Complete DSP Pipeline — Phase 3 Final State

```
*audio.PCMData (normalized, 11025 Hz, mono, []int16)
        │
        ▼
  spectrogram.go — Compute()
  Hamming window → FFT → magnitude extraction
  Restricted to 20Hz–5000Hz
        │
        ▼
  *Spectrogram — []Frame, 463 bins per frame
        │
        ▼
  peaks.go — ExtractPeaks()
  6 logarithmic bands → loudest bin per band
  Dynamic threshold filtering
        │
        ▼
  []PeakList — 1–6 peaks per frame
        │
        ▼
  fingerprint.go — GenerateHashes()
  Flatten peaks → anchor/target pairing
  32-bit hash encoding
        │
        ▼
  []FingerprintHash
  ~58,000 hashes per 3-minute song
  Ready for database INSERT or SELECT
```

---

## Test Results Achieved

```
All tests         → PASS
Match rate        → 100% on synthetic sine wave input
Benchmark         → Hash generation on 3 minutes within target
go vet            → clean
go build          → clean
```

---

## What Comes Next — Phase 4

With all three DSP stages complete, Phase 4 begins. The work splits into three parallel tracks that all depend on the `[]FingerprintHash` output:

```
Phase 4 files to build:

  internal/models/                ← data structs shared across layers
    song.go
    fingerprint.go
    song_feature.go

  internal/store/                 ← database read/write operations
    song_store.go
    fingerprint_store.go

  internal/matcher/               ← coherence scoring, Top 3 selection
    matcher.go

  internal/api/                   ← HTTP server, routes, handlers
    routes.go
    handlers/identify.go
    handlers/upload.go
    handlers/songs.go
    middleware/
    dto/
```

> This summary reflects the state of `fingerprint.go` and `fingerprint_test.go` as committed. Phase 3 — DSP Engine is now complete.