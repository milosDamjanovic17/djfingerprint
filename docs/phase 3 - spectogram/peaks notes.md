# Phase 3 — `peaks.go`

---

## What `peaks.go` Does

```
Receives  → a *Spectrogram from spectrogram.go
Does      → divides each frame into 6 logarithmic frequency bands
            finds the loudest bin in each band
            calculates a dynamic threshold from the 6 loudest values
            discards peaks that fall below the threshold
Returns   → a []PeakList — one PeakList per frame, each containing
            the peaks that survived the threshold filter
```

## What `peaks.go` Does NOT Do

```
Does not decode or normalize audio        ← audio package
Does not compute FFT or build spectrogram ← spectrogram.go
Does not generate hashes                  ← fingerprint.go
Does not pair anchors with targets        ← fingerprint.go
Does not touch the database               ← store layer
Does not know about songs or matching     ← matcher layer
```

---

## The 6 Logarithmic Bands — Locked In

These are defined in Hz and converted to bin indices at runtime using `freqToBin`. Never hardcoded as raw bin numbers.

| Band | Name | Hz Range | Purpose |
|------|------|----------|---------|
| 0 | Very Low | 20–300 Hz | Bass, kick drum |
| 1 | Low | 300–600 Hz | Bass lines |
| 2 | Low-Mid | 600–1200 Hz | Instruments |
| 3 | Mid | 1200–2400 Hz | Vocals, melodies |
| 4 | High-Mid | 2400–4000 Hz | Presence |
| 5 | High | 4000–5000 Hz | Brilliance |

---

## How the Dynamic Threshold Works

For each frame:

1. Find the loudest bin in each of the 6 bands → 6 candidate peaks
2. Calculate the average magnitude of those 6 candidates

$$\text{threshold} = \frac{1}{6}\sum_{b=0}^{5} \text{magnitude}(\text{loudestBin}_b)$$

3. Discard any candidate whose magnitude falls below the threshold
4. Keep only the survivors — these are the genuine peaks for this frame

This ensures a peak is not kept simply because it was the loudest in its band. It must also be genuinely strong relative to the other bands in the same frame.

---

# `peaks.go` & `peaks_test.go` — Build Summary

---

## What We Built

### The Purpose
`peaks.go` is the **second stage of the DSP engine**. Its sole job is to take the full 2D spectrogram matrix produced by `spectrogram.go` and reduce it to a compact, meaningful set of frequency peaks — the most distinctive audio events in each time frame. These peaks are the raw material that `fingerprint.go` will use to generate hashes.

---

## `peaks.go` — What Was Built and Why

### The `Band` Struct

```go
type Band struct {
    Name     string
    MinHz    float64
    MaxHz    float64
    binStart int
    binEnd   int
}
```

Each band is defined by its Hz boundaries (`MinHz`, `MaxHz`) which are human-readable and format-independent. The corresponding FFT bin indices (`binStart`, `binEnd`) are calculated once at package startup via `freqToBin` and stored privately. This design means:

- Band definitions are readable and maintainable in Hz
- Bin calculations are never duplicated in the hot path
- If FFT parameters change, only `freqToBin` needs updating — the band definitions stay the same

---

### The 6 Frequency Bands

```go
var FrequencyBands = []Band{
    {Name: "VeryLow", MinHz: 20,   MaxHz: 300},
    {Name: "Low",     MinHz: 300,  MaxHz: 600},
    {Name: "LowMid",  MinHz: 600,  MaxHz: 1200},
    {Name: "Mid",     MinHz: 1200, MaxHz: 2400},
    {Name: "HighMid", MinHz: 2400, MaxHz: 4000},
    {Name: "High",    MinHz: 4000, MaxHz: 5000},
}
```

| Band | Hz Range | Purpose |
|------|----------|---------|
| VeryLow | 20–300 Hz | Bass, kick drum |
| Low | 300–600 Hz | Bass lines |
| LowMid | 600–1200 Hz | Instruments |
| Mid | 1200–2400 Hz | Vocals, melodies |
| HighMid | 2400–4000 Hz | Presence |
| High | 4000–5000 Hz | Brilliance |

The bands are **contiguous with no gaps** — each band's `MinHz` equals the previous band's `MaxHz`. Together they cover the full 20 Hz–5000 Hz range established in `spectrogram.go`.

The bin boundaries are populated in the `init()` function which runs once automatically at package startup:

```go
func init() {
    for i := range FrequencyBands {
        FrequencyBands[i].binStart = freqToBin(FrequencyBands[i].MinHz)
        FrequencyBands[i].binEnd   = freqToBin(FrequencyBands[i].MaxHz)
    }
}
```

---

### The `Peak` Struct

```go
type Peak struct {
    Bin         int
    Magnitude   float64
    FrequencyHz float64
    FrameIndex  int
    TimeSeconds float64
    BandIndex   int
}
```

A `Peak` is the fundamental data point that the fingerprinting engine works with. Every field is populated at extraction time:

| Field | Content |
|-------|---------|
| `Bin` | Absolute FFT bin index — used for hash encoding |
| `Magnitude` | FFT magnitude at this bin — used for threshold filtering |
| `FrequencyHz` | Human-readable frequency — used for diagnostics |
| `FrameIndex` | Which frame this peak came from |
| `TimeSeconds` | Centre time of the frame — used for anchor timing in fingerprint.go |
| `BandIndex` | Which of the 6 bands (0–5) — used for diagnostics and testing |

---

### The `PeakList` Struct

```go
type PeakList struct {
    Peaks       []Peak
    FrameIndex  int
    TimeSeconds float64
}
```

One `PeakList` is produced per spectrogram frame. It holds all peaks from that frame that survived the dynamic threshold filter. Most frames will have between 1 and 6 peaks. Silent frames produce an empty `Peaks` slice.

---

### `ExtractPeaks(spec *Spectrogram) ([]PeakList, error)`

The primary function. Processes every frame in the spectrogram and returns one `PeakList` per frame.

**What it does step by step:**

```
1. Guard against nil spectrogram
        ↓
2. Guard against empty frames slice
        ↓
3. For each frame:
   a. Call extractFramePeaks(frame)
   b. Append resulting PeakList to result
        ↓
4. Return []PeakList — one entry per frame, same length as spec.Frames
```

---

### `extractFramePeaks(frame Frame) PeakList`

The core algorithm. Processes a single frame and returns its surviving peaks.

**What it does step by step:**

```
1. For each of the 6 bands:
   call loudestBinInBand() → zero or one candidate peak
        ↓
2. If no candidates found at all (fully silent frame):
   return empty PeakList immediately
        ↓
3. Calculate dynamic threshold:
   threshold = average magnitude of all candidates
        ↓
4. Filter: keep only candidates where magnitude >= threshold
        ↓
5. Return PeakList with surviving peaks
```

---

### `loudestBinInBand(frame Frame, band Band, bandIdx int) (Peak, bool)`

Scans the frame's magnitude slice within a band's bin range and returns the single loudest bin.

**Key implementation detail — index translation:**

The frame's `Magnitudes` slice is indexed relative to `BinStart`, not to the full FFT output. Absolute bin 1 (BinStart) maps to `Magnitudes[0]`. The translation is:

```go
magIdx := bin - BinStart
```

**The zero magnitude guard — critical fix:**

During testing we discovered that initialising `maxMag := -1.0` caused zero-magnitude bins to be selected as valid candidates in silent frames (`0.0 > -1.0` is true). This caused the threshold to collapse to `0.0`, which then passed all zero-magnitude candidates, producing 6 ghost peaks from a silent frame.

The fix — an explicit guard that skips any bin with magnitude `<= 0.0`:

```go
if mag <= 0.0 {
    continue
}
```

This ensures:
- Silent frames produce zero candidates → empty PeakList
- The threshold can never collapse to 0.0 from zero-magnitude inputs
- Ghost peaks from silence are impossible

---

### `dynamicThreshold(candidates []Peak) float64`

Calculates the average magnitude of the candidate peaks:

$$\text{threshold} = \frac{1}{N}\sum_{i=0}^{N-1} \text{candidate}_i.\text{Magnitude}$$

**Why average rather than a fixed value:**
A fixed threshold would be arbitrary and would fail across different recording volumes, different genres, and different mix styles. The average adapts to the actual energy in each frame — loud frames get a higher threshold, quiet frames get a lower one. This is the same adaptive approach used in the original Shazam algorithm.

**Threshold filtering behaviour:**

| Scenario | Result |
|----------|--------|
| All bands equal magnitude | Threshold = that value, all 6 survive (`>=`) |
| One band dominant | Threshold ≈ dominant/6, only dominant survives |
| Two bands strong, four weak | Depends on ratio — typically 1–2 survive |
| All magnitudes zero | No candidates found, empty PeakList returned |

---

### `PeakCount(peakLists []PeakList) int`

A utility function that returns the total number of peaks across all PeakLists. Used for logging, benchmarking, and verifying that a fingerprinting run produced a reasonable output volume.

---

## The Bug We Found and Fixed

### What Happened

`TestExtractPeaks_SilentFrame_EmptyPeakList` failed with:

```
expected 0 peaks for silent frame, got 6
```

### Root Cause — Three Compounding Issues

**Issue 1 — `maxMag` initialised to `-1.0`:**
```go
maxMag := -1.0
// 0.0 > -1.0 is TRUE
// zero-magnitude bins selected as candidates
```

**Issue 2 — Threshold collapses to zero:**
```go
threshold = (0.0 + 0.0 + 0.0 + 0.0 + 0.0 + 0.0) / 6 = 0.0
```

**Issue 3 — Filter passes everything at zero threshold:**
```go
0.0 >= 0.0  // TRUE — all 6 ghost peaks survive
```

### The Fix

Added an explicit zero-magnitude guard in `loudestBinInBand`:

```go
if mag <= 0.0 {
    continue
}
```

This breaks the chain at Issue 1 — zero-magnitude bins are never selected as candidates, so the threshold never collapses, and ghost peaks are never produced.

### Additional Test Added

`TestExtractPeaks_ZeroMagnitude_NotACandidate` was added to explicitly document and permanently protect against this behaviour:

```go
func TestExtractPeaks_ZeroMagnitude_NotACandidate(t *testing.T) {
    // Bins with magnitude 0.0 must never become peak candidates
    frame := makeFrame(0, 0.0)
    // All magnitudes left at default zero
    spec := makeSpectrogram([]Frame{frame})
    peakLists, _ := ExtractPeaks(spec)
    // Must produce 0 peaks, not 6 ghost peaks
}
```

---

## `peaks_test.go` — What Was Tested and Why

### Test Helper Functions

Three helpers keep the tests clean and free of FFmpeg or real audio dependencies:

**`makeFrame(frameIndex int, timeSeconds float64) Frame`**
Builds a `Frame` with a correctly-sized zero-magnitude slice. Used as the base for all peak extraction unit tests.

**`setMagnitude(f *Frame, absoluteBin int, magnitude float64)`**
Sets a specific bin's magnitude using an absolute bin index, handling the `BinStart` offset internally. Allows tests to place peaks at precise Hz values via `freqToBin(hz)` without knowing bin arithmetic.

**`makeSpectrogram(frames []Frame) *Spectrogram`**
Wraps a slice of Frames into a `*Spectrogram` with correct metadata fields populated.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestExtractPeaks_NilSpectrogram` | Returns error for nil input, does not panic |
| `TestExtractPeaks_NoFrames` | Returns error for spectrogram with empty frames slice |
| `TestExtractPeaks_OutputLength_MatchesFrameCount` | One PeakList produced per frame |
| `TestExtractPeaks_FrameIndex_Preserved` | Each PeakList carries the correct FrameIndex |
| `TestExtractPeaks_TimeSeconds_Preserved` | Each PeakList carries the correct TimeSeconds |
| `TestExtractPeaks_SilentFrame_EmptyPeakList` | All-zero frame produces empty PeakList |
| `TestExtractPeaks_ZeroMagnitude_NotACandidate` | Zero-magnitude bins are never selected as candidates |
| `TestDynamicThreshold_Empty` | Empty candidate slice returns threshold of 0.0 |
| `TestDynamicThreshold_SingleCandidate` | Single candidate returns its own magnitude as threshold |
| `TestDynamicThreshold_Average` | Three candidates return their correct mathematical average |
| `TestDynamicThreshold_AllEqual` | Equal magnitudes return that value as threshold |
| `TestExtractPeaks_WeakPeaks_Discarded` | One dominant peak causes 5 weak peaks to be discarded |
| `TestExtractPeaks_AllEqualMagnitude_AllSurvive` | Equal magnitudes cause all 6 to survive |
| `TestExtractPeaks_MaxSixPeaksPerFrame` | No frame ever produces more than 6 peaks |
| `TestExtractPeaks_Peak_BandIndex_Correct` | Surviving peak carries the correct BandIndex |
| `TestExtractPeaks_Peak_FrequencyHz_InBandRange` | Every peak's FrequencyHz falls within its band's Hz range |
| `TestExtractPeaks_Peak_TimeSeconds_MatchesFrame` | Every peak's TimeSeconds matches its parent frame |
| `TestExtractPeaks_Peak_MagnitudeNonNegative` | No peak ever carries a negative magnitude |
| `TestFrequencyBands_Count` | Exactly 6 bands defined |
| `TestFrequencyBands_BinStart_LessThan_BinEnd` | Every band has valid bin range |
| `TestFrequencyBands_NoGaps` | Each band's MinHz equals the previous band's MaxHz |
| `TestFrequencyBands_CoversFullRange` | First band starts at FreqMin, last band ends at FreqMax |
| `TestPeakCount_Empty` | Returns 0 for empty PeakList slice |
| `TestPeakCount_MultipleLists` | Returns correct total across multiple PeakLists |
| `TestExtractPeaks_Integration_WithRealSpectrogram` | Full pipeline from sine wave through Compute to ExtractPeaks |
| `BenchmarkExtractPeaks_ThreeMinutes` | Peak extraction on 3 minutes completes within acceptable time |

---

## Boundaries Established

`peaks.go` closes out the spectrogram analysis stage. After this point the data is no longer a 2D matrix — it is a compact set of time-tagged frequency events:

```
BEFORE peaks.go                    AFTER peaks.go
───────────────                    ──────────────
*Spectrogram                 →     []PeakList
~3871 frames × 463 bins            ~3871 PeakLists
Full magnitude matrix              1–6 peaks per frame
~1.8M float64 values               ~4000–23000 Peak structs
All frequency bins                 Only the most significant bins
```

---

## What `peaks.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Audio decoding | `audio/decoder.go` |
| Amplitude normalization | `audio/normalizer.go` |
| Spectrogram computation | `dsp/spectrogram.go` |
| Hash generation | `dsp/fingerprint.go` — next file |
| Anchor/target pairing | `dsp/fingerprint.go` — next file |
| Database storage | `store/song_store.go` |
| HTTP handling | `api/handlers/` |

---

## Complete DSP Pipeline — Current State

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
  Dynamic threshold filtering → discard weak peaks
        │
        ▼
  []PeakList — 1–6 peaks per frame, each with bin, magnitude, time
        │
        ▼
  fingerprint.go  ← NEXT
  Anchor/target pairing → 32-bit hash generation
```

---

## Test Results Achieved

```
All tests  → PASS (including fix for ghost peaks on silent frames)
Benchmark  → ExtractPeaks on 3 minutes completes well within target
go vet     → clean
go build   → clean
```

---

> This summary reflects the state of `peaks.go` and `peaks_test.go` as committed. Move forward to `fingerprint.go` to check the fingerprinting process.