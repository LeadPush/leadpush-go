package leadpush

import (
	"context"
	"time"
)

// FieldType is a custom contact field data type.
type FieldType string

const (
	FieldTypeInteger  FieldType = "integer"
	FieldTypeText     FieldType = "text"
	FieldTypeDate     FieldType = "date"
	FieldTypeDateTime FieldType = "datetime"
	FieldTypeBoolean  FieldType = "boolean"
)

// FieldTextFormat configures validation for text fields.
type FieldTextFormat string

const (
	FieldTextFormatEmail FieldTextFormat = "email"
	FieldTextFormatPhone FieldTextFormat = "phone"
	FieldTextFormatUUID  FieldTextFormat = "uuid"
	FieldTextFormatURL   FieldTextFormat = "url"
	FieldTextFormatRegex FieldTextFormat = "regex"
)

// FieldFormat describes optional field format settings.
type FieldFormat struct {
	Text      *FieldTextFormat `json:"text,omitzero"`
	Pattern   *string          `json:"pattern,omitzero"`
	ISOFormat *string          `json:"iso_format,omitzero"`
}

// Field is returned by custom field endpoints.
type Field struct {
	UUID      string       `json:"uuid"`
	Name      string       `json:"name"`
	Type      FieldType    `json:"type"`
	Format    *FieldFormat `json:"format"`
	CreatedAt time.Time    `json:"created_at"`
}

// FieldFilterID identifies a supported field list filter.
type FieldFilterID string

const (
	FieldFilterIDType FieldFilterID = "type"
)

// FieldFilter configures one API field-list filter.
type FieldFilter struct {
	ID    FieldFilterID `json:"id"`
	Value []FieldType   `json:"value"`
}

// FieldListParams configures a field list request.
type FieldListParams struct {
	Page    int
	PerPage int
	Search  string
	Filters []FieldFilter
}

// CreateFieldParams is accepted by FieldsService.Create.
type CreateFieldParams struct {
	Name   string                `json:"name"`
	Type   FieldType             `json:"type"`
	Format Optional[FieldFormat] `json:"format,omitzero"`
}

// UpdateFieldParams is accepted by FieldsService.Update.
type UpdateFieldParams struct {
	Name   *string               `json:"name,omitzero"`
	Type   *FieldType            `json:"type,omitzero"`
	Format Optional[FieldFormat] `json:"format,omitzero"`
}

// FieldsService provides custom field API operations.
type FieldsService struct {
	client *Client
}

// List retrieves one page of custom fields.
func (service *FieldsService) List(ctx context.Context, params *FieldListParams) (*Page[Field], error) {
	var response Page[Field]
	if err := service.client.Get(ctx, []string{"fields"}, fieldListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all custom field pages.
func (service *FieldsService) NewPager(params *FieldListParams) *Pager[Field] {
	copied := cloneFieldListParams(params)
	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[Field], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Get retrieves a custom field by UUID.
func (service *FieldsService) Get(ctx context.Context, uuid string) (*Field, error) {
	var response resourceResponse[Field]
	if err := service.client.Get(ctx, []string{"fields", uuid}, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Create creates a custom field.
func (service *FieldsService) Create(ctx context.Context, params CreateFieldParams) (*Field, error) {
	var response resourceResponse[Field]
	if err := service.client.Post(ctx, []string{"fields"}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Update updates a custom field by UUID.
func (service *FieldsService) Update(ctx context.Context, uuid string, params UpdateFieldParams) (*Field, error) {
	var response resourceResponse[Field]
	if err := service.client.Post(ctx, []string{"fields", uuid}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

func fieldListQuery(params *FieldListParams) map[string]any {
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

func cloneFieldListParams(params *FieldListParams) FieldListParams {
	if params == nil {
		return FieldListParams{}
	}
	copied := *params
	if params.Filters != nil {
		copied.Filters = make([]FieldFilter, len(params.Filters))
		for index, filter := range params.Filters {
			copied.Filters[index] = filter
			copied.Filters[index].Value = append([]FieldType(nil), filter.Value...)
		}
	}
	return copied
}
