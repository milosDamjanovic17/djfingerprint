# Phase 2 — `normalizer.go`

Before writing a single line, let us establish the boundaries clearly — same approach as `decoder.go`.

---

## What `normalizer.go` Does

```
Receives  → a *PCMData struct from decoder.go
Does      → analyzes peak amplitude, scales all samples proportionally
Returns   → a new *PCMData with normalized samples, same length, same rate
```

## What `normalizer.go` Does NOT Do

```
Does not decode audio files          ← that is decoder.go
Does not modify SampleRate           ← always stays 11025
Does not modify Channels             ← always stays 1
Does not modify Duration             ← sample count unchanged
Does not run FFT or any DSP          ← that is dsp/spectrogram.go
Does not touch the database          ← that is the store layer
```

---

## Why Normalization Is Necessary

Without normalization, two recordings of the **exact same song** at different volumes will produce different FFT magnitudes in the spectrogram. This directly affects which peaks get selected and which get discarded by the dynamic threshold in Phase 3.

The risk without it:

```
Recording A (loud) → strong peaks selected → fingerprint set A
Recording B (quiet) → weak peaks selected → fingerprint set B
A and B are the same song but produce different fingerprints → NO MATCH
```

With normalization both recordings are scaled to the same amplitude range before the spectrogram is computed — the peaks will be consistent regardless of recording volume.

---

## The Math

We use **peak normalization**. Find the loudest sample, calculate a scale factor that would bring it to the target ceiling, apply that factor to every sample.

$$\text{scale} = \frac{\text{targetPeak}}{\text{maxAmplitude}}$$

$$\text{sample}_{\text{normalized}} = \text{sample}_{\text{original}} \times \text{scale}$$

Where:
- $\text{targetPeak}$ is `0.95 × int16\_max` = `0.95 × 32767` = `31129`
- We use `0.95` rather than `1.0` to leave a small headroom and avoid clipping at the ceiling

---


# `normalizer.go` & `normalizer_test.go` — Build Summary

---

## What We Built

### The Purpose
`normalizer.go` is the **second and final stage of the audio pipeline**. Its sole job is to take the raw decoded PCM data from `decoder.go` and scale its amplitude to a consistent, predictable level before it enters the DSP engine. Nothing after this point needs to account for volume differences between recordings.

---

## `normalizer.go` — What Was Built and Why

### The `NormalizeResult` Struct

```go
type NormalizeResult struct {
    PCM          *PCMData
    WasSilent    bool
    OriginalPeak int16
    ScaleFactor  float64
}
```

Rather than returning just a `*PCMData`, the normalizer returns a richer result struct that carries diagnostic information alongside the audio data. This design decision serves two purposes:

| Field | Purpose |
|-------|---------|
| `PCM` | The normalized audio, ready for the DSP engine |
| `WasSilent` | Signals to the caller that normalization was skipped — the pipeline can decide whether to abort or continue |
| `OriginalPeak` | Useful for logging and debugging — shows how loud the audio was before normalization |
| `ScaleFactor` | Useful for testing and diagnostics — shows exactly how much the amplitude was adjusted |

---

### Constants Defined

```go
const (
    int16Max             = 32767
    normalizationCeiling = 0.95
    silenceThreshold     = 10
)
```

| Constant | Value | Reason |
|----------|-------|--------|
| `int16Max` | `32767` | Maximum value of a signed 16-bit integer |
| `normalizationCeiling` | `0.95` | Target peak as a fraction of int16Max — leaves 5% headroom to prevent hard clipping |
| `silenceThreshold` | `10` | Minimum peak to be considered non-silent — a peak of 10 out of 32767 is effectively inaudible |

The ceiling of `0.95` means the loudest sample after normalization will reach:

$$\text{targetPeak} = 0.95 \times 32767 = 31128.65 \approx 31129$$

---

### `Normalize(input *PCMData) (*NormalizeResult, error)`

The primary function. Takes a `*PCMData` and returns amplitude-normalized audio.

**What it does step by step:**

```
1. Guard against nil input
        ↓
2. Guard against empty samples slice
        ↓
3. Find the peak amplitude across all samples via findPeak()
        ↓
4. Check peak against silenceThreshold
   If below threshold → copy samples unchanged, return WasSilent=true
        ↓
5. Calculate scale factor:
   targetPeak = int16Max × normalizationCeiling
   scaleFactor = targetPeak / peak
        ↓
6. Apply scale factor to every sample:
   scaledSample = originalSample × scaleFactor
   Clamp result to [-32768, 32767] to guard against float rounding overflow
        ↓
7. Return NormalizeResult with new PCMData, original peak, and scale factor
```

**Key design decisions:**

- **Non-destructive** — a new `[]int16` slice is always allocated. The input `PCMData` is never modified. This prevents subtle bugs where the same PCMData is used in multiple places
- **Silence handled explicitly** — rather than dividing by zero or producing garbage output, audio below `silenceThreshold` is returned unchanged with a `WasSilent` flag so the caller can make an informed decision
- **Clamping after scaling** — floating point multiplication can push a value fractionally over the int16 boundary. The explicit clamp using `math.Max` / `math.Min` prevents silent integer overflow which would corrupt audio data
- **Copy-on-silence** — even when the audio is silent and unchanged, a fresh copy of the sample slice is returned. The caller never accidentally shares a slice with the input

---

### `findPeak(samples []int16) int16`

A private helper that finds the maximum absolute amplitude across all samples.

**The critical edge case — `math.MinInt16`:**

```
int16 range: -32768 to +32767
abs(-32768) = +32768
+32768 does NOT fit in int16
```

If handled naively with an `int16` intermediate, this overflows and wraps to `-32768` — a completely wrong result that would cause the scale factor calculation to produce garbage.

**The fix:**
```go
abs := int32(s)   // promote to int32 BEFORE taking absolute value
if abs < 0 {
    abs = -abs
}
```

By promoting to `int32` first, `abs(-32768)` becomes `32768` safely. The result is then clamped back to `int16Max` (32767) before returning.

---

### Convenience Functions

Two pipeline convenience functions were added to reduce boilerplate in the callers:

**`DecodeAndNormalize(filePath, ffmpegPath string) (*NormalizeResult, error)`**
```
file path → DecodeFile → Normalize → NormalizeResult
```
Used by the upload handler when processing a file already saved to disk.

**`DecodeBytesAndNormalize(data []byte, ffmpegPath string) (*NormalizeResult, error)`**
```
[]byte → DecodeBytes → Normalize → NormalizeResult
```
Used by the identify handler when audio arrives as a browser upload in memory.

These functions are what the rest of the application will actually call. The individual `DecodeFile` and `Normalize` functions remain public for testing and for cases where finer control is needed.

---

## The Math — Explained

### Scale Factor Calculation

$$\text{scaleFactor} = \frac{\text{int16Max} \times \text{normalizationCeiling}}{\text{peak}}$$

$$\text{scaleFactor} = \frac{32767 \times 0.95}{\text{peak}} = \frac{31128.65}{\text{peak}}$$

### Applied to Each Sample

$$\text{sample}_{\text{out}} = \text{clamp}\left(\text{sample}_{\text{in}} \times \text{scaleFactor},\ -32768,\ 32767\right)$$

### Practical Example

Given a quiet recording with a peak amplitude of `1000`:

$$\text{scaleFactor} = \frac{31128.65}{1000} = 31.13$$

A sample of `500` becomes:

$$500 \times 31.13 = 15565$$

A sample of `-1000` (the peak) becomes:

$$-1000 \times 31.13 = -31130 \approx -31129 \text{ (clamped to ceiling)}$$

---

## The Compiler Error We Encountered and Fixed

During testing, three test cases produced this error:

```
cannot convert float64(int16Max) * normalizationCeiling
(constant 31128.6 of type float64) to type int16
compilerInvalidConversion
```

**Why it happened:**

The Go compiler evaluates `float64(int16Max) * normalizationCeiling` at compile time as the constant `31128.65`. It then refuses to convert this non-integer float directly to `int16` because silent truncation of a fractional part is a known source of bugs. Go forces you to be explicit.

**The fix — `math.Round()` before the cast:**

```go
// Before (compiler error)
expectedPeak := int16(float64(int16Max) * normalizationCeiling)

// After (correct)
expectedPeak := int16(math.Round(float64(int16Max) * normalizationCeiling))
```

`math.Round(31128.65)` produces `31129.0` — a whole number — which the `int16()` cast then accepts cleanly.

**Affected locations:** `TestNormalize_PeakReachesCeiling`, `TestNormalize_AlreadyAtCeiling_ScaleNearOne`, `TestDecodeAndNormalize_ValidFile`

---

## `normalizer_test.go` — What Was Tested and Why

### Test Helper Functions

Two helpers were introduced to keep tests clean and self-contained:

**`makePCM(samples []int16) *PCMData`**
Builds a `PCMData` directly from a sample slice without needing FFmpeg. Used for all pure unit tests of normalization logic — fast, no I/O, no external dependencies.

**`peakOf(samples []int16) int16`**
Mirrors the internal `findPeak` logic. Used in tests to verify the peak of the output without calling the private function directly.

**`readFile(path string) ([]byte, error)`**
Thin wrapper around `os.ReadFile`. Keeps test helper functions consistent in style.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestNormalize_NilInput` | Returns error for nil input, does not panic |
| `TestNormalize_EmptySamples` | Returns error for empty samples slice |
| `TestNormalize_SilentAudio_WasSilentTrue` | All-zero samples sets `WasSilent=true` |
| `TestNormalize_SilentAudio_ScaleFactorOne` | Silent audio returns `ScaleFactor=1.0` unchanged |
| `TestNormalize_SilentAudio_SamplesUnchanged` | Silent samples are all still zero after normalization |
| `TestNormalize_BelowThreshold_WasSilentTrue` | Peak below `silenceThreshold=10` treated as silent |
| `TestNormalize_PeakReachesCeiling` | Output peak lands within ±2 counts of target ceiling |
| `TestNormalize_AlreadyAtCeiling_ScaleNearOne` | Audio already at ceiling produces scale factor within 1% of 1.0 |
| `TestNormalize_QuietAudio_AmplifiedCorrectly` | Very quiet audio scale factor matches expected calculation within 1% |
| `TestNormalize_OriginalPCMUnchanged` | Input sample slice is never modified by normalization |
| `TestNormalize_ReturnsSeparateSlice` | Modifying output samples does not affect input samples |
| `TestNormalize_SampleRatePreserved` | `SampleRate` remains `11025` after normalization |
| `TestNormalize_ChannelsPreserved` | `Channels` remains `1` after normalization |
| `TestNormalize_SampleCountUnchanged` | Output sample count equals input sample count exactly |
| `TestNormalize_OriginalPeakReported` | `OriginalPeak` in result matches the actual input peak |
| `TestNormalize_WasSilentFalse_ForAudibleAudio` | Audible audio correctly sets `WasSilent=false` |
| `TestFindPeak_AllPositive` | Finds correct peak from positive-only samples |
| `TestFindPeak_AllNegative` | Finds correct absolute peak from negative-only samples |
| `TestFindPeak_Mixed` | Finds correct peak from mixed positive and negative samples |
| `TestFindPeak_Int16Min_NoOverflow` | `MinInt16` (-32768) handled without int16 overflow or panic |
| `TestFindPeak_SingleSample` | Single sample slice returns correct peak |
| `TestFindPeak_AllZeros` | All-zero slice returns peak of 0 |
| `TestDecodeAndNormalize_ValidFile` | Full pipeline produces normalized output from a real WAV file |
| `TestDecodeAndNormalize_FileNotFound` | Returns error for nonexistent file path |
| `TestDecodeBytesAndNormalize_EmptyBytes` | Returns error for empty byte slice |
| `TestDecodeBytesAndNormalize_ValidBytes` | Full in-memory pipeline produces correct output from WAV bytes |
| `BenchmarkNormalize_LargeSlice` | Normalizing 30 seconds of audio completes within acceptable time |

---

## Boundaries Established

`normalizer.go` closes out the audio pipeline. After this point the data is clean, consistent, and ready for DSP:

```
BEFORE normalizer.go          AFTER normalizer.go
────────────────────          ───────────────────
[]int16 samples         →     []int16 samples
Variable amplitude            Peak at 95% of int16Max
Possible silence              Silence flagged explicitly
Raw decoded output            Pipeline-ready PCM
```

---

## What `normalizer.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Audio file decoding | `decoder.go` — previous file |
| Spectrogram generation | `dsp/spectrogram.go` — Phase 3 |
| Peak frequency extraction | `dsp/peaks.go` — Phase 3 |
| Hash generation | `dsp/fingerprint.go` — Phase 3 |
| Database storage | `store/song_store.go` — Phase 4 |
| HTTP request handling | `api/handlers/upload.go` — Phase 4 |

---

## Complete Audio Pipeline — Phase 2 Final State

With both files complete, the full audio pipeline is:

```
Any audio file or []byte
        │
        ▼
  decoder.go
  DecodeFile() / DecodeBytes()
  FFmpeg: any format → mono, 11025 Hz, int16 PCM
        │
        ▼
  normalizer.go
  Normalize()
  Scale peak amplitude to 95% of int16Max
        │
        ▼
  *PCMData
  []int16 samples, 11025 Hz, mono
  Consistent amplitude
  Ready for FFT
```

The convenience functions `DecodeAndNormalize()` and `DecodeBytesAndNormalize()` wrap both steps into a single call that the rest of the application will use.

---

## Test Results Achieved

```
26 tests    → all PASS
1 benchmark → normalizing 30s of audio completes well under 50ms
go vet      → clean
go build    → clean
```

---

## What Comes Next — Phase 3

With the audio pipeline complete and producing clean, normalized `*PCMData`, Phase 3 begins. The DSP engine will consume this output directly:

```
Phase 3 files to build:
  internal/dsp/spectrogram.go   ← FFT, Hamming window, sliding window
  internal/dsp/peaks.go         ← 6-band peak extraction, dynamic threshold
  internal/dsp/fingerprint.go   ← hash generation, anchor/target pairing
```

> This summary reflects the state of `normalizer.go` and `normalizer_test.go` as committed. Ready to move into Phase 3 — `spectrogram.go` whenever you are.