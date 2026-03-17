package handlers

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParsePagination_Defaults(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	params := ParsePagination(req)

	assert.Equal(t, defaultLimit, params.Limit)
	assert.Equal(t, 0, params.Offset)
	assert.Empty(t, params.Cursor)
}

func TestParsePagination_CursorMode(t *testing.T) {
	cursor := encodeCursor(20)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?limit=10&cursor="+cursor, nil)
	params := ParsePagination(req)

	assert.Equal(t, 10, params.Limit)
	assert.Equal(t, 20, params.Offset)
	assert.Equal(t, cursor, params.Cursor)
}

func TestParsePagination_PageFallback(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?page=3&page_size=10", nil)
	params := ParsePagination(req)

	assert.Equal(t, 10, params.Limit)
	assert.Equal(t, 20, params.Offset) // (3-1) * 10 = 20
}

func TestParsePagination_LimitClamped(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?limit=500", nil)
	params := ParsePagination(req)

	assert.Equal(t, defaultLimit, params.Limit) // falls back to default
}

func TestParsePagination_InvalidCursor(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases?cursor=garbage", nil)
	params := ParsePagination(req)

	assert.Equal(t, 0, params.Offset)
}

func TestEncodeDecode_Roundtrip(t *testing.T) {
	tests := []int{0, 1, 20, 100, 999}
	for _, offset := range tests {
		cursor := encodeCursor(offset)
		assert.Equal(t, offset, decodeCursor(cursor))
	}
}

func TestDecodeCursor_Invalid(t *testing.T) {
	tests := []struct {
		name   string
		cursor string
	}{
		{"empty", ""},
		{"not base64", "!!!invalid!!!"},
		{"wrong prefix", "aW52YWxpZA"}, // base64("invalid")
		{"negative offset", encodeCursor(-1)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, 0, decodeCursor(tt.cursor))
		})
	}
}

func TestPaginate_FirstPage(t *testing.T) {
	items := makeTestItems(50)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	params := PaginationParams{Limit: 20, Offset: 0}
	resp := Paginate(items, params, req, rec)

	assert.Len(t, resp.Data, 20)
	assert.Equal(t, 50, resp.Total)
	assert.Equal(t, 20, resp.Limit)
	assert.True(t, resp.HasMore)
	assert.NotEmpty(t, resp.NextCursor)
	assert.Empty(t, resp.PrevCursor)

	// Link header should contain "next" but not "prev"
	link := rec.Header().Get("Link")
	assert.Contains(t, link, `rel="next"`)
	assert.NotContains(t, link, `rel="prev"`)
}

func TestPaginate_MiddlePage(t *testing.T) {
	items := makeTestItems(50)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	params := PaginationParams{Limit: 20, Offset: 20}
	resp := Paginate(items, params, req, rec)

	assert.Len(t, resp.Data, 20)
	assert.True(t, resp.HasMore)
	assert.NotEmpty(t, resp.NextCursor)
	assert.NotEmpty(t, resp.PrevCursor)

	// Link header should contain both "next" and "prev"
	link := rec.Header().Get("Link")
	assert.Contains(t, link, `rel="next"`)
	assert.Contains(t, link, `rel="prev"`)
}

func TestPaginate_LastPage(t *testing.T) {
	items := makeTestItems(50)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	params := PaginationParams{Limit: 20, Offset: 40}
	resp := Paginate(items, params, req, rec)

	assert.Len(t, resp.Data, 10) // only 10 remaining
	assert.False(t, resp.HasMore)
	assert.Empty(t, resp.NextCursor)
	assert.NotEmpty(t, resp.PrevCursor)
}

func TestPaginate_EmptySlice(t *testing.T) {
	var items []string
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	params := PaginationParams{Limit: 20, Offset: 0}
	resp := Paginate(items, params, req, rec)

	assert.Empty(t, resp.Data)
	assert.Equal(t, 0, resp.Total)
	assert.False(t, resp.HasMore)
}

func TestPaginate_OffsetBeyondEnd(t *testing.T) {
	items := makeTestItems(5)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/releases", nil)
	rec := httptest.NewRecorder()

	params := PaginationParams{Limit: 20, Offset: 100}
	resp := Paginate(items, params, req, rec)

	assert.Empty(t, resp.Data)
	assert.False(t, resp.HasMore)
}

func TestPaginate_CursorNavigationRoundtrip(t *testing.T) {
	items := makeTestItems(50)

	// Page 1
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/releases?limit=20", nil)
	rec1 := httptest.NewRecorder()
	resp1 := Paginate(items, PaginationParams{Limit: 20, Offset: 0}, req1, rec1)
	require.True(t, resp1.HasMore)

	// Page 2 using cursor from page 1
	offset2 := decodeCursor(resp1.NextCursor)
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/releases?limit=20&cursor="+resp1.NextCursor, nil)
	rec2 := httptest.NewRecorder()
	resp2 := Paginate(items, PaginationParams{Limit: 20, Offset: offset2}, req2, rec2)
	require.True(t, resp2.HasMore)

	// Page 3 using cursor from page 2
	offset3 := decodeCursor(resp2.NextCursor)
	req3 := httptest.NewRequest(http.MethodGet, "/api/v1/releases?limit=20&cursor="+resp2.NextCursor, nil)
	rec3 := httptest.NewRecorder()
	resp3 := Paginate(items, PaginationParams{Limit: 20, Offset: offset3}, req3, rec3)
	assert.False(t, resp3.HasMore)
	assert.Len(t, resp3.Data, 10)

	// Navigate back using prev cursor
	prevOffset := decodeCursor(resp3.PrevCursor)
	assert.Equal(t, 20, prevOffset) // should go back to page 2 start
}

func makeTestItems(n int) []string {
	items := make([]string, n)
	for i := range items {
		items[i] = "item-" + string(rune('A'+i%26))
	}
	return items
}
