package leadpush

import "context"

// PaginationMeta describes one page returned by the Leadpush API.
type PaginationMeta struct {
	CurrentPage int  `json:"current_page"`
	PerPage     int  `json:"per_page"`
	Total       int  `json:"total"`
	LastPage    int  `json:"last_page"`
	HasNext     bool `json:"has_next"`
}

// Page contains one page of API resources and its pagination metadata.
type Page[T any] struct {
	Data []T            `json:"data"`
	Meta PaginationMeta `json:"meta"`
}

// Pager retrieves consecutive pages from a list endpoint.
type Pager[T any] struct {
	fetch    func(context.Context, int) (*Page[T], error)
	nextPage int
	page     *Page[T]
	err      error
	done     bool
}

func newPager[T any](startPage int, fetch func(context.Context, int) (*Page[T], error)) *Pager[T] {
	if startPage <= 0 {
		startPage = 1
	}

	return &Pager[T]{fetch: fetch, nextPage: startPage}
}

// Next retrieves the next page. It returns false after the final page or when
// a request fails. Inspect Err after iteration.
func (p *Pager[T]) Next(ctx context.Context) bool {
	if p == nil || p.done || p.err != nil {
		return false
	}

	page, err := p.fetch(ctx, p.nextPage)
	if err != nil {
		p.err = err
		return false
	}

	p.page = page
	if !page.Meta.HasNext {
		p.done = true
		return true
	}

	next := page.Meta.CurrentPage + 1
	if next <= p.nextPage {
		p.err = &PaginationError{CurrentPage: page.Meta.CurrentPage, RequestedPage: p.nextPage}
		return true
	}

	p.nextPage = next
	return true
}

// Page returns the most recently retrieved page, or nil before the first
// successful call to Next.
func (p *Pager[T]) Page() *Page[T] {
	if p == nil {
		return nil
	}

	return p.page
}

// Err returns the first error encountered by the pager.
func (p *Pager[T]) Err() error {
	if p == nil {
		return nil
	}

	return p.err
}

// PaginationError indicates that a paginated response did not advance its
// current page and automatic pagination was stopped to prevent a loop.
type PaginationError struct {
	CurrentPage   int
	RequestedPage int
}

// Error implements error.
func (e *PaginationError) Error() string {
	return "leadpush: paginated response did not advance the current page"
}
