package dto

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// buildMultipartRequest builds a multipart/form-data HTTP request
// with the given fields and an optional file attachment.
func buildMultipartRequest(
	t *testing.T,
	fileFieldName string,
	fileName string,
	fileContent []byte,
	extraFields map[string]string,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	// Add the file field
	if fileContent != nil {
		part, err := writer.CreateFormFile(fileFieldName, fileName)
		if err != nil {
			t.Fatalf("failed to create form file: %v", err)
		}
		part.Write(fileContent)
	}

	// Add extra text fields
	for key, val := range extraFields {
		writer.WriteField(key, val)
	}

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// -----------------------------------------------------------------------
// ParseUploadRequest
// -----------------------------------------------------------------------

func TestParseUploadRequest_Success(t *testing.T) {
	content := []byte("fake audio content")
	req := buildMultipartRequest(t, "file", "track.mp3", content, nil)

	upload, err := ParseUploadRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer upload.File.Close()

	if upload.File == nil {
		t.Error("expected non-nil File")
	}
	if upload.Header == nil {
		t.Error("expected non-nil Header")
	}
}

func TestParseUploadRequest_WithMetadata(t *testing.T) {
	content := []byte("fake audio content")
	extras := map[string]string{
		"title":  "Knock Knock",
		"artist": "DJ Koze",
		"album":  "Knock Knock",
	}
	req := buildMultipartRequest(t, "file", "track.mp3", content, extras)

	upload, err := ParseUploadRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer upload.File.Close()

	if upload.Title != "Knock Knock" {
		t.Errorf("expected Title='Knock Knock', got %q", upload.Title)
	}
	if upload.Artist != "DJ Koze" {
		t.Errorf("expected Artist='DJ Koze', got %q", upload.Artist)
	}
	if upload.Album != "Knock Knock" {
		t.Errorf("expected Album='Knock Knock', got %q", upload.Album)
	}
}

func TestParseUploadRequest_MissingFile_ReturnsError(t *testing.T) {
	// Request with no file field at all
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("title", "some title")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := ParseUploadRequest(req)
	if err == nil {
		t.Fatal("expected error for missing file field, got nil")
	}
}

func TestParseUploadRequest_NotMultipart_ReturnsError(t *testing.T) {
	// Plain JSON body — not multipart
	req := httptest.NewRequest(
		http.MethodPost, "/",
		bytes.NewBufferString(`{"title":"test"}`),
	)
	req.Header.Set("Content-Type", "application/json")

	_, err := ParseUploadRequest(req)
	if err == nil {
		t.Fatal("expected error for non-multipart request, got nil")
	}
}

func TestParseUploadRequest_EmptyMetadata_IsAllowed(t *testing.T) {
	// No title, artist, album — they are all optional
	content := []byte("fake audio content")
	req := buildMultipartRequest(t, "file", "track.mp3", content, nil)

	upload, err := ParseUploadRequest(req)
	if err != nil {
		t.Fatalf("expected no error for missing optional fields, got: %v", err)
	}
	defer upload.File.Close()

	if upload.Title != "" {
		t.Errorf("expected empty Title, got %q", upload.Title)
	}
	if upload.Artist != "" {
		t.Errorf("expected empty Artist, got %q", upload.Artist)
	}
	if upload.Album != "" {
		t.Errorf("expected empty Album, got %q", upload.Album)
	}
}

// -----------------------------------------------------------------------
// UploadRequest — Helper Methods
// -----------------------------------------------------------------------

func TestUploadRequest_HasTitle_True(t *testing.T) {
	r := &UploadRequest{Title: "Acid Rain"}
	if !r.HasTitle() {
		t.Error("expected HasTitle=true")
	}
}

func TestUploadRequest_HasTitle_False(t *testing.T) {
	r := &UploadRequest{Title: ""}
	if r.HasTitle() {
		t.Error("expected HasTitle=false for empty title")
	}
}

func TestUploadRequest_HasArtist_True(t *testing.T) {
	r := &UploadRequest{Artist: "Objekt"}
	if !r.HasArtist() {
		t.Error("expected HasArtist=true")
	}
}

func TestUploadRequest_HasArtist_False(t *testing.T) {
	r := &UploadRequest{Artist: ""}
	if r.HasArtist() {
		t.Error("expected HasArtist=false for empty artist")
	}
}

func TestUploadRequest_HasAlbum_True(t *testing.T) {
	r := &UploadRequest{Album: "Flatland"}
	if !r.HasAlbum() {
		t.Error("expected HasAlbum=true")
	}
}

func TestUploadRequest_HasAlbum_False(t *testing.T) {
	r := &UploadRequest{Album: ""}
	if r.HasAlbum() {
		t.Error("expected HasAlbum=false for empty album")
	}
}

func TestUploadRequest_Filename(t *testing.T) {
	r := &UploadRequest{
		Header: &multipart.FileHeader{Filename: "track.mp3"},
	}
	if r.Filename() != "track.mp3" {
		t.Errorf("expected 'track.mp3', got %q", r.Filename())
	}
}

func TestUploadRequest_Filename_NilHeader(t *testing.T) {
	r := &UploadRequest{}
	if r.Filename() != "" {
		t.Errorf("expected empty filename for nil header, got %q", r.Filename())
	}
}

func TestUploadRequest_FileSize(t *testing.T) {
	r := &UploadRequest{
		Header: &multipart.FileHeader{Size: 1024},
	}
	if r.FileSize() != 1024 {
		t.Errorf("expected FileSize=1024, got %d", r.FileSize())
	}
}

func TestUploadRequest_FileSize_NilHeader(t *testing.T) {
	r := &UploadRequest{}
	if r.FileSize() != 0 {
		t.Errorf("expected FileSize=0 for nil header, got %d", r.FileSize())
	}
}

// -----------------------------------------------------------------------
// ParseIdentifyRequest
// -----------------------------------------------------------------------

func TestParseIdentifyRequest_Success(t *testing.T) {
	content := []byte("fake snippet content")
	req := buildMultipartRequest(t, "file", "snippet.mp3", content, nil)

	identify, err := ParseIdentifyRequest(req)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	defer identify.File.Close()

	if identify.File == nil {
		t.Error("expected non-nil File")
	}
	if identify.Header == nil {
		t.Error("expected non-nil Header")
	}
}

func TestParseIdentifyRequest_MissingFile_ReturnsError(t *testing.T) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	_, err := ParseIdentifyRequest(req)
	if err == nil {
		t.Fatal("expected error for missing file field, got nil")
	}
}

func TestParseIdentifyRequest_NotMultipart_ReturnsError(t *testing.T) {
	req := httptest.NewRequest(
		http.MethodPost, "/",
		bytes.NewBufferString("binary audio data"),
	)
	req.Header.Set("Content-Type", "application/octet-stream")

	_, err := ParseIdentifyRequest(req)
	if err == nil {
		t.Fatal("expected error for non-multipart request, got nil")
	}
}

func TestIdentifyRequest_Filename(t *testing.T) {
	r := &IdentifyRequest{
		Header: &multipart.FileHeader{Filename: "snippet.wav"},
	}
	if r.Filename() != "snippet.wav" {
		t.Errorf("expected 'snippet.wav', got %q", r.Filename())
	}
}

func TestIdentifyRequest_Filename_NilHeader(t *testing.T) {
	r := &IdentifyRequest{}
	if r.Filename() != "" {
		t.Errorf("expected empty filename for nil header, got %q", r.Filename())
	}
}

// -----------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------

func TestConstants_MaxUploadSize(t *testing.T) {
	expected := int64(500 << 20)
	if MaxUploadSize != expected {
		t.Errorf("expected MaxUploadSize=%d, got %d", expected, MaxUploadSize)
	}
}

func TestConstants_MaxSnippetSize(t *testing.T) {
	expected := int64(10 << 20)
	if MaxSnippetSize != expected {
		t.Errorf("expected MaxSnippetSize=%d, got %d", expected, MaxSnippetSize)
	}
}