# DJFingerprint Application
## Phase 2 – BE Configuration and Audio Pipeline Logic

### decoder.go – 
### What it does?
- Receives  → a file path (string) pointing to any audio file
- Does      → invokes FFmpeg as a subprocess, pipes raw PCM out
- Returns   → a PCMData struct containing samples, rate, channels, duration
### What it doesn’t do

- Does not normalize amplitude — that is `normalizer.go`
- Does not run FFT — that is `dsp/spectrogram.go`
- Does not validate business logic — that is the handler layer
- Does not touch the database — that is the store layer

# `decoder.go` & `decoder_test.go` — Build Summary

---

## What was Built – decoder.go

### The Purpose
`decoder.go` is the **entry point of the entire audio pipeline**. Its sole job is to take any audio file — regardless of format — and produce a clean, consistent, predictable output that every downstream component can rely on. Nothing after this point needs to know or care what format the original audio was in.

---

## `decoder.go` — What Was Built and Why

### The `PCMData` Struct
```go
type PCMData struct {
    Samples    []int16
    SampleRate int
    Channels   int
    Duration   time.Duration
}
```

This is the **contract between the audio pipeline and everything downstream**. Every package that needs audio data — the DSP engine, the fingerprinter, the matcher — receives a `*PCMData`. It always contains:

| Field | Value | Why Fixed |
|-------|-------|-----------|
| `Samples` | `[]int16` | Signed 16-bit PCM, the raw audio waveform |
| `SampleRate` | Always `11025` | Covers 0–5512 Hz range, enough for fingerprinting, reduces computation |
| `Channels` | Always `1` (mono) | Stereo adds no value for fingerprinting |
| `Duration` | Calculated from samples | Derived, not trusted from file metadata |

Two helper methods were added to the struct:

- `SampleCount()` — returns `len(p.Samples)`, convenience wrapper
- `DurationSeconds()` — returns duration as `float64`, used in DSP calculations downstream

---

### `DecodeFile(filePath, ffmpegPath string) (*PCMData, error)`

The primary function. Takes a path to any audio file and returns decoded PCM.

**What it does step by step:**

```
1. Verify the input file exists on disk
        ↓
2. Verify FFmpeg is accessible (PATH or explicit path)
        ↓
3. Build and run the FFmpeg command with these flags:
   -ac 1          → force mono
   -ar 11025      → resample to 11025 Hz
   -f s16le       → signed 16-bit little-endian PCM output
   -acodec pcm_s16le → explicit codec match
   -vn            → strip any video stream (MP4, WebM files)
   -loglevel error → suppress FFmpeg noise, only surface real errors
   pipe:1         → write raw bytes to stdout
        ↓
4. Capture stdout (raw PCM bytes) and stderr (FFmpeg error messages) separately
        ↓
5. Wait for FFmpeg to exit, check exit code
   If non-zero → include FFmpeg's own stderr message in the error
        ↓
6. Guard against empty output (silent, corrupt, unreadable file)
        ↓
7. Guard against odd byte count (malformed PCM output)
        ↓
8. Convert []byte → []int16
   Every 2 bytes = 1 sample using binary.LittleEndian.Uint16, cast to int16
        ↓
9. Calculate duration from sample count and sample rate
        ↓
10. Return *PCMData
```

**Key design decisions:**
- FFmpeg's `stderr` is captured separately and included in error messages — this makes debugging format issues significantly easier
- Duration is **calculated from the actual sample count**, not trusted from file metadata — metadata can lie, sample count cannot
- The function fails fast at each step with a descriptive typed error rather than letting FFmpeg produce confusing output

---

### `DecodeBytes(data []byte, ffmpegPath string) (*PCMData, error)`

A companion function for the HTTP layer. When audio arrives as a browser upload (a `multipart/form-data` POST body), it exists in memory as `[]byte`, not on disk.

**What it does:**

```
1. Guard against empty input
        ↓
2. Write bytes to a temporary file (os.CreateTemp)
        ↓
3. defer os.Remove(tmpFile.Name()) — cleanup is guaranteed regardless of outcome
        ↓
4. Close the temp file before passing path to FFmpeg
        ↓
5. Call DecodeFile with the temp file path
        ↓
6. Return result — temp file is cleaned up automatically
```

**Why a temp file and not stdin pipe:**
FFmpeg needs a seekable input for most container formats (MP3, WAV, WebM). Piping via stdin works only for raw PCM or specific formats. Writing to a temp file is the safe universal approach.

---

## `decoder_test.go` — What Was Tested and Why

### Test File Generation Strategy

Rather than shipping binary audio files in the repository (which bloat Git history and are opaque), **all test audio is generated at test runtime using FFmpeg itself**:

```go
func generateSineWAV(t *testing.T, path string, freqHz int, durationSec float64)
func generateSilentWAV(t *testing.T, path string, durationSec float64)
```

Both use `t.TempDir()` which Go cleans up automatically after each test. No test fixtures pollute the repository.

A `ffmpegAvailable()` helper checks for FFmpeg before each test that needs it and calls `t.Skip()` if it is absent — this prevents CI failures on machines without FFmpeg installed.

---

### Tests Written

| Test | What It Verifies |
|------|-----------------|
| `TestDecodeFile_FileNotFound` | Returns error for nonexistent path, does not panic |
| `TestDecodeFile_EmptyPath` | Returns error for empty string path |
| `TestDecodeFile_FFmpegNotFound` | Returns error when FFmpeg binary does not exist |
| `TestDecodeFile_WAV_SampleRate` | Output sample rate is always `11025` regardless of input rate |
| `TestDecodeFile_WAV_Mono` | Output is always mono (`Channels=1`) regardless of input |
| `TestDecodeFile_WAV_SampleCount` | Sample count matches expected duration within ±5% tolerance |
| `TestDecodeFile_WAV_Duration` | Calculated duration matches expected within ±200ms tolerance |
| `TestDecodeFile_SilentAudio_NoError` | Silent audio decodes without error — normalizer handles zeros |
| `TestDecodeFile_CorruptFile_ReturnsError` | Corrupt/non-audio bytes return error, do not panic |
| `TestDecodeFile_SamplesNotEmpty` | Samples slice is non-empty and not all zeros for a sine wave |
| `TestDecodeBytes_EmptyInput` | Returns error for empty byte slice |
| `TestDecodeBytes_ValidWAVBytes` | WAV bytes decoded in memory match expected rate and channel output |
| `TestDecodeBytes_CorruptBytes` | Corrupt bytes return error, do not panic |
| `TestPCMData_SampleCount` | `SampleCount()` returns correct length |
| `TestPCMData_DurationSeconds` | `DurationSeconds()` returns exactly `1.0` for 11025 samples at 11025 Hz |
| `BenchmarkDecodeFile_30Seconds` | 30-second stereo WAV decodes within acceptable time |

---

### Tolerance Decisions

Two tests use tolerances rather than exact values — this is intentional:

**Sample count — ±5% tolerance:**
$$\text{expected} = 11025 \times \text{duration\_seconds}$$

Resampling from 44100 Hz to 11025 Hz involves integer rounding at boundaries. A 5% window accommodates this without producing false failures.

**Duration — ±200ms tolerance:**
Duration is derived from sample count which itself has resampling rounding. 200ms is generous enough to be stable but tight enough to catch genuine decode failures.

---

## Boundaries Established

This file establishes a clear boundary that the rest of the project respects:

```
BEFORE decoder.go          AFTER decoder.go
─────────────────          ────────────────
MP3, WAV, WebM,      →     []int16 samples
OGG, FLAC, AIFF            11025 Hz
Any sample rate            Mono
Stereo or mono             Known duration
Unknown duration
```

Nothing downstream of `decoder.go` ever needs to think about audio formats, sample rates, channel counts, or byte order again.

---

## Test Results Achieved

```
15 tests    → all PASS
1 benchmark → completes within 2.537s for a 30-second file
go vet      → clean
go build    → clean
```

---

## What `decoder.go` Deliberately Does Not Do

These are handled by other files — important to keep the boundaries clean:

| Concern | Handled By |
|---------|-----------|
| Amplitude normalization | `normalizer.go` — next file |
| FFT / frequency analysis | `dsp/spectrogram.go` — Phase 3 |
| Peak extraction | `dsp/peaks.go` — Phase 3 |
| Hash generation | `dsp/fingerprint.go` — Phase 3 |
| Database storage | `store/song_store.go` — Phase 4 |
| HTTP request handling | `api/handlers/upload.go` — Phase 4 |

---

> The  summary reflects the state of `decoder.go` and `decoder_test.go` as committed.

Quick explanation of the difference between testing.T and testing.B
-  testing.T → “Is it correct?”
-  testing.B → “How fast and how much memory?”


