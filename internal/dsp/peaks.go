package dsp

import (
	"fmt"
)

// Band defines a single logarithmic frequency band by its Hz boundaries.
// The bin indices are calculated at package initialisation from the Hz
// values using freqToBin so they are always consistent with the FFT
// parameters defined in spectrogram.go.
type Band struct {
	// Name is a human-readable label used in diagnostics and logging
	Name string

	// MinHz and MaxHz define the frequency range in Hz
	MinHz float64
	MaxHz float64

	// binStart and binEnd are the corresponding FFT bin indices
	// calculated from MinHz and MaxHz via freqToBin
	// These are relative to the full FFT output, not to BinStart
	binStart int
	binEnd   int
}

// FrequencyBands defines the 6 logarithmic bands used for peak extraction.
// Bands are ordered from lowest to highest frequency.
// They are initialised as a package-level variable so the bin calculations
// run once at startup and are reused across all calls.
var FrequencyBands = []Band{
	{Name: "VeryLow", MinHz: 20, MaxHz: 300},
	{Name: "Low", MinHz: 300, MaxHz: 600},
	{Name: "LowMid", MinHz: 600, MaxHz: 1200},
	{Name: "Mid", MinHz: 1200, MaxHz: 2400},
	{Name: "HighMid", MinHz: 2400, MaxHz: 4000},
	{Name: "High", MinHz: 4000, MaxHz: 5000},
}

func init() {
	// Calculate the bin boundaries for each band at package startup.
	// This runs once and avoids repeated freqToBin calls inside
	// the hot path of ExtractPeaks.
	for i := range FrequencyBands {
		FrequencyBands[i].binStart = freqToBin(FrequencyBands[i].MinHz)
		FrequencyBands[i].binEnd = freqToBin(FrequencyBands[i].MaxHz)
	}
}

// Peak represents a single frequency peak extracted from one frame
// of the spectrogram. It is the fundamental data point that the
// fingerprinting engine works with.
type Peak struct {
	// Bin is the absolute FFT bin index of this peak
	// (relative to the full FFT output, not to BinStart)
	Bin int

	// Magnitude is the FFT magnitude at this bin
	Magnitude float64

	// FrequencyHz is the centre frequency of this bin in Hz
	// Stored for readability and diagnostics
	FrequencyHz float64

	// FrameIndex is the spectrogram frame this peak belongs to
	FrameIndex int

	// TimeSeconds is the centre time of the frame this peak belongs to
	TimeSeconds float64

	// BandIndex is which of the 6 bands (0–5) this peak came from
	BandIndex int
}

// PeakList holds all peaks that survived the dynamic threshold filter
// for a single spectrogram frame. Most frames will have between 1 and 6
// peaks — fewer when the audio is sparse, more when it is dense.
type PeakList struct {
	// Peaks contains the surviving peaks for this frame
	Peaks []Peak

	// FrameIndex is the index of the frame these peaks belong to
	FrameIndex int

	// TimeSeconds is the centre time of this frame
	TimeSeconds float64
}

// ExtractPeaks processes every frame in the spectrogram and returns
// a PeakList for each frame containing the peaks that passed the
// dynamic threshold filter.
//
// For each frame:
//  1. Scan each of the 6 frequency bands for the loudest bin
//  2. Calculate the average magnitude of the 6 loudest candidates
//  3. Discard candidates below the average (dynamic threshold)
//  4. Return the survivors as a PeakList
//
// Frames where all candidates fall below the threshold (e.g. silence)
// produce an empty PeakList rather than an error.
func ExtractPeaks(spec *Spectrogram) ([]PeakList, error) {
	if spec == nil {
		return nil, fmt.Errorf("extractpeaks: spectrogram is nil")
	}

	if len(spec.Frames) == 0 {
		return nil, fmt.Errorf("extractpeaks: spectrogram has no frames")
	}

	result := make([]PeakList, 0, len(spec.Frames))

	for _, frame := range spec.Frames {
		peakList := extractFramePeaks(frame)
		result = append(result, peakList)
	}

	return result, nil
}

// extractFramePeaks processes a single Frame and returns its PeakList.
// This is the core of the peak extraction algorithm.
func extractFramePeaks(frame Frame) PeakList {
	// Step 1: For each band find the loudest bin and its magnitude
	// candidates holds at most one peak per band
	candidates := make([]Peak, 0, len(FrequencyBands))

	for bandIdx, band := range FrequencyBands {
		peak, found := loudestBinInBand(frame, band, bandIdx)
		if found {
			candidates = append(candidates, peak)
		}
	}

	// If no candidates were found at all (e.g. completely silent frame)
	// return an empty PeakList
	if len(candidates) == 0 {
		return PeakList{
			Peaks:       []Peak{},
			FrameIndex:  frame.Index,
			TimeSeconds: frame.TimeSeconds,
		}
	}

	// Step 2: Calculate the dynamic threshold
	// threshold = average magnitude of all candidates
	threshold := dynamicThreshold(candidates)

	// Step 3: Keep only candidates above the threshold
	survivors := make([]Peak, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate.Magnitude >= threshold {
			survivors = append(survivors, candidate)
		}
	}

	return PeakList{
		Peaks:       survivors,
		FrameIndex:  frame.Index,
		TimeSeconds: frame.TimeSeconds,
	}
}

// loudestBinInBand scans the frame magnitudes within a band's bin range
// and returns the Peak at the bin with the highest magnitude.
//
// The band's bin indices are absolute (relative to the full FFT output).
// The frame's Magnitudes slice is relative to BinStart, so we subtract
// BinStart when indexing into it.
//
// Returns (Peak, true) when a valid bin is found in the band.
// Returns (Peak, false) when the band has no overlap with the frame's
// bin range — this should not happen in normal operation but is guarded
// explicitly to avoid index out of bounds panics.
func loudestBinInBand(frame Frame, band Band, bandIdx int) (Peak, bool) {
	maxMag := -1.0
	maxBin := -1

	for bin := band.binStart; bin < band.binEnd; bin++ {
		magIdx := bin - BinStart

		if magIdx < 0 || magIdx >= len(frame.Magnitudes) {
			continue
		}

		mag := frame.Magnitudes[magIdx]

		// A bin with zero magnitude carries no audio information.
		// Allowing zero-magnitude bins through would cause the dynamic
		// threshold to calculate as 0.0, which would then pass every
		// candidate including the zero ones — producing ghost peaks
		// from silent frames.
		if mag <= 0.0 {
			continue
		}

		if mag > maxMag {
			maxMag = mag
			maxBin = bin
		}
	}

	if maxBin < 0 {
		return Peak{}, false
	}

	return Peak{
		Bin:         maxBin,
		Magnitude:   maxMag,
		FrequencyHz: BinToFreq(maxBin),
		FrameIndex:  frame.Index,
		TimeSeconds: frame.TimeSeconds,
		BandIndex:   bandIdx,
	}, true
}

// dynamicThreshold calculates the average magnitude of a set of
// candidate peaks. Peaks below this average are considered too weak
// to be reliable fingerprint anchors and are discarded.
//
// Using the average rather than a fixed value makes the threshold
// adaptive — it responds to the actual energy in each frame rather
// than applying a one-size-fits-all cutoff.
func dynamicThreshold(candidates []Peak) float64 {
	if len(candidates) == 0 {
		return 0.0
	}

	total := 0.0
	for _, p := range candidates {
		total += p.Magnitude
	}

	return total / float64(len(candidates))
}

// PeakCount returns the total number of peaks across all PeakLists.
// Useful for logging and benchmarking.
func PeakCount(peakLists []PeakList) int {
	total := 0
	for _, pl := range peakLists {
		total += len(pl.Peaks)
	}
	return total
}