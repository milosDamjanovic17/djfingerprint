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
