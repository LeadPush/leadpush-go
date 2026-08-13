package leadpush

import (
	"context"
	"time"
)

// SuppressionType identifies why an email address is suppressed.
type SuppressionType string

const (
	SuppressionTypeBounce    SuppressionType = "bounce"
	SuppressionTypeComplaint SuppressionType = "complaint"
	SuppressionTypeManual    SuppressionType = "manual"
)

// Suppression is returned by suppression endpoints.
type Suppression struct {
	UUID      string          `json:"uuid"`
	Email     string          `json:"email"`
	Type      SuppressionType `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
}

// SuppressionFilterID identifies a supported suppression list filter.
type SuppressionFilterID string

const (
	SuppressionFilterIDType SuppressionFilterID = "type"
)

// SuppressionFilter configures one API suppression-list filter.
type SuppressionFilter struct {
	ID    SuppressionFilterID `json:"id"`
	Value []SuppressionType   `json:"value"`
}

// SuppressionListParams configures a suppression list request.
type SuppressionListParams struct {
	Page    int
	PerPage int
	Search  string
	Filters []SuppressionFilter
}

// CreateSuppressionParams is accepted by SuppressionsService.Create.
type CreateSuppressionParams struct {
	Email string           `json:"email"`
	Type  *SuppressionType `json:"type,omitzero"`
}

// SuppressionsService provides suppression API operations.
type SuppressionsService struct {
	client *Client
}

// List retrieves one page of suppressions.
func (service *SuppressionsService) List(ctx context.Context, params *SuppressionListParams) (*Page[Suppression], error) {
	var response Page[Suppression]
	if err := service.client.Get(ctx, []string{"suppressions"}, suppressionListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all suppression pages.
func (service *SuppressionsService) NewPager(params *SuppressionListParams) *Pager[Suppression] {
	copied := cloneSuppressionListParams(params)
	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[Suppression], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Get retrieves a suppression by UUID.
func (service *SuppressionsService) Get(ctx context.Context, uuid string) (*Suppression, error) {
	var response resourceResponse[Suppression]
	if err := service.client.Get(ctx, []string{"suppressions", uuid}, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Create creates a suppression.
func (service *SuppressionsService) Create(ctx context.Context, params CreateSuppressionParams) (*Suppression, error) {
	var response resourceResponse[Suppression]
	if err := service.client.Post(ctx, []string{"suppressions"}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func suppressionListQuery(params *SuppressionListParams) map[string]any {
	if params == nil {
		return nil
	}
	query := make(map[string]any, 4)
	if params.Page != 0 {
		query["page"] = params.Page
	}
	if params.PerPage != 0 {
		query["per_page"] = params.PerPage
	}
	if params.Search != "" {
		query["search"] = params.Search
	}
	if params.Filters != nil {
		query["filters"] = params.Filters
	}
	return query
}

func cloneSuppressionListParams(params *SuppressionListParams) SuppressionListParams {
	if params == nil {
		return SuppressionListParams{}
	}
	copied := *params
	if params.Filters != nil {
		copied.Filters = make([]SuppressionFilter, len(params.Filters))
		for index, filter := range params.Filters {
			copied.Filters[index] = filter
			copied.Filters[index].Value = append([]SuppressionType(nil), filter.Value...)
		}
	}
	return copied
}
