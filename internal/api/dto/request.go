package dto

import (
	"fmt"
	"mime/multipart"
	"net/http"
)

const (
	// MaxUploadSize is the maximum allowed size for audio file uploads.
	// 500MB covers the largest uncompressed WAV files a DJ would upload.
	MaxUploadSize = 500 << 20 // 500MB

	// MaxSnippetSize is the maximum allowed size for identification snippets.
	// 10MB is generous for a 10-15 second audio clip in any format.
	MaxSnippetSize = 10 << 20 // 10MB

	// uploadFileField is the multipart form field name for song uploads.
	uploadFileField = "file"

	// identifyFileField is the multipart form field name for snippets.
	identifyFileField = "file"

	// optionalTitleField is the multipart form field for track title.
	optionalTitleField = "title"

	// optionalArtistField is the multipart form field for artist name.
	optionalArtistField = "artist"

	// optionalAlbumField is the multipart form field for album name.
	optionalAlbumField = "album"
)

// UploadRequest holds the parsed contents of a POST /api/v1/upload
// multipart form request.
//
// The File and Header fields are populated from the multipart form.
// Title, Artist and Album are optional metadata — they may be empty
// strings when the user does not provide them.
type UploadRequest struct {
	// File is the opened multipart file ready for reading.
	// The caller must close it after use.
	File multipart.File

	// Header contains the original filename, size, and MIME type
	// of the uploaded file.
	Header *multipart.FileHeader

	// Optional metadata — all may be empty strings
	Title  string
	Artist string
	Album  string
}

// ParseUploadRequest parses a multipart/form-data POST request into
// an UploadRequest.
//
// Returns an error when:
//   - The request body exceeds MaxUploadSize
//   - The "file" field is missing from the form
//   - The file cannot be opened
//
// The caller is responsible for closing UploadRequest.File after use.
func ParseUploadRequest(r *http.Request) (*UploadRequest, error) {
	// Enforce maximum upload size before parsing the body.
	// This prevents large uploads from exhausting server memory.
	r.Body = http.MaxBytesReader(nil, r.Body, MaxUploadSize)

	if err := r.ParseMultipartForm(MaxUploadSize); err != nil {
		return nil, fmt.Errorf("request body too large or not multipart: %w", err)
	}

	file, header, err := r.FormFile(uploadFileField)
	if err != nil {
		return nil, fmt.Errorf("missing required field %q: %w", uploadFileField, err)
	}

	return &UploadRequest{
		File:   file,
		Header: header,
		Title:  r.FormValue(optionalTitleField),
		Artist: r.FormValue(optionalArtistField),
		Album:  r.FormValue(optionalAlbumField),
	}, nil
}

// IdentifyRequest holds the parsed contents of a POST /api/v1/identify
// multipart form request.
//
// Only the audio snippet file is required — no metadata fields.
type IdentifyRequest struct {
	// File is the opened multipart file ready for reading.
	// The caller must close it after use.
	File multipart.File

	// Header contains the original filename, size, and MIME type
	// of the uploaded snippet.
	Header *multipart.FileHeader
}

// ParseIdentifyRequest parses a multipart/form-data POST request into
// an IdentifyRequest.
//
// Returns an error when:
//   - The request body exceeds MaxSnippetSize
//   - The "file" field is missing from the form
//   - The file cannot be opened
//
// The caller is responsible for closing IdentifyRequest.File after use.
func ParseIdentifyRequest(r *http.Request) (*IdentifyRequest, error) {
	r.Body = http.MaxBytesReader(nil, r.Body, MaxSnippetSize)

	if err := r.ParseMultipartForm(MaxSnippetSize); err != nil {
		return nil, fmt.Errorf("request body too large or not multipart: %w", err)
	}

	file, header, err := r.FormFile(identifyFileField)
	if err != nil {
		return nil, fmt.Errorf("missing required field %q: %w", identifyFileField, err)
	}

	return &IdentifyRequest{
		File:   file,
		Header: header,
	}, nil
}

// HasTitle returns true when the upload request includes a non-empty title.
func (r *UploadRequest) HasTitle() bool {
	return r.Title != ""
}

// HasArtist returns true when the upload request includes a non-empty artist.
func (r *UploadRequest) HasArtist() bool {
	return r.Artist != ""
}

// HasAlbum returns true when the upload request includes a non-empty album.
func (r *UploadRequest) HasAlbum() bool {
	return r.Album != ""
}

// Filename returns the original filename from the uploaded file header.
func (r *UploadRequest) Filename() string {
	if r.Header == nil {
		return ""
	}
	return r.Header.Filename
}

// FileSize returns the size of the uploaded file in bytes.
func (r *UploadRequest) FileSize() int64 {
	if r.Header == nil {
		return 0
	}
	return r.Header.Size
}

// Filename returns the original filename of the snippet.
func (r *IdentifyRequest) Filename() string {
	if r.Header == nil {
		return ""
	}
	return r.Header.Filename
}