package leadpush

import (
	"context"
	"encoding/json"
	"time"
)

// Attributes contains custom contact field values.
type Attributes map[string]any

// ContactEventAttributes contains arbitrary event metadata.
type ContactEventAttributes map[string]any

// Contact is returned by contact endpoints.
type Contact struct {
	UUID       string     `json:"uuid"`
	Subscribed bool       `json:"subscribed"`
	Attributes Attributes `json:"attributes"`
	Provider   *string    `json:"provider"`
	UpdatedAt  time.Time  `json:"updated_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// ContactEvent is returned by contact event list endpoints.
type ContactEvent struct {
	UUID       string                  `json:"uuid"`
	EventName  string                  `json:"event_name"`
	Attributes *ContactEventAttributes `json:"attributes"`
	CreatedAt  time.Time               `json:"created_at"`
}

// ContactListParams configures a contact list request.
type ContactListParams struct {
	Page    int
	PerPage int
}

// CreateContactParams is accepted by ContactsService.Create.
type CreateContactParams struct {
	Attributes Attributes `json:"attributes"`
	Subscribed *bool      `json:"subscribed,omitzero"`
}

// UpdateContactParams is accepted by ContactsService.Update.
type UpdateContactParams struct {
	Attributes Attributes `json:"attributes,omitzero"`
	Subscribed *bool      `json:"subscribed,omitzero"`
}

// ContactEventListParams configures a contact event list request.
type ContactEventListParams struct {
	Page    int
	PerPage int
	Search  string
}

// CreateContactEventParams is accepted by ContactEventsService.Create.
type CreateContactEventParams struct {
	EventName  string
	Attributes ContactEventAttributes
}

// ContactsService provides contact API operations.
type ContactsService struct {
	client *Client
}

// List retrieves one page of contacts.
func (service *ContactsService) List(ctx context.Context, params *ContactListParams) (*Page[Contact], error) {
	var response Page[Contact]
	if err := service.client.Get(ctx, []string{"contacts"}, contactListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all contact pages.
func (service *ContactsService) NewPager(params *ContactListParams) *Pager[Contact] {
	copied := ContactListParams{}
	if params != nil {
		copied = *params
	}

	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[Contact], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Get retrieves a contact by UUID or workspace identity value.
func (service *ContactsService) Get(ctx context.Context, identifier string) (*Contact, error) {
	var response resourceResponse[Contact]
	if err := service.client.Get(ctx, []string{"contacts", identifier}, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Create creates a contact.
func (service *ContactsService) Create(ctx context.Context, params CreateContactParams) (*Contact, error) {
	var response resourceResponse[Contact]
	if err := service.client.Post(ctx, []string{"contacts"}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Update updates a contact by UUID or workspace identity value.
func (service *ContactsService) Update(ctx context.Context, identifier string, params UpdateContactParams) (*Contact, error) {
	var response resourceResponse[Contact]
	if err := service.client.Post(ctx, []string{"contacts", identifier}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Subscribe subscribes a contact by UUID or workspace identity value.
func (service *ContactsService) Subscribe(ctx context.Context, identifier string) (*Contact, error) {
	return service.subscriptionAction(ctx, identifier, "subscribe")
}

// Unsubscribe unsubscribes a contact by UUID or workspace identity value.
func (service *ContactsService) Unsubscribe(ctx context.Context, identifier string) (*Contact, error) {
	return service.subscriptionAction(ctx, identifier, "unsubscribe")
}

// Events returns contact event operations for a UUID or workspace identity
// value.
func (service *ContactsService) Events(identifier string) *ContactEventsService {
	return &ContactEventsService{client: service.client, contactIdentifier: identifier}
}

func (service *ContactsService) subscriptionAction(ctx context.Context, identifier, action string) (*Contact, error) {
	var response resourceResponse[Contact]
	if err := service.client.Post(ctx, []string{"contacts", identifier, action}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// ContactEventsService provides nested contact event API operations.
type ContactEventsService struct {
	client            *Client
	contactIdentifier string
}

// List retrieves one page of contact events.
func (service *ContactEventsService) List(ctx context.Context, params *ContactEventListParams) (*Page[ContactEvent], error) {
	var response Page[ContactEvent]
	path := []string{"contacts", service.contactIdentifier, "events"}
	if err := service.client.Get(ctx, path, contactEventListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all contact event pages.
func (service *ContactEventsService) NewPager(params *ContactEventListParams) *Pager[ContactEvent] {
	copied := ContactEventListParams{}
	if params != nil {
		copied = *params
	}

	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[ContactEvent], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Create creates a contact event. The API does not return the created event.
func (service *ContactEventsService) Create(ctx context.Context, params CreateContactEventParams) error {
	request := struct {
		EventName  string  `json:"event_name"`
		Attributes *string `json:"attributes,omitzero"`
	}{EventName: params.EventName}

	if params.Attributes != nil {
		encoded, err := json.Marshal(params.Attributes)
		if err != nil {
			return err
		}
		serialized := string(encoded)
		request.Attributes = &serialized
	}

	path := []string{"contacts", service.contactIdentifier, "events"}
	return service.client.Post(ctx, path, request, nil, nil)
}

func contactListQuery(params *ContactListParams) map[string]any {
	if params == nil {
		return nil
	}
	query := make(map[string]any, 2)
	if params.Page != 0 {
		query["page"] = params.Page
	}
	if params.PerPage != 0 {
		query["per_page"] = params.PerPage
	}
	return query
}

func contactEventListQuery(params *ContactEventListParams) map[string]any {
	if params == nil {
		return nil
	}
	query := make(map[string]any, 3)
	if params.Page != 0 {
		query["page"] = params.Page
	}
	if params.PerPage != 0 {
		query["per_page"] = params.PerPage
	}
	if params.Search != "" {
		query["search"] = params.Search
	}
	return query
}
