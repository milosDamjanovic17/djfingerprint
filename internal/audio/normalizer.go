package audio

import (
	"fmt"
	"math"
)

const (
	// int16max == max value of signed 16-bit integer
	int16max = 32767

	// normalizationCeiling is the target peak amplitude after normalization
	// expressed as a fraction of int16Max.
	// 0.95 leaves 5% headroom to avoid hard clipping at the ceiling.
	// This means the loudest sample after normalization will be
	// at most 0.95 * 32767 = 31128
	normalizationCeiling = 0.95

	// silenceThreshold is the minimum peak amplitude we consider non-silent.
	// Any audio whose loudest sample falls below this value is considered
	// silence and is returned unchanged rather than being amplified into noise.
	// Value of 10 out of 32767 is effectively inaudible.
	silenceThreshold = 10
	
)

type NormalizeResult struct {
	// PCM is the normalized audio data
	// SampleRate, Channels and Duration are identical to the input
	PCM *PCMData

	// WasSilent is true when the input audio was below the silence threshold and normalization was skipped
	WasSilent bool

	// OriginalPeak is the loudest absolute sample value found in the input
	OriginalPeak int16

	// ScaleFactor = multiplier that was applied to every sample
	// Value > 1 means the audio was amplified, < 1 means it was attenuated, and = 1 means it was unchanged
	ScaleFactor float64
}

// Normalize scales the amplitude of a PCMData so that its loudest sample
// reaches the normalization ceiling (0.95 * int16Max).
//
// The operation is non-destructive: it returns a new PCMData with a new
// sample slice. The original PCMData is never modified.
//
// Silent audio (peak below silenceThreshold) is returned unchanged with
// WasSilent set to true. This prevents amplifying silence into noise.
//
// The SampleRate, Channels, and Duration fields of the input are preserved
// exactly in the output.
func Normalize(input *PCMData) (*NormalizeResult, error) {

	if input == nil {
		return nil, fmt.Errorf("normalize: input PCMData is nil")
	}

	if len(input.Samples) == 0 {
		return nil, fmt.Errorf("normalize: input PCMData has nos samples")
	}

	// --step 1: find the peak(maximum absolute amplitude)--
	peak := findPeak(input.Samples)

	// ------ Step 2: HANDLE SILENCE ------
	if peak < silenceThreshold {
		// Return a copy of the input unchanged so te caller always gets a fresh allocation and never accidentally shares sample slices
		copiedSamples := make([]int16, len(input.Samples))
		copy(copiedSamples, input.Samples)

		return &NormalizeResult{
			PCM: &PCMData{
				Samples: 	copiedSamples,
				SampleRate: input.SampleRate,
				Channels: 	input.Channels,
				Duration: 	input.Duration,
			},
			WasSilent: 		true,
			OriginalPeak: 	peak,
			ScaleFactor: 	1.0,
		}, nil
	}

	//  ---- Step 3: CALCULATE SCALE FACTOR ----
	// targetPeak is what we want the loudes sample to become
	targetPeak := float64(int16max) * normalizationCeiling

	// factor how much we need to multiply every sample by to get the peak to reach the target
	scaleFactor := targetPeak / float64(peak)

	// ---- Step 4: APPLY SCALE FACTOR TO ALL SAMPLES ----
	normalizedSamples := make([]int16, len(input.Samples))

	for i, s := range input.Samples {
		// Multiply the sample by the scale factor
		scaled := float64(s) * scaleFactor

		// Clamp to int16 range to guard against floating point rounding
		// pushing a value just over the int16 boundary.
		// This should not happen with a correctly calculated scale factor
		// but we guard it explicitly because int16 overflow wraps silently
		// and would corrupt the audio data.
		scaled = math.Max(float64(-int16max-1), math.Min(float64(int16max), scaled))

		normalizedSamples[i] = int16(scaled)
	}

	return &NormalizeResult{
		PCM: &PCMData{
			Samples: normalizedSamples,
			SampleRate: input.SampleRate,
			Channels: input.Channels,
			Duration: input.Duration,
		},
		WasSilent: false,
		OriginalPeak: peak,
		ScaleFactor: scaleFactor,
	}, nil
}


// findPeak returns the maximum absolute sample value across all samples.
// It handles the int16 minimum value (-32768) safely by converting to
// int32 before taking the absolute value — math.Abs(-32768) would overflow
// int16 since +32768 does not fit in a signed 16-bit integer.
func findPeak(samples []int16) int16 {
	var maxAbs int32

	for _, s := range samples {
		// Convert to int32 before abs to handle math.MinInt16 (-32768)
		// safely. The abs of -32768 is 32768 which does not fit in int16.
		abs := int32(s)
		if abs < 0 {
			abs = -abs
		}
		if abs > maxAbs {
			maxAbs = abs
		}
	}

	// Clamp back to int16 range
	// maxAbs can be at most 32768 (from -32768 input)
	// but we cap at 32767 to stay within int16 safely
	if maxAbs > int16max {
		return int16max
	}

	return int16(maxAbs)
}

// DecodeAndNormalize is a convenience function that decodes an audio file and immediately normalizes it in one call.
// This is the function most of the pipeline will actually use — raw file path in, normalized PCMData out.
func DecodeAndNormalize(filePath string, ffmpegPath string) (*NormalizeResult, error) {
	pcm, err := DecodeFile(filePath, ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	result, err := Normalize(pcm)
	if err != nil {
		return nil, fmt.Errorf("normalize failed: %w", err)
	}

	return result, nil
}

// DecodeBytesAndNormalize is the in-memory equivalent of DecodeAndNormalize.
// Used by the HTTP identify endpoint where audio arrives as a byte slice.
func DecodeBytesAndNormalize(data []byte, ffmpegPath string) (*NormalizeResult, error) {
	pcm, err := DecodeBytes(data, ffmpegPath)
	if err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	result, err := Normalize(pcm)
	if err != nil {
		return nil, fmt.Errorf("normalize failed: %w", err)
	}

	return result, nil
}