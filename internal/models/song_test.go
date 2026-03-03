package models

import (
	"encoding/json"
	"testing"
	"time"
)

// -----------------------------------------------------------------------
// Song — Construction and Zero Values
// -----------------------------------------------------------------------

func TestSong_ZeroValue_NoPanic(t *testing.T) {
	// An empty Song must not panic when methods are called
	var s Song
	_ = s.SongTitle()
	_ = s.SongArtist()
	_ = s.SongAlbum()
	_ = s.HasSpotifyID()
	_ = s.IsManualUpload()
	_ = s.DisplayName()
}

func TestNewSong_SourceIsManual(t *testing.T) {
	s := NewSong("abc123")
	if s.Source != "manual" {
		t.Errorf("expected Source='manual', got %q", s.Source)
	}
}

func TestNewSong_FileHashSet(t *testing.T) {
	const hash = "abc123def456"
	s := NewSong(hash)
	if s.FileHash != hash {
		t.Errorf("expected FileHash=%q, got %q", hash, s.FileHash)
	}
}

func TestNewSong_NullableFieldsAreNil(t *testing.T) {
	s := NewSong("abc123")

	if s.Title != nil {
		t.Error("expected Title=nil for new song")
	}
	if s.Artist != nil {
		t.Error("expected Artist=nil for new song")
	}
	if s.Album != nil {
		t.Error("expected Album=nil for new song")
	}
	if s.SpotifyID != nil {
		t.Error("expected SpotifyID=nil for new song")
	}
	if s.DurationMs != nil {
		t.Error("expected DurationMs=nil for new song")
	}
}

// -----------------------------------------------------------------------
// Song — Helper Methods
// -----------------------------------------------------------------------

func TestSong_SongTitle_NilReturnsEmpty(t *testing.T) {
	s := &Song{}
	if s.SongTitle() != "" {
		t.Errorf("expected empty string for nil Title, got %q", s.SongTitle())
	}
}

func TestSong_SongTitle_NonNilReturnsValue(t *testing.T) {
	s := &Song{Title: StringPtr("Acid Rain")}
	if s.SongTitle() != "Acid Rain" {
		t.Errorf("expected 'Acid Rain', got %q", s.SongTitle())
	}
}

func TestSong_SongArtist_NilReturnsEmpty(t *testing.T) {
	s := &Song{}
	if s.SongArtist() != "" {
		t.Errorf("expected empty string for nil Artist, got %q", s.SongArtist())
	}
}

func TestSong_SongArtist_NonNilReturnsValue(t *testing.T) {
	s := &Song{Artist: StringPtr("DJ Koze")}
	if s.SongArtist() != "DJ Koze" {
		t.Errorf("expected 'DJ Koze', got %q", s.SongArtist())
	}
}

func TestSong_SongAlbum_NilReturnsEmpty(t *testing.T) {
	s := &Song{}
	if s.SongAlbum() != "" {
		t.Errorf("expected empty string for nil Album, got %q", s.SongAlbum())
	}
}

func TestSong_SongAlbum_NonNilReturnsValue(t *testing.T) {
	s := &Song{Album: StringPtr("Knock Knock")}
	if s.SongAlbum() != "Knock Knock" {
		t.Errorf("expected 'Knock Knock', got %q", s.SongAlbum())
	}
}

// -----------------------------------------------------------------------
// Song — HasSpotifyID
// -----------------------------------------------------------------------

func TestSong_HasSpotifyID_NilIsFalse(t *testing.T) {
	s := &Song{}
	if s.HasSpotifyID() {
		t.Error("expected HasSpotifyID=false when SpotifyID is nil")
	}
}

func TestSong_HasSpotifyID_EmptyStringIsFalse(t *testing.T) {
	s := &Song{SpotifyID: StringPtr("")}
	if s.HasSpotifyID() {
		t.Error("expected HasSpotifyID=false for empty SpotifyID string")
	}
}

func TestSong_HasSpotifyID_ValidIDIsTrue(t *testing.T) {
	s := &Song{SpotifyID: StringPtr("3n3Ppam7vgaVa1iaRUIOKE")}
	if !s.HasSpotifyID() {
		t.Error("expected HasSpotifyID=true for valid SpotifyID")
	}
}

// -----------------------------------------------------------------------
// Song — IsManualUpload
// -----------------------------------------------------------------------

func TestSong_IsManualUpload_ManualIsTrue(t *testing.T) {
	s := &Song{Source: "manual"}
	if !s.IsManualUpload() {
		t.Error("expected IsManualUpload=true for source='manual'")
	}
}

func TestSong_IsManualUpload_SpotifyIsFalse(t *testing.T) {
	s := &Song{Source: "spotify"}
	if s.IsManualUpload() {
		t.Error("expected IsManualUpload=false for source='spotify'")
	}
}

// -----------------------------------------------------------------------
// Song — DisplayName
// -----------------------------------------------------------------------

func TestSong_DisplayName_BothKnown(t *testing.T) {
	s := &Song{
		Title:  StringPtr("Knock Knock"),
		Artist: StringPtr("DJ Koze"),
	}
	expected := "DJ Koze - Knock Knock"
	if s.DisplayName() != expected {
		t.Errorf("expected %q, got %q", expected, s.DisplayName())
	}
}

func TestSong_DisplayName_TitleOnly(t *testing.T) {
	s := &Song{Title: StringPtr("Knock Knock")}
	if s.DisplayName() != "Knock Knock" {
		t.Errorf("expected 'Knock Knock', got %q", s.DisplayName())
	}
}

func TestSong_DisplayName_ArtistOnly(t *testing.T) {
	s := &Song{Artist: StringPtr("DJ Koze")}
	if s.DisplayName() != "DJ Koze" {
		t.Errorf("expected 'DJ Koze', got %q", s.DisplayName())
	}
}

func TestSong_DisplayName_NeitherKnown(t *testing.T) {
	s := &Song{}
	if s.DisplayName() != "Unknown Track" {
		t.Errorf("expected 'Unknown Track', got %q", s.DisplayName())
	}
}

// -----------------------------------------------------------------------
// Song — IsValidSource
// -----------------------------------------------------------------------

func TestIsValidSource_ValidValues(t *testing.T) {
	valid := []string{"manual", "spotify", "unknown"}
	for _, v := range valid {
		if !IsValidSource(v) {
			t.Errorf("expected IsValidSource=true for %q", v)
		}
	}
}

func TestIsValidSource_InvalidValues(t *testing.T) {
	invalid := []string{"youtube", "soundcloud", "", "MANUAL", "Manual"}
	for _, v := range invalid {
		if IsValidSource(v) {
			t.Errorf("expected IsValidSource=false for %q", v)
		}
	}
}

// -----------------------------------------------------------------------
// Song — JSON Marshaling
// -----------------------------------------------------------------------

func TestSong_JSON_FieldNames(t *testing.T) {
	s := Song{
		ID:       1,
		Source:   "manual",
		FileHash: "abc123",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	expectedKeys := []string{"id", "source", "file_hash", "created_at", "updated_at"}
	for _, key := range expectedKeys {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q not found in output", key)
		}
	}
}

func TestSong_JSON_NilFieldsOmitted(t *testing.T) {
	// Nullable fields that are nil must be absent from JSON output
	s := Song{
		ID:       1,
		Source:   "manual",
		FileHash: "abc123",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	omittedKeys := []string{"title", "artist", "album", "spotify_id", "duration_ms"}
	for _, key := range omittedKeys {
		if _, ok := m[key]; ok {
			t.Errorf("expected nil field %q to be omitted from JSON, but it was present", key)
		}
	}
}

func TestSong_JSON_NonNilFieldsPresent(t *testing.T) {
	title := "Knock Knock"
	artist := "DJ Koze"
	s := Song{
		ID:     1,
		Title:  &title,
		Artist: &artist,
		Source: "manual",
	}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if m["title"] != title {
		t.Errorf("expected title=%q in JSON, got %v", title, m["title"])
	}
	if m["artist"] != artist {
		t.Errorf("expected artist=%q in JSON, got %v", artist, m["artist"])
	}
}

func TestSong_JSON_RoundTrip(t *testing.T) {
	title := "Acid Rain"
	artist := "Objekt"
	original := Song{
		ID:        42,
		Title:     &title,
		Artist:    &artist,
		Source:    "manual",
		FileHash:  "deadbeef",
		CreatedAt: time.Now().Truncate(time.Second),
		UpdatedAt: time.Now().Truncate(time.Second),
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}

	var decoded Song
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}

	if decoded.ID != original.ID {
		t.Errorf("ID mismatch: %d vs %d", decoded.ID, original.ID)
	}
	if decoded.SongTitle() != original.SongTitle() {
		t.Errorf("Title mismatch: %q vs %q", decoded.SongTitle(), original.SongTitle())
	}
	if decoded.SongArtist() != original.SongArtist() {
		t.Errorf("Artist mismatch: %q vs %q", decoded.SongArtist(), original.SongArtist())
	}
}