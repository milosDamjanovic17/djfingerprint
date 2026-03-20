package handlers

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"github.com/milosDamjanovic17/djfingerprint/internal/api/dto"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// buildUploadRequest creates a multipart request simulating a file upload.
func buildUploadRequest(
	t *testing.T,
	fileContent []byte,
	filename string,
	fields map[string]string,
) *http.Request {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	if fileContent != nil {
		part, err := writer.CreateFormFile("file", filename)
		if err != nil {
			t.Fatalf("create form file: %v", err)
		}
		part.Write(fileContent)
	}

	for k, v := range fields {
		writer.WriteField(k, v)
	}
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// minimalWAV returns a minimal valid WAV file structure.
// NOTE: this produces silence — it will not generate fingerprint hashes.
// Use it only for tests that stop before the DSP pipeline.
func minimalWAV() []byte {
	sampleRate := uint32(11025)
	numSamples := uint32(11025)
	byteRate := sampleRate * 2
	dataSize := numSamples * 2
	fileSize := 36 + dataSize

	buf := new(bytes.Buffer)

	buf.WriteString("RIFF")
	writeUint32LE(buf, fileSize)
	buf.WriteString("WAVE")

	buf.WriteString("fmt ")
	writeUint32LE(buf, 16)
	writeUint16LE(buf, 1)
	writeUint16LE(buf, 1)
	writeUint32LE(buf, sampleRate)
	writeUint32LE(buf, byteRate)
	writeUint16LE(buf, 2)
	writeUint16LE(buf, 16)

	buf.WriteString("data")
	writeUint32LE(buf, dataSize)
	buf.Write(make([]byte, dataSize))

	return buf.Bytes()
}

func writeUint32LE(buf *bytes.Buffer, v uint32) {
	buf.Write([]byte{byte(v), byte(v >> 8), byte(v >> 16), byte(v >> 24)})
}

func writeUint16LE(buf *bytes.Buffer, v uint16) {
	buf.Write([]byte{byte(v), byte(v >> 8)})
}

// ffmpegAvailable returns true when FFmpeg is installed and in PATH.
// Used to skip integration tests when FFmpeg is not present.
func ffmpegAvailable() bool {
	_, err := exec.LookPath("ffmpeg")
	return err == nil
}

// -----------------------------------------------------------------------
// Upload — request parsing (no FFmpeg required)
// -----------------------------------------------------------------------

func TestUpload_NotMultipart_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/upload",
		bytes.NewBufferString("not multipart"),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestUpload_MissingFileField_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.WriteField("title", "some track")
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/upload", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file field, got %d", rec.Code)
	}
}

func TestUpload_InvalidAudio_Returns400(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping audio decode test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	// Random bytes — FFmpeg will fail to decode these
	req := buildUploadRequest(t, []byte("this is not audio"), "track.mp3", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid audio, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Upload — duplicate detection (no FFmpeg required)
// -----------------------------------------------------------------------

func TestUpload_DuplicateFile_Returns409(t *testing.T) {
	wavBytes := minimalWAV()
	hash := computeFileHash(wavBytes)

	ss := newMockSongStore()
	existing := makeSong(1, "Already Here", "Artist")
	existing.FileHash = hash
	ss.songs[1] = existing

	h := NewHandler(ss, newMockFingerprintStore())

	req := buildUploadRequest(t, wavBytes, "track.wav", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	// Duplicate check happens before FFmpeg — no FFmpeg needed
	if rec.Code != http.StatusConflict {
		t.Errorf("expected 409 for duplicate file, got %d", rec.Code)
	}
}

func TestUpload_DuplicateFile_ReturnsErrorCode(t *testing.T) {
	wavBytes := minimalWAV()
	hash := computeFileHash(wavBytes)

	ss := newMockSongStore()
	existing := makeSong(1, "Already Here", "Artist")
	existing.FileHash = hash
	ss.songs[1] = existing

	h := NewHandler(ss, newMockFingerprintStore())

	req := buildUploadRequest(t, wavBytes, "track.wav", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["error"] != "duplicate_file" {
		t.Errorf("expected error='duplicate_file', got %v", body["error"])
	}
}

// -----------------------------------------------------------------------
// Upload — store errors (no FFmpeg required)
// -----------------------------------------------------------------------

func TestUpload_HashCheckStoreError_Returns500(t *testing.T) {
	ss := newMockSongStore()
	ss.failOn = "GetSongByHash"

	h := NewHandler(ss, newMockFingerprintStore())

	req := buildUploadRequest(t, minimalWAV(), "track.wav", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Upload — response shape (FFmpeg required)
// -----------------------------------------------------------------------

func TestUpload_ValidWAV_ReturnsJSON(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildUploadRequest(t, noisyWAV(), "track.wav", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestUpload_ValidWAV_NoFileHash_InResponse(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildUploadRequest(t, noisyWAV(), "track.wav", nil)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if _, ok := body["file_hash"]; ok {
		t.Error("file_hash must not appear in upload response")
	}
}

func TestUpload_ValidWAV_WithMetadata_MetadataInResponse(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	fields := map[string]string{
		"title":  "Knock Knock",
		"artist": "DJ Koze",
	}
	req := buildUploadRequest(t, noisyWAV(), "track.wav", fields)
	rec := httptest.NewRecorder()

	h.Upload(rec, req)

	if rec.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: body=%s", rec.Code, rec.Body.String())
	}

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["title"] != "Knock Knock" {
		t.Errorf("expected title='Knock Knock', got %v", body["title"])
	}
	if body["artist"] != "DJ Koze" {
		t.Errorf("expected artist='DJ Koze', got %v", body["artist"])
	}
}

// -----------------------------------------------------------------------
// buildSongFromRequest — pure unit tests, no FFmpeg needed
// -----------------------------------------------------------------------

func TestBuildSongFromRequest_WithAllMetadata(t *testing.T) {
	req := &dto.UploadRequest{
		Title:  "Acid Rain",
		Artist: "Objekt",
		Album:  "Flatland",
	}

	song := buildSongFromRequest(req, "abc123")

	if song.FileHash != "abc123" {
		t.Errorf("expected FileHash='abc123', got %q", song.FileHash)
	}
	if song.Source != "manual" {
		t.Errorf("expected Source='manual', got %q", song.Source)
	}
	if song.Title == nil || *song.Title != "Acid Rain" {
		t.Errorf("expected Title='Acid Rain'")
	}
	if song.Artist == nil || *song.Artist != "Objekt" {
		t.Errorf("expected Artist='Objekt'")
	}
	if song.Album == nil || *song.Album != "Flatland" {
		t.Errorf("expected Album='Flatland'")
	}
}

func TestBuildSongFromRequest_NoMetadata_NilPointers(t *testing.T) {
	req := &dto.UploadRequest{}
	song := buildSongFromRequest(req, "abc123")

	if song.Title != nil {
		t.Error("expected nil Title when not provided")
	}
	if song.Artist != nil {
		t.Error("expected nil Artist when not provided")
	}
	if song.Album != nil {
		t.Error("expected nil Album when not provided")
	}
}

// -----------------------------------------------------------------------
// computeFileHash — pure unit tests, no FFmpeg needed
// -----------------------------------------------------------------------

func TestComputeFileHash_Deterministic(t *testing.T) {
	data := []byte("same data every time")
	h1 := computeFileHash(data)
	h2 := computeFileHash(data)
	if h1 != h2 {
		t.Errorf("hash must be deterministic: %q != %q", h1, h2)
	}
}

func TestComputeFileHash_DifferentData_DifferentHash(t *testing.T) {
	h1 := computeFileHash([]byte("data one"))
	h2 := computeFileHash([]byte("data two"))
	if h1 == h2 {
		t.Error("different data must produce different hashes")
	}
}

func TestComputeFileHash_IsHex(t *testing.T) {
	hash := computeFileHash([]byte("test"))
	for _, c := range hash {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("hash must be lowercase hex, got character %q", c)
		}
	}
}