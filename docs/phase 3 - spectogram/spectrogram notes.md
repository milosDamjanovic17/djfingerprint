# Phase 3 — `spectrogram.go`

---

## What `spectrogram.go` Does

```
Receives  → a *PCMData struct from the audio pipeline (normalizer output)
Does      → applies Hamming window to overlapping segments, runs FFT on each
Returns   → a Spectrogram (2D matrix of magnitudes, time frames × frequency bins)
```

## What `spectrogram.go` Does NOT Do

```
Does not decode or normalize audio    ← that is the audio package
Does not extract peaks                ← that is peaks.go
Does not generate hashes              ← that is fingerprint.go
Does not touch the database           ← that is the store layer
Does not know about songs or matching ← that is the matcher layer
```

---

## Concepts to Lock In Before Coding

### The Sliding Window

Rather than running FFT on the entire audio at once, we slice it into overlapping segments. Each segment produces one **time frame** in the spectrogram.

```
Samples:  [.................................................]
Window 1: [■■■■■■■■■■■■■■■■]
Window 2:         [■■■■■■■■■■■■■■■■]
Window 3:                 [■■■■■■■■■■■■■■■■]
              ↑ hop size ↑
```

| Parameter | Value | Reason |
|-----------|-------|--------|
| Window size | `1024` samples | Matches FFT size, balances time and frequency resolution |
| Hop size | `512` samples | 50% overlap, standard for continuity between frames |

Number of frames produced:

$$\text{frames} = \left\lfloor \frac{\text{totalSamples} - \text{windowSize}}{\text{hopSize}} \right\rfloor + 1$$

---

### The Hamming Window

Each segment is multiplied sample-by-sample by a Hamming window before FFT. This smooths the edges of the segment to zero, preventing **spectral leakage** — the artificial frequency smearing that happens when you abruptly cut a signal.

$$w(n) = 0.54 - 0.46 \times \cos\left(\frac{2\pi n}{N - 1}\right)$$

Where $n$ is the sample index and $N$ is the window size (1024).

The result: first and last samples of the window are near zero, the center is near 1.0.

---

### The FFT Output

FFT on a real-valued input of size $N$ produces $N$ complex numbers. For real audio we only need the first $\frac{N}{2} + 1$ bins — the rest are a mirror image.

$$\text{usable bins} = \frac{1024}{2} + 1 = 513$$

Each bin $k$ corresponds to a frequency:

$$f_k = \frac{k \times f_s}{N} = \frac{k \times 11025}{1024}$$

Bin magnitudes are calculated as:

$$\text{magnitude}(k) = \sqrt{\text{real}(k)^2 + \text{imag}(k)^2}$$

---

### Frequency Range Restriction

The full 513 bins cover 0 Hz to 5512 Hz. We restrict output to bins covering **20 Hz to 5000 Hz** — the range that contains musically discriminative information.

$$\text{binStart} = \left\lfloor \frac{20 \times 1024}{11025} \right\rfloor = 1$$

$$\text{binEnd} = \left\lfloor \frac{5000 \times 1024}{11025} \right\rfloor = 464$$

So the output matrix per frame contains bins 1 through 464 — **464 usable bins**.

---

# `spectrogram.go` & `spectrogram_test.go` — Build Summary

---

## What We Built

### The Purpose
`spectrogram.go` is the **first stage of the DSP engine** and the bridge between the audio pipeline and the fingerprinting system. Its sole job is to take clean, normalized PCM audio and convert it from the time domain into the frequency domain — producing a 2D matrix of frequency magnitudes over time that the peak extractor and fingerprinter can work with.

---

## `spectrogram.go` — What Was Built and Why

### Constants Defined

```go
const (
    WindowSize = 1024
    HopSize    = 512
    SampleRate = 11025
    FreqMin    = 20.0
    FreqMax    = 5000.0
    NumBins    = WindowSize/2 + 1
)
```

| Constant | Value | Reason |
|----------|-------|--------|
| `WindowSize` | `1024` | Number of samples per FFT window. Must be a power of 2 for FFT efficiency. Balances time resolution and frequency resolution |
| `HopSize` | `512` | Samples between successive windows. 50% overlap — standard for continuity between frames |
| `SampleRate` | `11025` | Matches the guaranteed output of the audio pipeline exactly |
| `FreqMin` | `20.0 Hz` | Lower bound of musically useful frequency range |
| `FreqMax` | `5000.0 Hz` | Upper bound — contains all discriminative fingerprint information |
| `NumBins` | `513` | Usable FFT bins for real-valued input of size $N$: $\frac{N}{2} + 1$ |

Two package-level variables are calculated once at startup and reused across all calls:

```go
var (
    BinStart = freqToBin(FreqMin)   // bin 1  (≈ 20 Hz)
    BinEnd   = freqToBin(FreqMax)   // bin 464 (≈ 5000 Hz)
)
```

---

### The `Frame` Struct

```go
type Frame struct {
    Magnitudes  []float64
    Index       int
    TimeSeconds float64
}
```

One `Frame` is produced per sliding window position. It contains:

| Field | Content |
|-------|---------|
| `Magnitudes` | FFT magnitude for each bin in `[BinStart, BinEnd]` — 463 values |
| `Index` | Sequential position of this frame in the spectrogram — 0, 1, 2... |
| `TimeSeconds` | Center time of this window in seconds, used downstream for timing calculations |

The `TimeSeconds` for frame $i$ is calculated as:

$$\text{timeSeconds} = \frac{i \times \text{HopSize} + \frac{\text{WindowSize}}{2}}{\text{SampleRate}}$$

---

### The `Spectrogram` Struct

```go
type Spectrogram struct {
    Frames       []Frame
    NumFrames    int
    BinsPerFrame int
}
```

The full 2D time-frequency representation of the audio. A slice of `Frame` values — one per sliding window position. `NumFrames` and `BinsPerFrame` are stored explicitly to avoid repeated `len()` calls downstream.

---

### `Compute(pcm *audio.PCMData) (*Spectrogram, error)`

The primary function. Takes normalized PCM and returns a full spectrogram.

**What it does step by step:**

```
1. Guard against nil input
        ↓
2. Guard against empty samples slice
        ↓
3. Guard against audio shorter than one full window (< 1024 samples)
        ↓
4. Guard against unexpected sample rate (must be 11025)
        ↓
5. Pre-compute Hamming coefficients once for all frames
        ↓
6. Calculate total frame count:
   frames = floor((totalSamples - WindowSize) / HopSize) + 1
        ↓
7. Initialise the gonum FFT plan once — reused across all frames
        ↓
8. Allocate a single reusable windowed[] float64 buffer
        ↓
9. For each frame i:
   a. start = i * HopSize
   b. Apply Hamming: windowed[j] = sample[start+j] * hammingCoeff[j]
   c. Run FFT → complex output slice
   d. For each bin in [BinStart, BinEnd]:
      magnitude = sqrt(real² + imag²)  [via cmplx.Abs]
   e. Calculate TimeSeconds for this frame
   f. Append Frame to result
        ↓
10. Return *Spectrogram
```

**Key design decisions:**

- **Hamming coefficients computed once** — the same 1024 coefficients apply to every frame. Computing them once and reusing saves significant CPU time on long audio files
- **FFT plan initialised once** — `fourier.NewFFT(WindowSize)` creates an optimised FFT plan. Creating a new plan per frame would be expensive and wasteful
- **Single reusable buffer for windowed samples** — `windowed := make([]float64, WindowSize)` is allocated once outside the loop and overwritten each iteration, avoiding 1024 allocations per frame
- **`cmplx.Abs` for magnitude** — uses Go's standard library complex number absolute value which correctly computes $\sqrt{\text{real}^2 + \text{imag}^2}$
- **Frequency restriction to [BinStart, BinEnd]** — only 463 of the 513 available bins are stored per frame, reducing memory usage and focusing the downstream peak extractor on the relevant frequency range

---

### `buildHammingWindow(size int) []float64`

Computes the Hamming window coefficients using the standard formula:

$$w(n) = 0.54 - 0.46 \times \cos\left(\frac{2\pi n}{N - 1}\right)$$

Where $n$ is the sample index and $N$ is the window size.

**What this produces:**

```
n=0    → w(0) = 0.54 - 0.46 = 0.08   (near zero — edge suppressed)
n=512  → w(512) ≈ 1.0                 (center — fully preserved)
n=1023 → w(1023) ≈ 0.08              (near zero — edge suppressed)
```

The smooth bell curve eliminates the hard boundary artifacts that cause **spectral leakage** — without it, a single frequency tone would bleed energy into dozens of neighbouring bins, making peak identification unreliable.

---

### `freqToBin(freqHz float64) int` and `BinToFreq(bin int) float64`

Two utility functions for converting between Hz and FFT bin indices:

$$\text{bin} = \left\lfloor \frac{f \times \text{WindowSize}}{\text{SampleRate}} \right\rfloor = \left\lfloor \frac{f \times 1024}{11025} \right\rfloor$$

$$f = \frac{\text{bin} \times \text{SampleRate}}{\text{WindowSize}} = \frac{\text{bin} \times 11025}{1024}$$

`freqToBin` is private — used internally to calculate `BinStart` and `BinEnd`.
`BinToFreq` is exported — used in tests and diagnostics to convert detected bin indices back to readable Hz values.

---

## The Math — Explained

### Frequency Resolution

Each FFT bin covers a frequency range of:

$$\Delta f = \frac{f_s}{N} = \frac{11025}{1024} \approx 10.77 \text{ Hz per bin}$$

This means two frequencies closer than 10.77 Hz apart cannot be distinguished from each other in a single frame.

### Time Resolution

Each frame covers a time span of:

$$\Delta t = \frac{\text{HopSize}}{f_s} = \frac{512}{11025} \approx 46.4 \text{ ms per frame}$$

Consecutive frames are 46.4ms apart in time.

### Total Usable Bins Per Frame

$$\text{binsPerFrame} = \text{BinEnd} - \text{BinStart} = 464 - 1 = 463 \text{ bins}$$

### Frame Count for a 3-Minute Song

$$\text{totalSamples} = 11025 \times 180 = 1{,}984{,}500$$

$$\text{frames} = \left\lfloor \frac{1{,}984{,}500 - 1024}{512} \right\rfloor + 1 = 3{,}871 \text{ frames}$$

---

## The Compiler Error We Encountered and Fixed

During initial testing, `makeSineWave` in `spectrogram_test.go` referenced `int16Max` which is defined as an **unexported constant** in the `audio` package (`normalizer.go`). The `dsp` package cannot access unexported identifiers from other packages.

**The error:**
```
undefined: int16Max
```

**Why it happened:**
`int16Max` is lowercase in `normalizer.go`, making it package-private to the `audio` package. The `dsp` package is a completely separate package and has no visibility into `audio` package internals.

**The fix — define it locally in the test file:**
```go
// spectrogram_test.go
const int16Max = 32767
```

**Why this approach over exporting from `audio`:**
- The `dsp` package should not depend on internal constants from the `audio` package
- `32767` is a fundamental type constant, not business logic — safe to define in both places
- No changes needed to `normalizer.go` or `normalizer_test.go`
- Zero risk of regression in existing tests

---

## `spectrogram_test.go` — What Was Tested and Why

### Test Helper Functions

Three helpers keep the tests clean and self-contained:

**`makePCM(samples []int16) *audio.PCMData`**
Builds a `*PCMData` directly from a sample slice without needing FFmpeg. Used for all unit tests — fast, no I/O, no external dependencies. Always sets `SampleRate=11025` and `Channels=1` to match pipeline output.

**`makeSineWave(freqHz float64, numSamples int) []int16`**
Generates a mathematically pure sine wave at a known frequency. Used to verify that FFT peaks appear at the correct bins. Amplitude is scaled to 90% of `int16Max` to simulate real audio.

$$\text{sample}[i] = \sin\left(\frac{2\pi \times f \times i}{f_s}\right) \times 32767 \times 0.9$$

**`dominantBin(f Frame) int`**
Returns the index of the bin with the highest magnitude in a given frame. Used in the sine wave peak detection test to identify which frequency dominated the FFT output.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestCompute_NilInput` | Returns error for nil input, does not panic |
| `TestCompute_EmptySamples` | Returns error for empty samples slice |
| `TestCompute_TooFewSamples` | Returns error when samples < WindowSize (cannot form one frame) |
| `TestCompute_WrongSampleRate` | Returns error when sample rate is not 11025 |
| `TestCompute_FrameCount_ExactWindow` | Exactly WindowSize samples produces exactly 1 frame |
| `TestCompute_FrameCount_Formula` | Frame count matches $\lfloor(\text{samples} - W) / H\rfloor + 1$ |
| `TestCompute_NumFrames_MatchesSliceLength` | `NumFrames` field equals `len(Frames)` |
| `TestCompute_BinsPerFrame` | `BinsPerFrame` equals `BinEnd - BinStart` |
| `TestCompute_EachFrame_HasCorrectMagnitudeLength` | Every frame has exactly `BinEnd - BinStart` magnitudes |
| `TestCompute_FrameIndexSequential` | Frame indices are 0, 1, 2... with no gaps |
| `TestCompute_FrameTimeSeconds_Increasing` | Each frame's `TimeSeconds` is strictly greater than the previous |
| `TestCompute_FirstFrame_TimeSeconds` | First frame center time matches $\frac{\text{WindowSize}/2}{\text{SampleRate}}$ |
| `TestHammingWindow_Size` | Hamming window has exactly `WindowSize` coefficients |
| `TestHammingWindow_EdgeValuesNearZero` | First and last coefficients are below 0.09 |
| `TestHammingWindow_CenterValueNearOne` | Center coefficient is within 0.01 of 1.0 |
| `TestHammingWindow_Symmetric` | Coefficient at index $i$ equals coefficient at index $N-1-i$ |
| `TestHammingWindow_AllValuesInRange` | Every coefficient is within $[0.0, 1.0]$ |
| `TestFreqToBin_20Hz` | `freqToBin(20)` returns `BinStart` |
| `TestFreqToBin_5000Hz` | `freqToBin(5000)` returns `BinEnd` |
| `TestBinToFreq_RoundTrip` | Frequency → bin → frequency recovers within one bin width |
| `TestBinStart_LessThan_BinEnd` | `BinStart < BinEnd` — frequency range is valid |
| `TestBinEnd_WithinNumBins` | `BinEnd < NumBins` — no out-of-bounds reads possible |
| `TestCompute_SineWave_PeakAtCorrectBin` | 440 Hz sine wave produces dominant FFT peak within 2 bins of expected |
| `TestCompute_SilentAudio_MagnitudesNearZero` | All-zero input produces near-zero magnitudes throughout |
| `TestCompute_MagnitudesNonNegative` | No magnitude value is ever negative |
| `TestCompute_Deterministic` | Same input always produces byte-identical output |
| `BenchmarkCompute_ThreeMinutes` | 3 minutes of audio processed within acceptable time |

---

## Boundaries Established

`spectrogram.go` closes out the time-domain view of the audio. After this point everything works in the frequency domain:

```
BEFORE spectrogram.go              AFTER spectrogram.go
─────────────────────              ────────────────────
[]int16 PCM samples          →     []Frame (2D matrix)
Time domain waveform               Frequency domain magnitudes
Variable frequency content         Restricted to 20Hz–5000Hz
Single dimension                   Time × Frequency grid
```

---

## What `spectrogram.go` Deliberately Does Not Do

| Concern | Handled By |
|---------|-----------|
| Audio decoding | `audio/decoder.go` |
| Amplitude normalization | `audio/normalizer.go` |
| Peak frequency extraction | `dsp/peaks.go` — next file |
| Hash generation | `dsp/fingerprint.go` |
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
  *Spectrogram
  []Frame — each with 463 frequency magnitudes and a timestamp
        │
        ▼
  peaks.go  ← NEXT
  6-band logarithmic peak extraction
  Dynamic threshold filtering
```

---

## Test Results Achieved

```
All tests  → PASS
Benchmark  → 3 minutes of audio processed well within 10 second target
go vet     → clean
go build   → clean
```

---

> This summary reflects the state of `spectrogram.go` and `spectrogram_test.go` as committed. Move forward to `peaks.go` to read about how peaks are extracted.