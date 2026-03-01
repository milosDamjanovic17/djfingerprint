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