package handlers

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

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

	// ── 5. Write to temp file for FFmpeg ─────────────────────────────
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

	// ── 6. Decode audio via FFmpeg ───────────────────────────────────
	pcm, err := audio.DecodeFile(tmpFile, "")
	if err != nil {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"could not decode audio file — ensure it is a valid WAV or MP3",
		)
		return
	}

	// ── 7. Compute spectrogram ───────────────────────────────────────
	spec, err := dsp.Compute(pcm)
	if err != nil {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"audio file is too short or contains no usable audio",
		)
		return
	}

	// ── 8. Extract peaks and generate hashes ─────────────────────────
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

	if len(queryHashes) == 0 {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"audio file is too short or contains no usable audio",
		)
		return
	}

	// ── 9. Build song record ─────────────────────────────────────────
	song := buildSongFromRequest(req, fileHash)

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

// buildSongFromRequest constructs a models.Song from an upload request.
func buildSongFromRequest(req *dto.UploadRequest, fileHash string) *models.Song {
	song := &models.Song{
		FileHash:  fileHash,
		Source:    "manual",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	if req.HasTitle() {
		song.Title = &req.Title
	}
	if req.HasArtist() {
		song.Artist = &req.Artist
	}
	if req.HasAlbum() {
		song.Album = &req.Album
	}
	return song
}

// withTimeout wraps a context with the standard request timeout.
func withTimeout(ctx context.Context, d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, d)
}