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