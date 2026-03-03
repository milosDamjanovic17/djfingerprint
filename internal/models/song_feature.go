package models

import "time"

// SongFeature represents the Spotify audio features for a track.
// It mirrors the song_features table in PostgreSQL exactly.
//
// All audio feature columns are nullable in the database because:
//   - Features are only available for tracks with a Spotify ID
//   - A track uploaded manually from a local file has no features
//   - Features are fetched asynchronously after the track is indexed
//
// All feature fields use *float64 or *int so that:
//   - nil means "not yet fetched" or "not available"
//   - 0.0 means the feature was fetched and its value is genuinely zero
//
// This distinction cannot be made with plain float64 (0.0 is ambiguous).
type SongFeature struct {
	ID              int     `json:"id"`
	SongID          int     `json:"song_id"`
	SpotifyTrackID  *string `json:"spotify_track_id,omitempty"`

	// Spotify audio features — all nullable
	// Range 0.0–1.0 unless noted otherwise
	Energy          *float64 `json:"energy,omitempty"`
	Danceability    *float64 `json:"danceability,omitempty"`

	// Valence is Spotify's term for musical positiveness
	// 0.0 = sad / negative, 1.0 = happy / positive
	Valence         *float64 `json:"valence,omitempty"`

	Acousticness    *float64 `json:"acousticness,omitempty"`
	Instrumentalness *float64 `json:"instrumentalness,omitempty"`
	Liveness        *float64 `json:"liveness,omitempty"`
	Speechiness     *float64 `json:"speechiness,omitempty"`

	// Loudness is in decibels, typically between -60.0 and 0.0
	Loudness        *float64 `json:"loudness,omitempty"`

	// Tempo is in beats per minute (BPM)
	Tempo           *float64 `json:"tempo,omitempty"`

	// MusicalKey uses Spotify's Pitch Class notation:
	// 0=C, 1=C#, 2=D, 3=D#, 4=E, 5=F,
	// 6=F#, 7=G, 8=G#, 9=A, 10=A#, 11=B
	// -1 means no key detected (Spotify returns -1)
	MusicalKey      *int `json:"musical_key,omitempty"`

	CreatedAt time.Time `json:"created_at"`
}

// Float64Val safely dereferences a *float64 pointer.
// Returns 0.0 when the pointer is nil.
// Use HasFeature() first when you need to distinguish nil from 0.0.
func Float64Val(f *float64) float64 {
	if f == nil {
		return 0.0
	}
	return *f
}

// IntVal safely dereferences a *int pointer.
// Returns 0 when the pointer is nil.
func IntVal(i *int) int {
	if i == nil {
		return 0
	}
	return *i
}

// HasFeature returns true when a *float64 feature field is non-nil.
// Used to distinguish "feature not available" (nil) from
// "feature value is zero" (non-nil pointer to 0.0).
func HasFeature(f *float64) bool {
	return f != nil
}

// NewSongFeature creates a SongFeature with the song ID populated.
// All feature fields are left nil — they are populated when
// the Spotify API response is received (Phase 7).
func NewSongFeature(songID int) *SongFeature {
	return &SongFeature{
		SongID: songID,
	}
}

// KeyName returns the musical key as a human-readable string.
// Returns "Unknown" when MusicalKey is nil or -1.
func (sf *SongFeature) KeyName() string {
	if sf.MusicalKey == nil {
		return "Unknown"
	}

	keys := []string{
		"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B",
	}

	key := *sf.MusicalKey
	if key < 0 || key >= len(keys) {
		return "Unknown"
	}

	return keys[key]
}

// IsComplete returns true when all Spotify audio features are populated.
// A complete SongFeature has no nil feature fields.
// Used to determine whether a feature fetch is needed.
func (sf *SongFeature) IsComplete() bool {
	return sf.Energy != nil &&
		sf.Danceability != nil &&
		sf.Valence != nil &&
		sf.Acousticness != nil &&
		sf.Instrumentalness != nil &&
		sf.Liveness != nil &&
		sf.Speechiness != nil &&
		sf.Loudness != nil &&
		sf.Tempo != nil &&
		sf.MusicalKey != nil
}

// Float64Ptr is a convenience function that returns a pointer to
// a float64 value. Used when constructing SongFeature in tests
// and in the Spotify integration layer (Phase 7).
func Float64Ptr(f float64) *float64 {
	return &f
}

// IntPtr is a convenience function that returns a pointer to
// an int value. Used when constructing SongFeature in tests.
func IntPtr(i int) *int {
	return &i
}

// StringPtr is a convenience function that returns a pointer to
// a string value. Used when constructing Song and SongFeature in tests.
func StringPtr(s string) *string {
	return &s
}