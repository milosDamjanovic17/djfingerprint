package handlers

import (
	"bytes"
	"encoding/json"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

// -----------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------

// buildIdentifyRequest creates a multipart request for the identify endpoint.
func buildIdentifyRequest(t *testing.T, fileContent []byte, filename string) *http.Request {
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

	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identify", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

// noisyWAV returns a WAV file containing a sine wave tone.
// Unlike minimalWAV (silence), this produces spectral peaks that
// the DSP pipeline can fingerprint.
// 3 seconds at 11025 Hz mono 16-bit, 440 Hz sine wave.
func noisyWAV() []byte {
	const sampleRate = 11025
	const durationSecs = 3
	const frequency = 440.0 // A4 tone
	const amplitude = 16000 // below int16 max of 32767

	numSamples := sampleRate * durationSecs
	dataSize := uint32(numSamples * 2) // 16-bit = 2 bytes per sample
	fileSize := 36 + dataSize

	buf := new(bytes.Buffer)

	// RIFF header
	buf.WriteString("RIFF")
	writeUint32LE(buf, fileSize)
	buf.WriteString("WAVE")

	// fmt chunk
	buf.WriteString("fmt ")
	writeUint32LE(buf, 16)
	writeUint16LE(buf, 1)              // PCM
	writeUint16LE(buf, 1)              // mono
	writeUint32LE(buf, sampleRate)
	writeUint32LE(buf, sampleRate*2)   // byte rate
	writeUint16LE(buf, 2)              // block align
	writeUint16LE(buf, 16)             // bits per sample

	// data chunk
	buf.WriteString("data")
	writeUint32LE(buf, dataSize)

	for i := 0; i < numSamples; i++ {
		t := float64(i) / float64(sampleRate)
		sample := int16(amplitude * math.Sin(2*math.Pi*frequency*t))
		writeUint16LE(buf, uint16(sample))
	}

	return buf.Bytes()
}

// -----------------------------------------------------------------------
// Identify — request parsing (no FFmpeg required)
// -----------------------------------------------------------------------

func TestIdentify_NotMultipart_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := httptest.NewRequest(
		http.MethodPost, "/api/v1/identify",
		bytes.NewBufferString("not multipart"),
	)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rec.Code)
	}
}

func TestIdentify_MissingFileField_Returns400(t *testing.T) {
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/identify", &body)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for missing file, got %d", rec.Code)
	}
}

func TestIdentify_InvalidAudio_Returns400(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping audio decode test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, []byte("not audio data"), "snippet.mp3")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for invalid audio, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Identify — store errors (FFmpeg required)
// -----------------------------------------------------------------------

func TestIdentify_QueryStoreError_Returns500(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping store error test")
	}

	fs := newMockFingerprintStore()
	fs.failOn = "QueryByHashes"

	h := NewHandler(newMockSongStore(), fs)

	// Use noisyWAV so the pipeline reaches the QueryByHashes call
	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rec.Code)
	}
}

// -----------------------------------------------------------------------
// Identify — full pipeline (FFmpeg required)
// -----------------------------------------------------------------------

func TestIdentify_ValidSnippet_Returns200(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestIdentify_ValidSnippet_ResponseIsJSON(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}
}

func TestIdentify_NoMatch_FoundIsFalse(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	// Empty fingerprint store — no hashes to match against
	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["found"] != false {
		t.Errorf("expected found=false for no match, got %v", body["found"])
	}
}

func TestIdentify_NoMatch_MatchesIsEmptyArray(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	matches, ok := body["matches"].([]interface{})
	if !ok {
		t.Fatal("expected matches to be an array")
	}
	if len(matches) != 0 {
		t.Errorf("expected empty matches array, got %d items", len(matches))
	}
}

func TestIdentify_ResponseContainsProcessedIn(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	if body["processed_in"] == "" || body["processed_in"] == nil {
		t.Error("expected non-empty processed_in in response")
	}
}

func TestIdentify_ResponseShape_AllRequiredKeys(t *testing.T) {
	if !ffmpegAvailable() {
		t.Skip("FFmpeg not available — skipping full pipeline test")
	}

	h := NewHandler(newMockSongStore(), newMockFingerprintStore())

	req := buildIdentifyRequest(t, noisyWAV(), "snippet.wav")
	rec := httptest.NewRecorder()

	h.Identify(rec, req)

	var body map[string]interface{}
	json.NewDecoder(rec.Body).Decode(&body)

	required := []string{"found", "confidence", "matches", "processed_in"}
	for _, key := range required {
		if _, ok := body[key]; !ok {
			t.Errorf("expected key %q in response body", key)
		}
	}
}