package handlers

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/dhowden/tag"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/dto"
	"github.com/milosDamjanovic17/djfingerprint/internal/audio"
	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/models"
)

// Upload handles POST /api/v1/upload.
//
// Accepts a multipart/form-data request containing an audio file and
// optional metadata (title, artist, album). Fingerprints the audio,
// checks for duplicates, stores the song and its hashes, and returns
// the created song.
//
// Responses:
//
//	201 Created         — song successfully fingerprinted and stored
//	400 Bad Request     — invalid request or unreadable audio file
//	409 Conflict        — duplicate file already in the library
//	500 Internal Error  — database or processing failure
func (h *Handler) Upload(w http.ResponseWriter, r *http.Request) {
	// ── 1. Parse request ────────────────────────────────────────────
	req, err := dto.ParseUploadRequest(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, dto.ErrCodeInvalidRequest, err.Error())
		return
	}
	defer req.File.Close()

	// ── 2. Read file bytes ───────────────────────────────────────────
	fileBytes, err := io.ReadAll(req.File)
	if err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to read uploaded file",
		)
		return
	}

	// ── 3. Compute SHA-256 hash for duplicate detection ──────────────
	fileHash := computeFileHash(fileBytes)

	// ── 4. Check for duplicate ───────────────────────────────────────
	ctx := r.Context()
	existing, err := h.songStore.GetSongByHash(ctx, fileHash)
	if err != nil && !isNotFound(err) {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to check for duplicate",
		)
		return
	}
	if existing != nil {
		writeError(w,
			http.StatusConflict,
			dto.ErrCodeDuplicateFile,
			"a song with this file already exists in the library",
		)
		return
	}

	// ── 5. Parse ID3 / metadata tags ─────────────────────────────────
	// Attempt to read embedded tags (ID3v1, ID3v2, FLAC, M4A, OGG).
	// Failures are non-fatal — we fall back to form fields or nulls.
	fileTags := readTags(fileBytes)

	// ── 6. Write to temp file for FFmpeg ─────────────────────────────
	// audio.DecodeFile requires a file path — FFmpeg reads from disk.
	tmpFile, err := writeTempFile(fileBytes)
	if err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to prepare audio file",
		)
		return
	}
	defer os.Remove(tmpFile)

	// ── 7. Decode audio via FFmpeg ───────────────────────────────────
	pcm, err := audio.DecodeFile(tmpFile, "")
	if err != nil {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"could not decode audio file — ensure it is a valid WAV or MP3",
		)
		return
	}

	// Duration derived from the decoded PCMData (computed by the decoder from sample count).
	durationMs := int(pcm.Duration.Milliseconds())

	// ── 8. Compute spectrogram ───────────────────────────────────────
	spec, err := dsp.Compute(pcm)
	if err != nil {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"audio file is too short or contains no usable audio",
		)
		return
	}

	// ── 9. Extract peaks and generate hashes ─────────────────────────
	peaks, _ := dsp.ExtractPeaks(spec)

	queryHashes, err := dsp.GenerateHashes(peaks, 0)
	if err != nil || len(queryHashes) == 0 {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"audio file is too short or contains no usable audio",
		)
		return
	}

	// ── 10. Build song record ─────────────────────────────────────────
	song := buildSongFromRequest(req, fileHash, fileTags, durationMs)

	// ── 10. Insert song record ───────────────────────────────────────
	if err := h.songStore.CreateSong(ctx, song); err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to save song",
		)
		return
	}

	// ── 11. Stamp song ID onto hashes and store ──────────────────────
	// models.Fingerprint is the correct type — not models.FingerprintHash
	fingerprints := make([]models.Fingerprint, len(queryHashes))
	for i, qh := range queryHashes {
		fingerprints[i] = models.Fingerprint{
			SongID:     song.ID,
			Hash:       qh.Hash,
			AnchorTime: qh.AnchorTime,
		}
	}

	if err := h.fingerprintStore.StoreFingerprints(ctx, fingerprints); err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to store fingerprint",
		)
		return
	}

	// ── 12. Return 201 Created ───────────────────────────────────────
	writeJSON(w, http.StatusCreated, dto.NewSongResponse(song)) //nolint:errcheck
}

// computeFileHash returns the hex-encoded SHA-256 hash of the given bytes.
func computeFileHash(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum)
}

// writeTempFile writes bytes to a temporary file and returns the path.
// The caller is responsible for removing the file when done.
func writeTempFile(data []byte) (string, error) {
	tmp, err := os.CreateTemp("", "djfp-upload-*.audio")
	if err != nil {
		return "", fmt.Errorf("failed to create temp file: %w", err)
	}
	defer tmp.Close()

	if _, err := tmp.Write(data); err != nil {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("failed to write temp file: %w", err)
	}
	return tmp.Name(), nil
}

// audioTags holds metadata extracted from an audio file's embedded tags.
type audioTags struct {
	Title  string
	Artist string
	Album  string
}

// readTags attempts to parse embedded metadata tags (ID3, FLAC, M4A, OGG)
// from raw audio bytes. Returns nil on any failure — tags are optional.
func readTags(data []byte) *audioTags {
	m, err := tag.ReadFrom(bytes.NewReader(data))
	if err != nil {
		return nil
	}
	return &audioTags{
		Title:  m.Title(),
		Artist: m.Artist(),
		Album:  m.Album(),
	}
}

// buildSongFromRequest constructs a models.Song from an upload request.
// Metadata priority (highest → lowest):
//  1. Explicit form fields (title, artist, album)
//  2. Embedded ID3/FLAC/M4A tags
//  3. Filename parsed as "Artist - Title.ext" (common DJ track naming)
//
// durationMs comes from the decoded PCM sample count.
func buildSongFromRequest(req *dto.UploadRequest, fileHash string, tags *audioTags, durationMs int) *models.Song {
	song := &models.Song{
		FileHash:  fileHash,
		Source:    "manual",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// Populate duration from decoded audio (always available after FFmpeg decode)
	if durationMs > 0 {
		song.DurationMs = &durationMs
	}

	// Parse filename as last-resort fallback ("Artist - Title.mp3")
	filenameArtist, filenameTitle := parseFilename(req.Filename())

	// Resolve title: form field > tag > filename
	title := req.Title
	if title == "" && tags != nil {
		title = tags.Title
	}
	if title == "" {
		title = filenameTitle
	}

	// Resolve artist: form field > tag > filename
	artist := req.Artist
	if artist == "" && tags != nil {
		artist = tags.Artist
	}
	if artist == "" {
		artist = filenameArtist
	}

	// Resolve album: form field > tag (no filename fallback for album)
	album := req.Album
	if album == "" && tags != nil {
		album = tags.Album
	}

	if title != "" {
		song.Title = &title
	}
	if artist != "" {
		song.Artist = &artist
	}
	if album != "" {
		song.Album = &album
	}

	return song
}

// parseFilename extracts artist and title from a filename using the
// common DJ track convention: "Artist - Title.ext"
// Returns empty strings if the pattern is not matched.
func parseFilename(filename string) (artist, title string) {
	// Strip extension
	name := filename
	if idx := strings.LastIndex(name, "."); idx != -1 {
		name = name[:idx]
	}

	// Split on first " - "
	const sep = " - "
	idx := strings.Index(name, sep)
	if idx == -1 {
		return "", ""
	}

	return strings.TrimSpace(name[:idx]), strings.TrimSpace(name[idx+len(sep):])
}

// withTimeout wraps a context with the standard request timeout.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}