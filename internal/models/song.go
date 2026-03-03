package models

import "time"

// Song represents a single indexed track in the DJ library.
// It mirrors the songs table in PostgreSQL exactly.
//
// Nullable database columns are represented as pointer types:
//   - nil pointer → database NULL (field not known)
//   - non-nil pointer → field has a value
//
// This distinction matters for unlabeled tracks from old CDs
// where title, artist and album may genuinely be unknown.
type Song struct {
	ID         int     `json:"id"`
	Title      *string `json:"title,omitempty"`
	Artist     *string `json:"artist,omitempty"`
	Album      *string `json:"album,omitempty"`
	DurationMs *int    `json:"duration_ms,omitempty"`
	Source     string  `json:"source"`
	SpotifyID  *string `json:"spotify_id,omitempty"`
	FileHash   string  `json:"file_hash"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SongTitle returns the song title as a plain string.
// Returns an empty string when Title is nil (unknown track).
func (s *Song) SongTitle() string {
	return stringVal(s.Title)
}

// SongArtist returns the artist as a plain string.
// Returns an empty string when Artist is nil.
func (s *Song) SongArtist() string {
	return stringVal(s.Artist)
}

// SongAlbum returns the album as a plain string.
// Returns an empty string when Album is nil.
func (s *Song) SongAlbum() string {
	return stringVal(s.Album)
}

// HasSpotifyID returns true when the song has a known Spotify track ID.
// Used to determine whether Spotify audio features can be fetched.
func (s *Song) HasSpotifyID() bool {
	return s.SpotifyID != nil && *s.SpotifyID != ""
}

// IsManualUpload returns true when the song was uploaded directly
// by the user rather than linked via Spotify metadata.
func (s *Song) IsManualUpload() bool {
	return s.Source == "manual"
}

// DisplayName returns the most descriptive name available for the song.
// Falls back gracefully when metadata is missing:
//
//	"Artist - Title"  when both are known
//	"Title"           when only title is known
//	"Artist"          when only artist is known
//	"Unknown Track"   when neither is known
func (s *Song) DisplayName() string {
	title := stringVal(s.Title)
	artist := stringVal(s.Artist)

	switch {
	case artist != "" && title != "":
		return artist + " - " + title
	case title != "":
		return title
	case artist != "":
		return artist
	default:
		return "Unknown Track"
	}
}

// stringVal safely dereferences a *string pointer.
// Returns empty string when the pointer is nil.
// Defined in this file as it is used only by Song methods.
// SongFeature has its own equivalent for float64 and int.
func stringVal(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}

// NewSong is a convenience constructor that creates a Song with
// source set to "manual" and the file hash populated.
// Title, Artist and Album are left nil — they are filled in by
// the upload handler after metadata extraction.
func NewSong(fileHash string) *Song {
	return &Song{
		Source:   "manual",
		FileHash: fileHash,
	}
}

// ValidSources lists the allowed values for the Source field.
// Mirrors the CHECK constraint defined in Migration 001.
var ValidSources = []string{"manual", "spotify", "unknown"}

// IsValidSource returns true when the source value is one of the
// three permitted values defined in the database constraint.
func IsValidSource(source string) bool {
	for _, s := range ValidSources {
		if s == source {
			return true
		}
	}
	return false
}