package leadpush

import (
	"context"
	"time"
)

// DomainProvider identifies the provider backing a domain or address.
type DomainProvider string

const (
	DomainProviderAWS      DomainProvider = "aws"
	DomainProviderLeadpush DomainProvider = "leadpush"
)

// DomainStatus describes the lifecycle status of a domain.
type DomainStatus string

const (
	DomainStatusPending DomainStatus = "pending"
)

// DomainVerification describes domain or address verification status.
type DomainVerification string

const (
	DomainVerificationPending   DomainVerification = "pending"
	DomainVerificationCompleted DomainVerification = "completed"
	DomainVerificationFailed    DomainVerification = "failed"
)

// DomainTrackingMode configures tracking DNS behavior.
type DomainTrackingMode string

const (
	DomainTrackingModeDirect     DomainTrackingMode = "direct"
	DomainTrackingModeCloudflare DomainTrackingMode = "cloudflare"
)

// DomainDNSRecord describes a DNS record required for domain verification.
type DomainDNSRecord struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsValid bool   `json:"is_valid"`
}

// Domain is returned by domain endpoints.
type Domain struct {
	UUID             string             `json:"uuid"`
	Name             string             `json:"name"`
	Domain           string             `json:"domain"`
	Verified         bool               `json:"verified"`
	Provider         DomainProvider     `json:"provider"`
	Status           DomainStatus       `json:"status"`
	Verification     DomainVerification `json:"verification"`
	MailFromDomain   string             `json:"mail_from_domain"`
	MailFromVerified bool               `json:"mail_from_verified"`
	DNS              []DomainDNSRecord  `json:"dns"`
	UpdatedAt        time.Time          `json:"updated_at"`
	CreatedAt        time.Time          `json:"created_at"`
}

// DomainAddress is returned by nested domain address endpoints.
type DomainAddress struct {
	UUID         string             `json:"uuid"`
	DomainUUID   string             `json:"domain_uuid"`
	Address      string             `json:"address"`
	FullAddress  string             `json:"full_address"`
	Provider     *DomainProvider    `json:"provider"`
	DisplayName  string             `json:"display_name"`
	Verification DomainVerification `json:"verification"`
	UpdatedAt    time.Time          `json:"updated_at"`
	CreatedAt    time.Time          `json:"created_at"`
}

// DomainListParams configures a domain list request.
type DomainListParams struct {
	Page    int
	PerPage int
	Search  string
}

// CreateDomainParams is accepted by DomainsService.Create.
type CreateDomainParams struct {
	Name              string                       `json:"name"`
	DKIMSelectors     Optional[[]string]           `json:"dkim_selectors,omitzero"`
	TrackingSubdomain Optional[string]             `json:"tracking_subdomain,omitzero"`
	TrackingMode      Optional[DomainTrackingMode] `json:"tracking_mode,omitzero"`
}

// DomainAddressListParams configures a domain address list request.
type DomainAddressListParams struct {
	Page    int
	PerPage int
}

// CreateDomainAddressParams is accepted by DomainAddressesService.Create.
type CreateDomainAddressParams struct {
	Address         string           `json:"address"`
	DisplayName     string           `json:"display_name"`
	ReplyTo         string           `json:"reply_to"`
	CompanyAddress  string           `json:"company_address"`
	CompanyAddress2 Optional[string] `json:"company_address_2,omitzero"`
	CompanyCity     string           `json:"company_city"`
	CompanyState    string           `json:"company_state"`
	CompanyZIP      string           `json:"company_zip"`
	CompanyCountry  string           `json:"company_country"`
}

// DomainsService provides domain API operations.
type DomainsService struct {
	client *Client
}

// List retrieves one page of domains.
func (service *DomainsService) List(ctx context.Context, params *DomainListParams) (*Page[Domain], error) {
	var response Page[Domain]
	if err := service.client.Get(ctx, []string{"domains"}, domainListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all domain pages.
func (service *DomainsService) NewPager(params *DomainListParams) *Pager[Domain] {
	copied := DomainListParams{}
	if params != nil {
		copied = *params
	}
	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[Domain], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Get retrieves a domain by UUID.
func (service *DomainsService) Get(ctx context.Context, uuid string) (*Domain, error) {
	var response resourceResponse[Domain]
	if err := service.client.Get(ctx, []string{"domains", uuid}, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Create creates a domain.
func (service *DomainsService) Create(ctx context.Context, params CreateDomainParams) (*Domain, error) {
	var response resourceResponse[Domain]
	if err := service.client.Post(ctx, []string{"domains"}, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Verify refreshes domain verification status.
func (service *DomainsService) Verify(ctx context.Context, uuid string) (*Domain, error) {
	var response resourceResponse[Domain]
	if err := service.client.Post(ctx, []string{"domains", uuid, "verification"}, nil, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Delete deletes a domain by UUID.
func (service *DomainsService) Delete(ctx context.Context, uuid string) error {
	return service.client.Delete(ctx, []string{"domains", uuid}, nil, nil)
}

// Addresses returns address operations for a domain UUID.
func (service *DomainsService) Addresses(uuid string) *DomainAddressesService {
	return &DomainAddressesService{client: service.client, domainUUID: uuid}
}

// DomainAddressesService provides nested domain address API operations.
type DomainAddressesService struct {
	client     *Client
	domainUUID string
}

// List retrieves one page of domain addresses.
func (service *DomainAddressesService) List(ctx context.Context, params *DomainAddressListParams) (*Page[DomainAddress], error) {
	var response Page[DomainAddress]
	path := []string{"domains", service.domainUUID, "addresses"}
	if err := service.client.Get(ctx, path, domainAddressListQuery(params), &response); err != nil {
		return nil, err
	}
	return &response, nil
}

// NewPager returns a pager that retrieves all domain address pages.
func (service *DomainAddressesService) NewPager(params *DomainAddressListParams) *Pager[DomainAddress] {
	copied := DomainAddressListParams{}
	if params != nil {
		copied = *params
	}
	return newPager(copied.Page, func(ctx context.Context, page int) (*Page[DomainAddress], error) {
		request := copied
		request.Page = page
		return service.List(ctx, &request)
	})
}

// Get retrieves a domain address by UUID.
func (service *DomainAddressesService) Get(ctx context.Context, uuid string) (*DomainAddress, error) {
	var response resourceResponse[DomainAddress]
	path := []string{"domains", service.domainUUID, "addresses", uuid}
	if err := service.client.Get(ctx, path, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Create creates a domain address.
func (service *DomainAddressesService) Create(ctx context.Context, params CreateDomainAddressParams) (*DomainAddress, error) {
	var response resourceResponse[DomainAddress]
	path := []string{"domains", service.domainUUID, "addresses"}
	if err := service.client.Post(ctx, path, params, nil, &response); err != nil {
		return nil, err
	}
	return &response.Data, nil
}

// Delete deletes a domain address by UUID.
func (service *DomainAddressesService) Delete(ctx context.Context, uuid string) error {
	path := []string{"domains", service.domainUUID, "addresses", uuid}
	return service.client.Delete(ctx, path, nil, nil)
}

func domainListQuery(params *DomainListParams) map[string]any {
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

func domainAddressListQuery(params *DomainAddressListParams) map[string]any {
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
