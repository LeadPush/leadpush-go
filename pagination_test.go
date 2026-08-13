package leadpush

import (
	"context"
	"errors"
	"net/http"
	"testing"
)

func TestPagerHonorsStartingPageAndStops(t *testing.T) {
	expectations := []expectedRequest{
		{method: http.MethodGet, path: "/v1/contacts", query: map[string]string{"page": "2", "per_page": "1"}, response: pageJSON(contactJSON, 2, true)},
		{method: http.MethodGet, path: "/v1/contacts", query: map[string]string{"page": "3", "per_page": "1"}, response: pageJSON(contactJSON, 3, false)},
	}
	client := expectationClient(t, expectations)
	params := &ContactListParams{Page: 2, PerPage: 1}
	pager := client.Contacts.NewPager(params)
	params.Page = 99

	var pages []int
	for pager.Next(context.Background()) {
		pages = append(pages, pager.Page().Meta.CurrentPage)
	}
	if err := pager.Err(); err != nil {
		t.Fatalf("Pager.Err() = %v", err)
	}
	if len(pages) != 2 || pages[0] != 2 || pages[1] != 3 {
		t.Fatalf("pages = %v", pages)
	}
	if pager.Next(context.Background()) {
		t.Fatal("Pager.Next() returned true after final page")
	}
}

func TestPagerPreservesMidstreamError(t *testing.T) {
	expectations := []expectedRequest{
		{method: http.MethodGet, path: "/v1/domains", query: map[string]string{"page": "1"}, response: pageJSON(domainJSON, 1, true)},
		{method: http.MethodGet, path: "/v1/domains", query: map[string]string{"page": "2"}, status: http.StatusInternalServerError, response: `{"message":"failed"}`},
	}
	pager := expectationClient(t, expectations).Domains.NewPager(nil)
	if !pager.Next(context.Background()) {
		t.Fatalf("first Next() = false, error = %v", pager.Err())
	}
	if pager.Next(context.Background()) {
		t.Fatal("second Next() = true, want false")
	}
	var apiError *APIError
	if !errors.As(pager.Err(), &apiError) || apiError.StatusCode != http.StatusInternalServerError {
		t.Fatalf("Pager.Err() = %#v", pager.Err())
	}
}

func TestPagerStopsNonAdvancingResponses(t *testing.T) {
	t.Parallel()

	pager := newPager(2, func(context.Context, int) (*Page[int], error) {
		return &Page[int]{Data: []int{1}, Meta: PaginationMeta{CurrentPage: 1, HasNext: true}}, nil
	})
	if page := pager.Page(); page != nil {
		t.Fatalf("Page() before Next() = %#v", page)
	}
	if !pager.Next(context.Background()) {
		t.Fatalf("Next() = false, error = %v", pager.Err())
	}
	var paginationError *PaginationError
	if !errors.As(pager.Err(), &paginationError) {
		t.Fatalf("Pager.Err() = %#v, want PaginationError", pager.Err())
	}
	if pager.Next(context.Background()) {
		t.Fatal("Next() continued after PaginationError")
	}
}
