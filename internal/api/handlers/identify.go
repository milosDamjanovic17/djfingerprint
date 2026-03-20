package handlers

import (
	"io"
	"net/http"
	"os"
	"time"

	"github.com/milosDamjanovic17/djfingerprint/internal/api/dto"
	"github.com/milosDamjanovic17/djfingerprint/internal/audio"
	"github.com/milosDamjanovic17/djfingerprint/internal/dsp"
	"github.com/milosDamjanovic17/djfingerprint/internal/matcher"
)

// Identify handles POST /api/v1/identify.
//
// Accepts a multipart/form-data request containing a short audio snippet.
// Fingerprints the snippet, queries the hash store for candidates,
// runs coherence scoring, and returns ranked match results.
//
// A 200 OK is returned even when no match is found — the response body
// carries found: false to communicate the outcome. This is intentional:
// "no match" is a valid result, not an error condition.
//
// Responses:
//
//	200 OK              — identification complete (match or no match)
//	400 Bad Request     — invalid request or unreadable audio snippet
//	500 Internal Error  — database or processing failure
func (h *Handler) Identify(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// ── 1. Parse request ─────────────────────────────────────────────
	req, err := dto.ParseIdentifyRequest(r)
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
			"failed to read snippet",
		)
		return
	}

	// ── 3. Write to temp file for FFmpeg ─────────────────────────────
	tmpFile, err := writeTempFile(fileBytes)
	if err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to prepare snippet file",
		)
		return
	}
	defer os.Remove(tmpFile)

	// ── 4. Decode audio via FFmpeg ───────────────────────────────────
	pcm, err := audio.DecodeFile(tmpFile, "")
	if err != nil {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidFile,
			"could not decode audio snippet — ensure it is a valid WAV or MP3",
		)
		return
	}

	// ── 5. Compute spectrogram ───────────────────────────────────────
	spec, err := dsp.Compute(pcm)
	if err != nil {
		// Snippet too short or silent — valid input, just unidentifiable
		writeJSON(w, http.StatusOK, dto.NewMatchResponse( //nolint:errcheck
			nil,
			nil,
			time.Since(start),
		))
		return
	}

	// ── 6. Extract peaks ─────────────────────────────────────────────
	peaks, _ := dsp.ExtractPeaks(spec)

	// ── 7. Generate hashes ───────────────────────────────────────────
	// GenerateHashes may fail when audio has no distinguishable peaks
	// (silence, white noise, etc.). This is not a server error —
	// return no-match rather than 500.
	queryHashes, err := dsp.GenerateHashes(peaks, 0)
	if err != nil || len(queryHashes) == 0 {
		writeJSON(w, http.StatusOK, dto.NewMatchResponse( //nolint:errcheck
			nil,
			nil,
			time.Since(start),
		))
		return
	}

	// ── 8. Extract hash values for database query ────────────────────
	hashValues := make([]int32, len(queryHashes))
	for i, qh := range queryHashes {
		hashValues[i] = qh.Hash
	}

	// ── 9. Query database for matching hashes ────────────────────────
	ctx := r.Context()
	dbResults, err := h.fingerprintStore.QueryByHashes(ctx, hashValues)
	if err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to query fingerprint store",
		)
		return
	}

	// ── 10. Run coherence scoring ────────────────────────────────────
	matchResults := matcher.Match(queryHashes, dbResults)

	// ── 11. Fetch song metadata for matched song IDs ─────────────────
	songIDs := make([]int, len(matchResults))
	for i, mr := range matchResults {
		songIDs[i] = mr.SongID
	}
	songs := h.getSongsByIDs(ctx, songIDs)

	// ── 12. Build and return response ────────────────────────────────
	resp := dto.NewMatchResponse(matchResults, songs, time.Since(start))
	writeJSON(w, http.StatusOK, resp) //nolint:errcheck
}