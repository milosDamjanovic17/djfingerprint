package dto

import (
	"fmt"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/matcher"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// Confidence level thresholds.
// These define how the raw coherence score is translated into a
// human-readable confidence label for the frontend.
const (
	ConfidenceHighThreshold   = 50
	ConfidenceMediumThreshold = 15
)

// Confidence level strings returned in MatchResponse.
const (
	ConfidenceHigh   = "high"
	ConfidenceMedium = "medium"
	ConfidenceLow    = "low"
)

// Error code constants.
// Machine-readable codes that allow the frontend to handle specific
// error conditions programmatically without parsing message strings.
const (
	ErrCodeDuplicateFile  = "duplicate_file"
	ErrCodeInvalidFile    = "invalid_file"
	ErrCodeFileTooLarge   = "file_too_large"
	ErrCodeNotFound       = "not_found"
	ErrCodeInternalError  = "internal_error"
	ErrCodeNoMatch        = "no_match"
	ErrCodeInvalidRequest = "invalid_request"
)

// SongResponse is the external representation of a song.
// It is a subset of models.Song — fields that are internal
// implementation details (FileHash) are deliberately excluded.
type SongResponse struct {
	ID        int     `json:"id"`
	Title     *string `json:"title,omitempty"`
	Artist    *string `json:"artist,omitempty"`
	Album     *string `json:"album,omitempty"`
	DurationMs *int   `json:"duration_ms,omitempty"`
	Source    string  `json:"source"`
	SpotifyID *string `json:"spotify_id,omitempty"`

	// CreatedAt is included so the frontend can display
	// when the track was added to the library
	CreatedAt time.Time `json:"created_at"`
}

// MatchItem represents a single candidate song in an identification
// response. Ranked 1–3 with the highest-scoring candidate at rank 1.
type MatchItem struct {
	// Rank is 1-indexed — rank 1 is the best match
	Rank int `json:"rank"`

	// Song is the candidate song's metadata
	Song SongResponse `json:"song"`

	// Score is the raw coherence score from the matcher.
	// Included so advanced clients can use it for their own logic.
	Score int `json:"score"`

	// OffsetSecs is approximately where in the full song
	// the submitted snippet came from
	OffsetSecs float64 `json:"offset_secs"`
}

// MatchResponse is the response body for POST /api/v1/identify.
// It carries the identification result including all candidate matches,
// a confidence label, and processing time.
type MatchResponse struct {
	// Found is true when at least one match exceeded MinScore
	Found bool `json:"found"`

	// Confidence is a human-readable confidence label derived from
	// the top match's coherence score.
	// One of: "high", "medium", "low", or "" when Found is false.
	Confidence string `json:"confidence"`

	// Matches contains the Top 3 candidate songs sorted by score.
	// Empty slice (not null) when Found is false.
	Matches []MatchItem `json:"matches"`

	// ProcessedIn is the total time taken to process the request
	// formatted as a human-readable string e.g. "1.23s"
	ProcessedIn string `json:"processed_in"`
}

// ErrorResponse is the response body for all error responses.
// Used for 4xx and 5xx status codes.
type ErrorResponse struct {
	// Error is a machine-readable code the frontend can switch on
	Error string `json:"error"`

	// Message is a human-readable description of the error
	Message string `json:"message"`
}

// ListResponse wraps a paginated list of songs with metadata.
type ListResponse struct {
	// Songs is the list of songs for the current page
	Songs []SongResponse `json:"songs"`

	// Total is the total number of songs in the library
	Total int `json:"total"`

	// Page is the current page number (1-indexed)
	Page int `json:"page"`

	// PageSize is the number of songs per page
	PageSize int `json:"page_size"`
}

// NewSongResponse converts a models.Song into a SongResponse.
// FileHash is deliberately not included — it is an internal
// implementation detail that the client should never see.
func NewSongResponse(song *models.Song) SongResponse {
	return SongResponse{
		ID:         song.ID,
		Title:      song.Title,
		Artist:     song.Artist,
		Album:      song.Album,
		DurationMs: song.DurationMs,
		Source:     song.Source,
		SpotifyID:  song.SpotifyID,
		CreatedAt:  song.CreatedAt,
	}
}

// NewMatchResponse builds a MatchResponse from matcher results and
// the song metadata map retrieved from the database.
//
// songs is a map of songID → *models.Song populated by the handler
// after the matcher returns its results.
//
// duration is the total processing time for the identification request.
func NewMatchResponse(
	results []matcher.MatchResult,
	songs map[int]*models.Song,
	duration time.Duration,
) MatchResponse {
	if len(results) == 0 {
		return MatchResponse{
			Found:       false,
			Confidence:  "",
			Matches:     []MatchItem{},
			ProcessedIn: formatProcessingTime(duration),
		}
	}

	matches := make([]MatchItem, 0, len(results))

	for i, result := range results {
		song, ok := songs[result.SongID]
		if !ok {
			// Song metadata not found — skip this result
			// This should not happen in normal operation
			continue
		}

		matches = append(matches, MatchItem{
			Rank:       i + 1, // 1-indexed
			Song:       NewSongResponse(song),
			Score:      result.Score,
			OffsetSecs: result.OffsetSecs,
		})
	}

	confidence := ""
	if len(matches) > 0 {
		confidence = scoreToConfidence(results[0].Score)
	}

	return MatchResponse{
		Found:       len(matches) > 0,
		Confidence:  confidence,
		Matches:     matches,
		ProcessedIn: formatProcessingTime(duration),
	}
}

// NewErrorResponse builds an ErrorResponse with the given error code
// and human-readable message.
func NewErrorResponse(code, message string) ErrorResponse {
	return ErrorResponse{
		Error:   code,
		Message: message,
	}
}

// NewListResponse builds a paginated list response.
func NewListResponse(songs []*models.Song, total, page, pageSize int) ListResponse {
	songResponses := make([]SongResponse, 0, len(songs))
	for _, s := range songs {
		songResponses = append(songResponses, NewSongResponse(s))
	}

	return ListResponse{
		Songs:    songResponses,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}
}

// scoreToConfidence converts a raw coherence score to a confidence label.
func scoreToConfidence(score int) string {
	switch {
	case score >= ConfidenceHighThreshold:
		return ConfidenceHigh
	case score >= ConfidenceMediumThreshold:
		return ConfidenceMedium
	default:
		return ConfidenceLow
	}
}

// formatProcessingTime formats a duration as a human-readable string.
// Uses milliseconds for sub-second durations, seconds for longer ones.
func formatProcessingTime(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}

// SongTitle safely dereferences the Title pointer.
// Returns empty string when Title is nil.
func (s SongResponse) SongTitle() string {
	if s.Title == nil {
		return ""
	}
	return *s.Title
}

// SongArtist safely dereferences the Artist pointer.
// Returns empty string when Artist is nil.
func (s SongResponse) SongArtist() string {
	if s.Artist == nil {
		return ""
	}
	return *s.Artist
}