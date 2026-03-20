package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/milosDamjanovic17/djfingerprint/internal/api/dto"
)

const (
	// defaultPage is the default page number for list requests.
	defaultPage = 1

	// defaultPageSize is the default number of songs per page.
	defaultPageSize = 20

	// maxPageSize is the maximum allowed page size to prevent
	// returning the entire library in one request.
	maxPageSize = 100
)

// ListSongs handles GET /api/v1/songs.
//
// Returns a paginated list of songs in the library.
// Accepts optional query parameters: page and page_size.
//
// Responses:
//
//	200 OK          — list of songs with pagination metadata
//	500 Internal    — database failure
func (h *Handler) ListSongs(w http.ResponseWriter, r *http.Request) {
	page := parseIntParam(r, "page", defaultPage)
	pageSize := parseIntParam(r, "page_size", defaultPageSize)

	// Clamp page and page_size to valid ranges
	if page < 1 {
		page = defaultPage
	}
	if pageSize < 1 || pageSize > maxPageSize {
		pageSize = defaultPageSize
	}

	songs, total, err := h.songStore.GetSongs(r.Context(), page, pageSize)
	if err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to retrieve songs",
		)
		return
	}

	writeJSON(w, http.StatusOK, dto.NewListResponse(songs, total, page, pageSize)) //nolint:errcheck
}

// GetSong handles GET /api/v1/songs/{id}.
//
// Returns a single song by its ID.
//
// Responses:
//
//	200 OK          — song found and returned
//	400 Bad Request — id is not a valid integer
//	404 Not Found   — no song with this id exists
//	500 Internal    — database failure
func (h *Handler) GetSong(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSongID(w, r)
	if !ok {
		return
	}

	song, err := h.songStore.GetSongByID(r.Context(), id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, dto.ErrCodeNotFound, "song not found")
			return
		}
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to retrieve song",
		)
		return
	}

	writeJSON(w, http.StatusOK, dto.NewSongResponse(song)) //nolint:errcheck
}

// DeleteSong handles DELETE /api/v1/songs/{id}.
//
// Removes a song and its fingerprint hashes from the library.
//
// Responses:
//
//	204 No Content  — song deleted successfully
//	400 Bad Request — id is not a valid integer
//	404 Not Found   — no song with this id exists
//	500 Internal    — database failure
func (h *Handler) DeleteSong(w http.ResponseWriter, r *http.Request) {
	id, ok := parseSongID(w, r)
	if !ok {
		return
	}

	ctx := r.Context()

	// Verify the song exists before attempting deletion
	_, err := h.songStore.GetSongByID(ctx, id)
	if err != nil {
		if isNotFound(err) {
			writeError(w, http.StatusNotFound, dto.ErrCodeNotFound, "song not found")
			return
		}
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to retrieve song",
		)
		return
	}

	// Delete fingerprint hashes first — maintain referential integrity
	if err := h.fingerprintStore.DeleteHashesBySongID(ctx, id); err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to delete fingerprint data",
		)
		return
	}

	// Delete the song record
	if err := h.songStore.DeleteSong(ctx, id); err != nil {
		writeError(w,
			http.StatusInternalServerError,
			dto.ErrCodeInternalError,
			"failed to delete song",
		)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// parseSongID extracts and validates the {id} URL parameter.
// Writes a 400 error and returns false if the id is invalid.
func parseSongID(w http.ResponseWriter, r *http.Request) (int, bool) {
	idStr := chi.URLParam(r, "id")
	id, err := strconv.Atoi(idStr)
	if err != nil || id < 1 {
		writeError(w,
			http.StatusBadRequest,
			dto.ErrCodeInvalidRequest,
			"id must be a positive integer",
		)
		return 0, false
	}
	return id, true
}

// parseIntParam reads a query parameter as an integer.
// Returns the defaultVal if the parameter is missing or invalid.
func parseIntParam(r *http.Request, name string, defaultVal int) int {
	val := r.URL.Query().Get(name)
	if val == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return n
}