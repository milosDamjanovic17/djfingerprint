package models

import (
	"encoding/json"
	"testing"
)

// -----------------------------------------------------------------------
// SongFeature — Construction
// -----------------------------------------------------------------------

func TestSongFeature_ZeroValue_NoPanic(t *testing.T) {
	var sf SongFeature
	_ = sf.KeyName()
	_ = sf.IsComplete()
}

func TestNewSongFeature_SongIDSet(t *testing.T) {
	sf := NewSongFeature(99)
	if sf.SongID != 99 {
		t.Errorf("expected SongID=99, got %d", sf.SongID)
	}
}

func TestNewSongFeature_AllFeaturesNil(t *testing.T) {
	sf := NewSongFeature(1)

	if sf.Energy != nil {
		t.Error("expected Energy=nil")
	}
	if sf.Danceability != nil {
		t.Error("expected Danceability=nil")
	}
	if sf.Valence != nil {
		t.Error("expected Valence=nil")
	}
	if sf.Acousticness != nil {
		t.Error("expected Acousticness=nil")
	}
	if sf.Instrumentalness != nil {
		t.Error("expected Instrumentalness=nil")
	}
	if sf.Liveness != nil {
		t.Error("expected Liveness=nil")
	}
	if sf.Speechiness != nil {
		t.Error("expected Speechiness=nil")
	}
	if sf.Loudness != nil {
		t.Error("expected Loudness=nil")
	}
	if sf.Tempo != nil {
		t.Error("expected Tempo=nil")
	}
	if sf.MusicalKey != nil {
		t.Error("expected MusicalKey=nil")
	}
}

// -----------------------------------------------------------------------
// SongFeature — IsComplete
// -----------------------------------------------------------------------

func TestSongFeature_IsComplete_AllNil(t *testing.T) {
	sf := NewSongFeature(1)
	if sf.IsComplete() {
		t.Error("expected IsComplete=false when all features are nil")
	}
}

func TestSongFeature_IsComplete_SomeNil(t *testing.T) {
	sf := NewSongFeature(1)
	sf.Energy = Float64Ptr(0.8)
	sf.Danceability = Float64Ptr(0.7)
	// rest remain nil

	if sf.IsComplete() {
		t.Error("expected IsComplete=false when some features are nil")
	}
}

func TestSongFeature_IsComplete_AllSet(t *testing.T) {
	sf := NewSongFeature(1)
	sf.Energy = Float64Ptr(0.8)
	sf.Danceability = Float64Ptr(0.7)
	sf.Valence = Float64Ptr(0.6)
	sf.Acousticness = Float64Ptr(0.1)
	sf.Instrumentalness = Float64Ptr(0.9)
	sf.Liveness = Float64Ptr(0.2)
	sf.Speechiness = Float64Ptr(0.05)
	sf.Loudness = Float64Ptr(-5.0)
	sf.Tempo = Float64Ptr(128.0)
	sf.MusicalKey = IntPtr(5)

	if !sf.IsComplete() {
		t.Error("expected IsComplete=true when all features are set")
	}
}

// -----------------------------------------------------------------------
// SongFeature — KeyName
// -----------------------------------------------------------------------

func TestSongFeature_KeyName_NilReturnsUnknown(t *testing.T) {
	sf := &SongFeature{}
	if sf.KeyName() != "Unknown" {
		t.Errorf("expected KeyName='Unknown' for nil key, got %q", sf.KeyName())
	}
}

func TestSongFeature_KeyName_NegativeReturnsUnknown(t *testing.T) {
	sf := &SongFeature{MusicalKey: IntPtr(-1)}
	if sf.KeyName() != "Unknown" {
		t.Errorf("expected KeyName='Unknown' for key=-1, got %q", sf.KeyName())
	}
}

func TestSongFeature_KeyName_AllTwelveKeys(t *testing.T) {
	expected := []string{
		"C", "C#", "D", "D#", "E", "F",
		"F#", "G", "G#", "A", "A#", "B",
	}

	for i, name := range expected {
		sf := &SongFeature{MusicalKey: IntPtr(i)}
		if sf.KeyName() != name {
			t.Errorf("key %d: expected %q, got %q", i, name, sf.KeyName())
		}
	}
}

func TestSongFeature_KeyName_OutOfRangeReturnsUnknown(t *testing.T) {
	sf := &SongFeature{MusicalKey: IntPtr(12)}
	if sf.KeyName() != "Unknown" {
		t.Errorf("expected 'Unknown' for key=12, got %q", sf.KeyName())
	}
}

// -----------------------------------------------------------------------
// Helper Functions
// -----------------------------------------------------------------------

func TestFloat64Val_NilReturnsZero(t *testing.T) {
	result := Float64Val(nil)
	if result != 0.0 {
		t.Errorf("expected 0.0 for nil input, got %f", result)
	}
}

func TestFloat64Val_NonNilReturnsValue(t *testing.T) {
	result := Float64Val(Float64Ptr(0.75))
	if result != 0.75 {
		t.Errorf("expected 0.75, got %f", result)
	}
}

func TestFloat64Val_ZeroValueDistinguishable(t *testing.T) {
	// Float64Val(nil) and Float64Val(ptr to 0.0) both return 0.0
	// HasFeature() is what distinguishes them
	fromNil := Float64Val(nil)
	fromZero := Float64Val(Float64Ptr(0.0))

	if fromNil != 0.0 || fromZero != 0.0 {
		t.Error("both nil and zero pointer should return 0.0 from Float64Val")
	}
}

func TestIntVal_NilReturnsZero(t *testing.T) {
	result := IntVal(nil)
	if result != 0 {
		t.Errorf("expected 0 for nil input, got %d", result)
	}
}

func TestIntVal_NonNilReturnsValue(t *testing.T) {
	result := IntVal(IntPtr(7))
	if result != 7 {
		t.Errorf("expected 7, got %d", result)
	}
}

func TestHasFeature_NilIsFalse(t *testing.T) {
	if HasFeature(nil) {
		t.Error("expected HasFeature=false for nil")
	}
}

func TestHasFeature_NonNilIsTrue(t *testing.T) {
	if !HasFeature(Float64Ptr(0.0)) {
		t.Error("expected HasFeature=true for non-nil pointer to 0.0")
	}
}

func TestStringPtr_ReturnsPointer(t *testing.T) {
	s := StringPtr("hello")
	if s == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *s != "hello" {
		t.Errorf("expected 'hello', got %q", *s)
	}
}

func TestFloat64Ptr_ReturnsPointer(t *testing.T) {
	f := Float64Ptr(3.14)
	if f == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *f != 3.14 {
		t.Errorf("expected 3.14, got %f", *f)
	}
}

func TestIntPtr_ReturnsPointer(t *testing.T) {
	i := IntPtr(42)
	if i == nil {
		t.Fatal("expected non-nil pointer")
	}
	if *i != 42 {
		t.Errorf("expected 42, got %d", *i)
	}
}

// -----------------------------------------------------------------------
// SongFeature — JSON
// -----------------------------------------------------------------------

func TestSongFeature_JSON_NilFeaturesOmitted(t *testing.T) {
	sf := SongFeature{
		ID:     1,
		SongID: 42,
	}

	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	omitted := []string{
		"energy", "danceability", "valence", "acousticness",
		"instrumentalness", "liveness", "speechiness",
		"loudness", "tempo", "musical_key", "spotify_track_id",
	}

	for _, key := range omitted {
		if _, ok := m[key]; ok {
			t.Errorf("expected nil field %q to be omitted from JSON", key)
		}
	}
}

func TestSongFeature_JSON_PopulatedFeaturesPresent(t *testing.T) {
	sf := SongFeature{
		ID:           1,
		SongID:       42,
		Energy:       Float64Ptr(0.85),
		Danceability: Float64Ptr(0.72),
		Tempo:        Float64Ptr(128.0),
		MusicalKey:   IntPtr(5),
	}

	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if m["energy"] == nil {
		t.Error("expected energy to be present in JSON")
	}
	if m["danceability"] == nil {
		t.Error("expected danceability to be present in JSON")
	}
	if m["tempo"] == nil {
		t.Error("expected tempo to be present in JSON")
	}
	if m["musical_key"] == nil {
		t.Error("expected musical_key to be present in JSON")
	}
}

func TestSongFeature_JSON_FieldNames(t *testing.T) {
	sf := SongFeature{
		ID:     1,
		SongID: 42,
	}

	data, err := json.Marshal(sf)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	required := []string{"id", "song_id", "created_at"}
	for _, key := range required {
		if _, ok := m[key]; !ok {
			t.Errorf("expected JSON key %q not found", key)
		}
	}
}