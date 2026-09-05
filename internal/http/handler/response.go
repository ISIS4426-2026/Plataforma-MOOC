package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

const (
	HeaderIdempotencyKey = "Idempotency-Key"
	HeaderETag           = "ETag"
	HeaderIfNoneMatch    = "If-None-Match"
	HeaderIfMatch        = "If-Match"
)

// ErrorResponse defines the uniform error format across all API v1 endpoints.
type ErrorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// CursorPagination defines the reusable cursor-based pagination pattern metadata.
type CursorPagination struct {
	NextCursor *string `json:"next_cursor"`
	HasMore    bool    `json:"has_more"`
}

// PaginatedResponse represents a generic cursor-paginated list envelope.
type PaginatedResponse[T any] struct {
	Items      []T              `json:"items"`
	Pagination CursorPagination `json:"pagination"`
}

// RespondWithJSON writes a JSON response with status code and Content-Type header.
func RespondWithJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if payload != nil {
		_ = json.NewEncoder(w).Encode(payload)
	}
}

// RespondWithError writes a uniform ErrorResponse JSON body.
func RespondWithError(w http.ResponseWriter, status int, code string, message string, details any) {
	RespondWithJSON(w, status, ErrorResponse{
		Code:    code,
		Message: message,
		Details: details,
	})
}

// RespondWithETag handles ETag header setting and If-None-Match conditional GET handling.
// If the If-None-Match header matches the etag, it writes 304 Not Modified without body.
func RespondWithETag(w http.ResponseWriter, r *http.Request, etag string, payload any) {
	if etag != "" {
		w.Header().Set(HeaderETag, etag)
	}

	ifIfNoneMatch := r.Header.Get(HeaderIfNoneMatch)
	if ifIfNoneMatch != "" && etag != "" {
		// Clean quotes for comparison
		reqTag := strings.Trim(ifIfNoneMatch, `"`)
		resTag := strings.Trim(etag, `"`)
		if reqTag == resTag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
	}

	RespondWithJSON(w, http.StatusOK, payload)
}
