package dto

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/matcher"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// testSong builds a models.Song for use in response tests.
func testSong(id int, title, artist string) *models.Song {
	return &models.Song{
		ID:        id,
		Title:     models.StringPtr(title),
		Artist:    models.StringPtr(artist),
		Source:    "manual",
		FileHash:  "abc123def456",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}

// -----------------------------------------------------------------------
// NewSongResponse
// -----------------------------------------------------------------------

func TestNewSongResponse_FieldsPopulated(t *testing.T) {
	song := testSong(42, "Knock Knock", "DJ Koze")
	resp := NewSongResponse(song)

	if resp.ID != 42 {
		t.Errorf("expected ID=42, got %d", resp.ID)
	}
	if resp.SongTitle() != "Knock Knock" {
		t.Errorf("expected Title='Knock Knock', got %q", resp.SongTitle())
	}
	if resp.SongArtist() != "DJ Koze" {
		t.Errorf("expected Artist='DJ Koze', got %q", resp.SongArtist())
	}
	if resp.Source != "manual" {
		t.Errorf("expected Source='manual', got %q", resp.Source)
	}
}

func TestNewSongResponse_FileHash_Excluded(t *testing.T) {
	song := testSong(1, "Track", "Artist")
	resp := NewSongResponse(song)

	// Marshal to JSON and verify file_hash is not present
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if _, ok := m["file_hash"]; ok {
		t.Error("file_hash must not appear in SongResponse JSON — it is an internal field")
	}
}

func TestNewSongResponse_NilFields_OmittedFromJSON(t *testing.T) {
	song := &models.Song{
		ID:       1,
		Source:   "manual",
		FileHash: "abc123",
	}
	// Title, Artist, Album all nil

	resp := NewSongResponse(song)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	omitted := []string{"title", "artist", "album", "spotify_id", "duration_ms"}
	for _, key := range omitted {
		if _, ok := m[key]; ok {
			t.Errorf("nil field %q should be omitted from JSON", key)
		}
	}
}

func TestNewSongResponse_JSON_FieldNames(t *testing.T) {
	song := testSong(1, "Track", "Artist")
	resp := NewSongResponse(song)

	data, _ := json.Marshal(resp)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	required := []string{"id", "source", "created_at"}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q not found in SongResponse", key)
		}
	}
}

// -----------------------------------------------------------------------
// SongResponse helper methods
// -----------------------------------------------------------------------

func TestSongResponse_SongTitle_NilReturnsEmpty(t *testing.T) {
	resp := SongResponse{}
	if resp.SongTitle() != "" {
		t.Errorf("expected empty string for nil Title, got %q", resp.SongTitle())
	}
}

func TestSongResponse_SongTitle_NonNilReturnsValue(t *testing.T) {
	title := "Acid Rain"
	resp := SongResponse{Title: &title}
	if resp.SongTitle() != "Acid Rain" {
		t.Errorf("expected 'Acid Rain', got %q", resp.SongTitle())
	}
}

func TestSongResponse_SongArtist_NilReturnsEmpty(t *testing.T) {
	resp := SongResponse{}
	if resp.SongArtist() != "" {
		t.Errorf("expected empty string for nil Artist, got %q", resp.SongArtist())
	}
}

func TestSongResponse_SongArtist_NonNilReturnsValue(t *testing.T) {
	artist := "Objekt"
	resp := SongResponse{Artist: &artist}
	if resp.SongArtist() != "Objekt" {
		t.Errorf("expected 'Objekt', got %q", resp.SongArtist())
	}
}

// -----------------------------------------------------------------------
// scoreToConfidence
// -----------------------------------------------------------------------

func TestScoreToConfidence_High(t *testing.T) {
	cases := []int{ConfidenceHighThreshold, 100, 500, 847}
	for _, score := range cases {
		result := scoreToConfidence(score)
		if result != ConfidenceHigh {
			t.Errorf("score %d: expected %q, got %q", score, ConfidenceHigh, result)
		}
	}
}

func TestScoreToConfidence_Medium(t *testing.T) {
	cases := []int{ConfidenceMediumThreshold, 20, 30, ConfidenceHighThreshold - 1}
	for _, score := range cases {
		result := scoreToConfidence(score)
		if result != ConfidenceMedium {
			t.Errorf("score %d: expected %q, got %q", score, ConfidenceMedium, result)
		}
	}
}

func TestScoreToConfidence_Low(t *testing.T) {
	cases := []int{5, 8, 10, ConfidenceMediumThreshold - 1}
	for _, score := range cases {
		result := scoreToConfidence(score)
		if result != ConfidenceLow {
			t.Errorf("score %d: expected %q, got %q", score, ConfidenceLow, result)
		}
	}
}

func TestScoreToConfidence_ExactBoundaries(t *testing.T) {
	if scoreToConfidence(ConfidenceHighThreshold) != ConfidenceHigh {
		t.Errorf("score at HighThreshold should be %q", ConfidenceHigh)
	}
	if scoreToConfidence(ConfidenceHighThreshold-1) != ConfidenceMedium {
		t.Errorf("score just below HighThreshold should be %q", ConfidenceMedium)
	}
	if scoreToConfidence(ConfidenceMediumThreshold) != ConfidenceMedium {
		t.Errorf("score at MediumThreshold should be %q", ConfidenceMedium)
	}
	if scoreToConfidence(ConfidenceMediumThreshold-1) != ConfidenceLow {
		t.Errorf("score just below MediumThreshold should be %q", ConfidenceLow)
	}
}

// -----------------------------------------------------------------------
// formatProcessingTime
// -----------------------------------------------------------------------

func TestFormatProcessingTime_Milliseconds(t *testing.T) {
	result := formatProcessingTime(450 * time.Millisecond)
	if result != "450ms" {
		t.Errorf("expected '450ms', got %q", result)
	}
}

func TestFormatProcessingTime_Seconds(t *testing.T) {
	result := formatProcessingTime(1230 * time.Millisecond)
	if result != "1.23s" {
		t.Errorf("expected '1.23s', got %q", result)
	}
}

func TestFormatProcessingTime_ExactlyOneSecond(t *testing.T) {
	result := formatProcessingTime(1 * time.Second)
	if result != "1.00s" {
		t.Errorf("expected '1.00s', got %q", result)
	}
}

// -----------------------------------------------------------------------
// NewMatchResponse — No Results
// -----------------------------------------------------------------------

func TestNewMatchResponse_NoResults_FoundIsFalse(t *testing.T) {
	resp := NewMatchResponse(
		[]matcher.MatchResult{},
		map[int]*models.Song{},
		100*time.Millisecond,
	)

	if resp.Found {
		t.Error("expected Found=false for empty results")
	}
}

func TestNewMatchResponse_NoResults_MatchesIsEmptySlice(t *testing.T) {
	resp := NewMatchResponse(
		[]matcher.MatchResult{},
		map[int]*models.Song{},
		100*time.Millisecond,
	)

	if resp.Matches == nil {
		t.Error("expected empty slice, not nil — JSON must encode as [] not null")
	}
	if len(resp.Matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(resp.Matches))
	}
}

func TestNewMatchResponse_NoResults_ConfidenceIsEmpty(t *testing.T) {
	resp := NewMatchResponse(
		[]matcher.MatchResult{},
		map[int]*models.Song{},
		100*time.Millisecond,
	)

	if resp.Confidence != "" {
		t.Errorf("expected empty confidence for no results, got %q", resp.Confidence)
	}
}

func TestNewMatchResponse_NoResults_ProcessedInPopulated(t *testing.T) {
	resp := NewMatchResponse(
		[]matcher.MatchResult{},
		map[int]*models.Song{},
		450*time.Millisecond,
	)

	if resp.ProcessedIn == "" {
		t.Error("expected non-empty ProcessedIn even for no results")
	}
}

// -----------------------------------------------------------------------
// NewMatchResponse — With Results
// -----------------------------------------------------------------------

func TestNewMatchResponse_WithResults_FoundIsTrue(t *testing.T) {
	results := []matcher.MatchResult{
		{SongID: 1, Score: 100, Offset: 970, OffsetSecs: 45.1},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "Knock Knock", "DJ Koze"),
	}

	resp := NewMatchResponse(results, songs, 200*time.Millisecond)

	if !resp.Found {
		t.Error("expected Found=true when results present")
	}
}

func TestNewMatchResponse_WithResults_MatchCount(t *testing.T) {
	results := []matcher.MatchResult{
		{SongID: 1, Score: 100, Offset: 970},
		{SongID: 2, Score: 50, Offset: 200},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "Track A", "Artist A"),
		2: testSong(2, "Track B", "Artist B"),
	}

	resp := NewMatchResponse(results, songs, 200*time.Millisecond)

	if len(resp.Matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(resp.Matches))
	}
}

func TestNewMatchResponse_WithResults_Ranks_OneIndexed(t *testing.T) {
	results := []matcher.MatchResult{
		{SongID: 1, Score: 100},
		{SongID: 2, Score: 50},
		{SongID: 3, Score: 20},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "A", "A"),
		2: testSong(2, "B", "B"),
		3: testSong(3, "C", "C"),
	}

	resp := NewMatchResponse(results, songs, 100*time.Millisecond)

	for i, match := range resp.Matches {
		expectedRank := i + 1
		if match.Rank != expectedRank {
			t.Errorf("match[%d]: expected Rank=%d, got %d", i, expectedRank, match.Rank)
		}
	}
}

func TestNewMatchResponse_WithResults_ConfidenceHigh(t *testing.T) {
	results := []matcher.MatchResult{
		{SongID: 1, Score: ConfidenceHighThreshold},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "Track", "Artist"),
	}

	resp := NewMatchResponse(results, songs, 100*time.Millisecond)

	if resp.Confidence != ConfidenceHigh {
		t.Errorf("expected confidence=%q for score=%d, got %q",
			ConfidenceHigh, ConfidenceHighThreshold, resp.Confidence)
	}
}

func TestNewMatchResponse_WithResults_OffsetSecs(t *testing.T) {
	const expectedOffset = 45.14

	results := []matcher.MatchResult{
		{SongID: 1, Score: 100, OffsetSecs: expectedOffset},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "Track", "Artist"),
	}

	resp := NewMatchResponse(results, songs, 100*time.Millisecond)

	if len(resp.Matches) == 0 {
		t.Fatal("expected at least one match")
	}
	if resp.Matches[0].OffsetSecs != expectedOffset {
		t.Errorf("expected OffsetSecs=%f, got %f",
			expectedOffset, resp.Matches[0].OffsetSecs)
	}
}

func TestNewMatchResponse_MissingSongMetadata_SkipsResult(t *testing.T) {
	// If a song ID from the matcher has no entry in the songs map
	// the match is skipped — not included in the response
	results := []matcher.MatchResult{
		{SongID: 999, Score: 100}, // no entry in songs map
	}
	songs := map[int]*models.Song{} // empty

	resp := NewMatchResponse(results, songs, 100*time.Millisecond)

	if len(resp.Matches) != 0 {
		t.Errorf("expected 0 matches when song metadata missing, got %d", len(resp.Matches))
	}
}

// -----------------------------------------------------------------------
// NewErrorResponse
// -----------------------------------------------------------------------

func TestNewErrorResponse_FieldsSet(t *testing.T) {
	resp := NewErrorResponse(ErrCodeNotFound, "song not found")

	if resp.Error != ErrCodeNotFound {
		t.Errorf("expected Error=%q, got %q", ErrCodeNotFound, resp.Error)
	}
	if resp.Message != "song not found" {
		t.Errorf("expected Message='song not found', got %q", resp.Message)
	}
}

func TestNewErrorResponse_JSON(t *testing.T) {
	resp := NewErrorResponse(ErrCodeDuplicateFile, "already exists")
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	if m["error"] != ErrCodeDuplicateFile {
		t.Errorf("expected error=%q in JSON, got %v", ErrCodeDuplicateFile, m["error"])
	}
	if m["message"] != "already exists" {
		t.Errorf("expected message='already exists' in JSON, got %v", m["message"])
	}
}

// -----------------------------------------------------------------------
// Error Code Constants
// -----------------------------------------------------------------------

func TestErrorCodeConstants_Defined(t *testing.T) {
	codes := []string{
		ErrCodeDuplicateFile,
		ErrCodeInvalidFile,
		ErrCodeFileTooLarge,
		ErrCodeNotFound,
		ErrCodeInternalError,
		ErrCodeNoMatch,
		ErrCodeInvalidRequest,
	}

	for _, code := range codes {
		if code == "" {
			t.Errorf("error code constant must not be empty string")
		}
	}
}

// -----------------------------------------------------------------------
// NewListResponse
// -----------------------------------------------------------------------

func TestNewListResponse_FieldsSet(t *testing.T) {
	songs := []*models.Song{
		testSong(1, "Track A", "Artist A"),
		testSong(2, "Track B", "Artist B"),
	}

	resp := NewListResponse(songs, 10, 1, 20)

	if len(resp.Songs) != 2 {
		t.Errorf("expected 2 songs, got %d", len(resp.Songs))
	}
	if resp.Total != 10 {
		t.Errorf("expected Total=10, got %d", resp.Total)
	}
	if resp.Page != 1 {
		t.Errorf("expected Page=1, got %d", resp.Page)
	}
	if resp.PageSize != 20 {
		t.Errorf("expected PageSize=20, got %d", resp.PageSize)
	}
}

func TestNewListResponse_EmptySongs(t *testing.T) {
	resp := NewListResponse([]*models.Song{}, 0, 1, 20)

	if len(resp.Songs) != 0 {
		t.Errorf("expected 0 songs, got %d", len(resp.Songs))
	}
	if resp.Total != 0 {
		t.Errorf("expected Total=0, got %d", resp.Total)
	}
}

func TestNewListResponse_FileHash_ExcludedFromSongs(t *testing.T) {
	songs := []*models.Song{testSong(1, "Track", "Artist")}
	resp := NewListResponse(songs, 1, 1, 20)

	data, _ := json.Marshal(resp)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	songList, ok := m["songs"].([]interface{})
	if !ok || len(songList) == 0 {
		t.Fatal("expected songs array in JSON")
	}

	firstSong := songList[0].(map[string]interface{})
	if _, ok := firstSong["file_hash"]; ok {
		t.Error("file_hash must not appear in list response songs")
	}
}

// -----------------------------------------------------------------------
// MatchResponse — JSON Shape
// -----------------------------------------------------------------------

func TestMatchResponse_JSON_Shape(t *testing.T) {
	results := []matcher.MatchResult{
		{SongID: 1, Score: 100, OffsetSecs: 45.1},
	}
	songs := map[int]*models.Song{
		1: testSong(1, "Knock Knock", "DJ Koze"),
	}

	resp := NewMatchResponse(results, songs, 200*time.Millisecond)
	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	json.Unmarshal(data, &m)

	required := []string{"found", "confidence", "matches", "processed_in"}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q not found in MatchResponse", key)
		}
	}
}

func TestMatchResponse_EmptyMatches_JSON_IsArray(t *testing.T) {
	// matches must encode as [] not null in JSON
	resp := NewMatchResponse(
		[]matcher.MatchResult{},
		map[int]*models.Song{},
		100*time.Millisecond,
	)

	data, _ := json.Marshal(resp)
	var m map[string]interface{}
	json.Unmarshal(data, &m)

	matches, ok := m["matches"]
	if !ok {
		t.Fatal("expected matches key in JSON")
	}

	// Must be a JSON array ([]interface{}) not null
	if matches == nil {
		t.Error("matches must be [] not null in JSON")
	}
}