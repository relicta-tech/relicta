package handlers

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/relicta-tech/relicta/internal/httpserver/dto"
)

const (
	// defaultLimit is the default page size for cursor pagination.
	defaultLimit = 20
	// maxLimit is the maximum allowed page size.
	maxLimit = 100
	// cursorPrefix is the prefix for cursor tokens to identify the format.
	cursorPrefix = "idx:"
)

// PaginationParams holds parsed pagination parameters from a request.
type PaginationParams struct {
	Limit  int
	Cursor string
	Offset int // decoded offset from cursor, or 0
}

// ParsePagination extracts cursor-based pagination params from the request.
// Supports: ?limit=N&cursor=<opaque> for cursor mode,
// and falls back to ?page=N&page_size=N for offset mode.
func ParsePagination(r *http.Request) PaginationParams {
	q := r.URL.Query()

	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > maxLimit {
		// Fall back to page_size for backward compatibility
		limit, _ = strconv.Atoi(q.Get("page_size"))
		if limit <= 0 || limit > maxLimit {
			limit = defaultLimit
		}
	}

	cursor := q.Get("cursor")
	offset := 0

	if cursor != "" {
		offset = decodeCursor(cursor)
	} else {
		// Fall back to page-based offset
		page, _ := strconv.Atoi(q.Get("page"))
		if page > 1 {
			offset = (page - 1) * limit
		}
	}

	return PaginationParams{
		Limit:  limit,
		Cursor: cursor,
		Offset: offset,
	}
}

// Paginate applies cursor-based pagination to a slice and returns
// the paginated response with Link headers set on the writer.
func Paginate[T any](items []T, params PaginationParams, r *http.Request, w http.ResponseWriter) dto.CursorPaginatedResponse[T] {
	total := len(items)

	start := params.Offset
	if start > total {
		start = total
	}
	end := start + params.Limit
	if end > total {
		end = total
	}

	page := items[start:end]
	hasMore := end < total

	var nextCursor, prevCursor string
	if hasMore {
		nextCursor = encodeCursor(end)
	}
	if start > 0 {
		prev := start - params.Limit
		if prev < 0 {
			prev = 0
		}
		prevCursor = encodeCursor(prev)
	}

	// Set Link headers for navigation
	setLinkHeaders(w, r, params.Limit, nextCursor, prevCursor)

	return dto.CursorPaginatedResponse[T]{
		Data:       page,
		Total:      total,
		Limit:      params.Limit,
		NextCursor: nextCursor,
		PrevCursor: prevCursor,
		HasMore:    hasMore,
	}
}

// encodeCursor encodes an offset into an opaque cursor string.
func encodeCursor(offset int) string {
	raw := fmt.Sprintf("%s%d", cursorPrefix, offset)
	return base64.RawURLEncoding.EncodeToString([]byte(raw))
}

// decodeCursor decodes a cursor string back to an offset.
// Returns 0 for invalid cursors.
func decodeCursor(cursor string) int {
	decoded, err := base64.RawURLEncoding.DecodeString(cursor)
	if err != nil {
		return 0
	}
	s := string(decoded)
	if !strings.HasPrefix(s, cursorPrefix) {
		return 0
	}
	offset, err := strconv.Atoi(s[len(cursorPrefix):])
	if err != nil || offset < 0 {
		return 0
	}
	return offset
}

// setLinkHeaders sets RFC 8288 Link headers for pagination navigation.
func setLinkHeaders(w http.ResponseWriter, r *http.Request, limit int, nextCursor, prevCursor string) {
	var links []string
	basePath := r.URL.Path

	if nextCursor != "" {
		links = append(links, fmt.Sprintf(`<%s?limit=%d&cursor=%s>; rel="next"`, basePath, limit, nextCursor))
	}
	if prevCursor != "" {
		links = append(links, fmt.Sprintf(`<%s?limit=%d&cursor=%s>; rel="prev"`, basePath, limit, prevCursor))
	}

	if len(links) > 0 {
		w.Header().Set("Link", strings.Join(links, ", "))
	}
}
