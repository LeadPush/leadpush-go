package leadpush

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
)

const (
	contactJSON     = `{"uuid":"contact-uuid","subscribed":true,"attributes":{"email":"person@example.test","first_name":"Person"},"provider":"leadpush","created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-02T12:00:00Z","future_field":"ignored"}`
	eventJSON       = `{"uuid":"event-uuid","event_name":"purchase","attributes":{"plan":"enterprise"},"created_at":"2026-01-03T12:00:00Z"}`
	domainJSON      = `{"uuid":"domain-uuid","name":"example.test","domain":"example.test","verified":false,"provider":"leadpush","status":"pending","verification":"pending","mail_from_domain":"mail.example.test","mail_from_verified":false,"dns":[{"type":"TXT","name":"example.test","value":"value","is_valid":false}],"created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-02T12:00:00Z"}`
	addressJSON     = `{"uuid":"address-uuid","domain_uuid":"domain-uuid","address":"sender","full_address":"sender@example.test","provider":"leadpush","display_name":"Sender","verification":"completed","created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-02T12:00:00Z"}`
	fieldJSON       = `{"uuid":"field-uuid","name":"company","type":"text","format":{"text":"url","pattern":null,"iso_format":null},"created_at":"2026-01-01T12:00:00Z"}`
	suppressionJSON = `{"uuid":"suppression-uuid","email":"blocked@example.test","type":"manual","created_at":"2026-01-01T12:00:00Z"}`
	emailSendJSON   = `{"accepted":true,"message_count":2,"messages":[{"uuid":"message-1","recipient":"person@example.test","type":"to","from":"sender@example.test","status":"pending"},{"uuid":"message-2","recipient":"audit@example.test","type":"bcc","from":"sender@example.test","status":"pending"}]}`
)

type expectedRequest struct {
	method   string
	path     string
	query    map[string]string
	body     string
	status   int
	response string
}

func TestContactsAndContactEvents(t *testing.T) {
	expectations := []expectedRequest{
		{method: http.MethodGet, path: "/v1/contacts", query: map[string]string{"page": "1", "per_page": "10"}, response: pageJSON(contactJSON, 1, false)},
		{method: http.MethodGet, path: "/v1/contacts/person%2Fname%40example.test", response: resourceJSON(contactJSON)},
		{method: http.MethodPost, path: "/v1/contacts", body: `{"attributes":{"email":"person@example.test"},"subscribed":false}`, response: resourceJSON(contactJSON)},
		{method: http.MethodPost, path: "/v1/contacts/person%40example.test", body: `{"attributes":{"first_name":"Updated"},"subscribed":false}`, response: resourceJSON(contactJSON)},
		{method: http.MethodPost, path: "/v1/contacts/person%40example.test/subscribe", response: resourceJSON(contactJSON)},
		{method: http.MethodPost, path: "/v1/contacts/person%40example.test/unsubscribe", response: resourceJSON(contactJSON)},
		{method: http.MethodGet, path: "/v1/contacts/person%40example.test/events", query: map[string]string{"page": "2", "per_page": "5", "search": "purchase"}, response: pageJSON(eventJSON, 2, false)},
		{method: http.MethodPost, path: "/v1/contacts/person%40example.test/events", body: `{"event_name":"purchase","attributes":"{\"plan\":\"enterprise\"}"}`, status: http.StatusNoContent},
		{method: http.MethodPost, path: "/v1/contacts/person%40example.test/events", body: `{"event_name":"login"}`, status: http.StatusNoContent},
	}
	client := expectationClient(t, expectations)
	ctx := context.Background()

	page, err := client.Contacts.List(ctx, &ContactListParams{Page: 1, PerPage: 10})
	if err != nil || len(page.Data) != 1 || page.Data[0].UUID != "contact-uuid" {
		t.Fatalf("Contacts.List() = %#v, %v", page, err)
	}
	if page.Data[0].CreatedAt.Year() != 2026 || page.Data[0].Provider == nil || *page.Data[0].Provider != "leadpush" {
		t.Fatalf("contact model = %#v", page.Data[0])
	}

	contact, err := client.Contacts.Get(ctx, "person/name@example.test")
	if err != nil || contact.Attributes["email"] != "person@example.test" {
		t.Fatalf("Contacts.Get() = %#v, %v", contact, err)
	}
	contact, err = client.Contacts.Create(ctx, CreateContactParams{
		Attributes: Attributes{"email": "person@example.test"},
		Subscribed: Ptr(false),
	})
	if err != nil || contact.UUID != "contact-uuid" {
		t.Fatalf("Contacts.Create() = %#v, %v", contact, err)
	}
	contact, err = client.Contacts.Update(ctx, "person@example.test", UpdateContactParams{
		Attributes: Attributes{"first_name": "Updated"},
		Subscribed: Ptr(false),
	})
	if err != nil || contact.UUID != "contact-uuid" {
		t.Fatalf("Contacts.Update() = %#v, %v", contact, err)
	}
	if _, err = client.Contacts.Subscribe(ctx, "person@example.test"); err != nil {
		t.Fatalf("Contacts.Subscribe() error = %v", err)
	}
	if _, err = client.Contacts.Unsubscribe(ctx, "person@example.test"); err != nil {
		t.Fatalf("Contacts.Unsubscribe() error = %v", err)
	}

	events := client.Contacts.Events("person@example.test")
	eventPage, err := events.List(ctx, &ContactEventListParams{Page: 2, PerPage: 5, Search: "purchase"})
	if err != nil || len(eventPage.Data) != 1 || eventPage.Data[0].EventName != "purchase" || eventPage.Data[0].CreatedAt.Year() != 2026 {
		t.Fatalf("ContactEvents.List() = %#v, %v", eventPage, err)
	}
	if err := events.Create(ctx, CreateContactEventParams{EventName: "purchase", Attributes: ContactEventAttributes{"plan": "enterprise"}}); err != nil {
		t.Fatalf("ContactEvents.Create(attributes) error = %v", err)
	}
	if err := events.Create(ctx, CreateContactEventParams{EventName: "login"}); err != nil {
		t.Fatalf("ContactEvents.Create(no attributes) error = %v", err)
	}
}

func TestDomainsAndDomainAddresses(t *testing.T) {
	verifiedDomain := `{"uuid":"domain-uuid","name":"example.test","domain":"example.test","verified":true,"provider":"leadpush","status":"pending","verification":"completed","mail_from_domain":"mail.example.test","mail_from_verified":true,"dns":[],"created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-02T12:00:00Z"}`
	expectations := []expectedRequest{
		{method: http.MethodGet, path: "/v1/domains", query: map[string]string{"page": "1", "per_page": "10", "search": "example"}, response: pageJSON(domainJSON, 1, false)},
		{method: http.MethodGet, path: "/v1/domains/domain-uuid", response: resourceJSON(domainJSON)},
		{method: http.MethodPost, path: "/v1/domains", body: `{"name":"example.test","dkim_selectors":["default"],"tracking_subdomain":null,"tracking_mode":"cloudflare"}`, response: resourceJSON(domainJSON)},
		{method: http.MethodPost, path: "/v1/domains/domain-uuid/verification", response: resourceJSON(verifiedDomain)},
		{method: http.MethodDelete, path: "/v1/domains/domain-uuid", status: http.StatusNoContent},
		{method: http.MethodGet, path: "/v1/domains/domain-uuid/addresses", query: map[string]string{"page": "1", "per_page": "25"}, response: pageJSON(addressJSON, 1, false)},
		{method: http.MethodGet, path: "/v1/domains/domain-uuid/addresses/address-uuid", response: resourceJSON(addressJSON)},
		{method: http.MethodPost, path: "/v1/domains/domain-uuid/addresses", body: `{"address":"sender","display_name":"Sender","reply_to":"reply@example.test","company_address":"123 Main St","company_address_2":null,"company_city":"New York","company_state":"NY","company_zip":"10001","company_country":"US"}`, response: resourceJSON(addressJSON)},
		{method: http.MethodDelete, path: "/v1/domains/domain-uuid/addresses/address-uuid", status: http.StatusNoContent},
	}
	client := expectationClient(t, expectations)
	ctx := context.Background()

	page, err := client.Domains.List(ctx, &DomainListParams{Page: 1, PerPage: 10, Search: "example"})
	if err != nil || len(page.Data) != 1 || page.Data[0].DNS[0].Type != "TXT" {
		t.Fatalf("Domains.List() = %#v, %v", page, err)
	}
	if _, err := client.Domains.Get(ctx, "domain-uuid"); err != nil {
		t.Fatalf("Domains.Get() error = %v", err)
	}
	domain, err := client.Domains.Create(ctx, CreateDomainParams{
		Name:              "example.test",
		DKIMSelectors:     Some([]string{"default"}),
		TrackingSubdomain: Null[string](),
		TrackingMode:      Some(DomainTrackingModeCloudflare),
	})
	if err != nil || domain.Provider != DomainProviderLeadpush {
		t.Fatalf("Domains.Create() = %#v, %v", domain, err)
	}
	domain, err = client.Domains.Verify(ctx, "domain-uuid")
	if err != nil || !domain.Verified || domain.Verification != DomainVerificationCompleted {
		t.Fatalf("Domains.Verify() = %#v, %v", domain, err)
	}
	if err := client.Domains.Delete(ctx, "domain-uuid"); err != nil {
		t.Fatalf("Domains.Delete() error = %v", err)
	}

	addresses := client.Domains.Addresses("domain-uuid")
	addressPage, err := addresses.List(ctx, &DomainAddressListParams{Page: 1, PerPage: 25})
	if err != nil || len(addressPage.Data) != 1 || addressPage.Data[0].Provider == nil || *addressPage.Data[0].Provider != DomainProviderLeadpush {
		t.Fatalf("DomainAddresses.List() = %#v, %v", addressPage, err)
	}
	if _, err := addresses.Get(ctx, "address-uuid"); err != nil {
		t.Fatalf("DomainAddresses.Get() error = %v", err)
	}
	address, err := addresses.Create(ctx, CreateDomainAddressParams{
		Address:         "sender",
		DisplayName:     "Sender",
		ReplyTo:         "reply@example.test",
		CompanyAddress:  "123 Main St",
		CompanyAddress2: Null[string](),
		CompanyCity:     "New York",
		CompanyState:    "NY",
		CompanyZIP:      "10001",
		CompanyCountry:  "US",
	})
	if err != nil || address.FullAddress != "sender@example.test" {
		t.Fatalf("DomainAddresses.Create() = %#v, %v", address, err)
	}
	if err := addresses.Delete(ctx, "address-uuid"); err != nil {
		t.Fatalf("DomainAddresses.Delete() error = %v", err)
	}
}

func TestEmailsFieldsAndSuppressions(t *testing.T) {
	expectations := []expectedRequest{
		{method: http.MethodPost, path: "/v1/emails", body: `{"from":"sender@example.test","subject":"Developer API email","html":"<p>Hello</p>","text":"","to":["person@example.test"],"bcc":["audit@example.test"],"reply_to":"reply@example.test","headers":{"X-Correlation-ID":"abc-123"}}`, response: resourceJSON(emailSendJSON)},
		{method: http.MethodGet, path: "/v1/fields", query: map[string]string{"page": "1", "per_page": "10", "search": "company", "filters": `[{"id":"type","value":["text"]}]`}, response: pageJSON(fieldJSON, 1, false)},
		{method: http.MethodGet, path: "/v1/fields/field-uuid", response: resourceJSON(fieldJSON)},
		{method: http.MethodPost, path: "/v1/fields", body: `{"name":"company","type":"text","format":{"text":"url"}}`, response: resourceJSON(fieldJSON)},
		{method: http.MethodPost, path: "/v1/fields/field-uuid", body: `{"name":"","format":null}`, response: resourceJSON(fieldJSON)},
		{method: http.MethodGet, path: "/v1/suppressions", query: map[string]string{"page": "1", "per_page": "10", "search": "blocked", "filters": `[{"id":"type","value":["manual"]}]`}, response: pageJSON(suppressionJSON, 1, false)},
		{method: http.MethodGet, path: "/v1/suppressions/suppression-uuid", response: resourceJSON(suppressionJSON)},
		{method: http.MethodPost, path: "/v1/suppressions", body: `{"email":"blocked@example.test","type":"manual"}`, response: resourceJSON(suppressionJSON)},
	}
	client := expectationClient(t, expectations)
	ctx := context.Background()

	send, err := client.Emails.Send(ctx, SendEmailParams{
		From:    "sender@example.test",
		Subject: "Developer API email",
		HTML:    Ptr("<p>Hello</p>"),
		Text:    Ptr(""),
		To:      []string{"person@example.test"},
		BCC:     []string{"audit@example.test"},
		ReplyTo: Ptr("reply@example.test"),
		Headers: map[string]string{"X-Correlation-ID": "abc-123"},
	})
	if err != nil || !send.Accepted || send.MessageCount != 2 || send.Messages[1].Type != EmailRecipientTypeBCC {
		t.Fatalf("Emails.Send() = %#v, %v", send, err)
	}

	fieldPage, err := client.Fields.List(ctx, &FieldListParams{
		Page: 1, PerPage: 10, Search: "company",
		Filters: []FieldFilter{{ID: FieldFilterIDType, Value: []FieldType{FieldTypeText}}},
	})
	if err != nil || len(fieldPage.Data) != 1 || fieldPage.Data[0].Format == nil || *fieldPage.Data[0].Format.Text != FieldTextFormatURL {
		t.Fatalf("Fields.List() = %#v, %v", fieldPage, err)
	}
	if _, err := client.Fields.Get(ctx, "field-uuid"); err != nil {
		t.Fatalf("Fields.Get() error = %v", err)
	}
	field, err := client.Fields.Create(ctx, CreateFieldParams{
		Name:   "company",
		Type:   FieldTypeText,
		Format: Some(FieldFormat{Text: Ptr(FieldTextFormatURL)}),
	})
	if err != nil || field.Type != FieldTypeText {
		t.Fatalf("Fields.Create() = %#v, %v", field, err)
	}
	field, err = client.Fields.Update(ctx, "field-uuid", UpdateFieldParams{Name: Ptr(""), Format: Null[FieldFormat]()})
	if err != nil || field.UUID != "field-uuid" {
		t.Fatalf("Fields.Update() = %#v, %v", field, err)
	}

	suppressionPage, err := client.Suppressions.List(ctx, &SuppressionListParams{
		Page: 1, PerPage: 10, Search: "blocked",
		Filters: []SuppressionFilter{{ID: SuppressionFilterIDType, Value: []SuppressionType{SuppressionTypeManual}}},
	})
	if err != nil || len(suppressionPage.Data) != 1 || suppressionPage.Data[0].CreatedAt.Year() != 2026 {
		t.Fatalf("Suppressions.List() = %#v, %v", suppressionPage, err)
	}
	if _, err := client.Suppressions.Get(ctx, "suppression-uuid"); err != nil {
		t.Fatalf("Suppressions.Get() error = %v", err)
	}
	suppression, err := client.Suppressions.Create(ctx, CreateSuppressionParams{Email: "blocked@example.test", Type: Ptr(SuppressionTypeManual)})
	if err != nil || suppression.Type != SuppressionTypeManual {
		t.Fatalf("Suppressions.Create() = %#v, %v", suppression, err)
	}
}

func expectationClient(t *testing.T, expectations []expectedRequest) *Client {
	t.Helper()
	var mutex sync.Mutex
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		mutex.Lock()
		defer mutex.Unlock()
		if index >= len(expectations) {
			t.Errorf("unexpected request: %s %s", request.Method, request.RequestURI)
			writer.WriteHeader(http.StatusInternalServerError)
			return
		}
		expected := expectations[index]
		index++

		if request.Method != expected.method {
			t.Errorf("request %d method = %q, want %q", index, request.Method, expected.method)
		}
		if request.URL.EscapedPath() != expected.path {
			t.Errorf("request %d path = %q, want %q", index, request.URL.EscapedPath(), expected.path)
		}
		if len(request.URL.Query()) != len(expected.query) {
			t.Errorf("request %d query = %v, want %v", index, request.URL.Query(), expected.query)
		}
		for name, value := range expected.query {
			if actual := request.URL.Query().Get(name); actual != value {
				t.Errorf("request %d query %s = %q, want %q", index, name, actual, value)
			}
		}
		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("request %d read body: %v", index, err)
		}
		if expected.body == "" {
			if len(body) != 0 {
				t.Errorf("request %d body = %s, want empty", index, body)
			}
		} else {
			assertJSONEqual(t, body, expected.body)
		}

		status := expected.status
		if status == 0 {
			status = http.StatusOK
		}
		writer.Header().Set("Content-Type", "application/json")
		writer.WriteHeader(status)
		_, _ = writer.Write([]byte(expected.response))
	}))
	t.Cleanup(func() {
		server.Close()
		mutex.Lock()
		defer mutex.Unlock()
		if index != len(expectations) {
			t.Errorf("received %d requests, want %d", index, len(expectations))
		}
	})
	return mustClient(t, server.URL+"/v1")
}

func resourceJSON(data string) string {
	return `{"data":` + data + `}`
}

func pageJSON(data string, current int, hasNext bool) string {
	lastPage := current
	total := 1
	if hasNext {
		lastPage = current + 1
		total = 2
	}
	encoded, _ := json.Marshal(map[string]any{
		"data": []json.RawMessage{json.RawMessage(data)},
		"meta": map[string]any{
			"current_page": current,
			"per_page":     1,
			"total":        total,
			"last_page":    lastPage,
			"has_next":     hasNext,
		},
	})
	return string(encoded)
}

func TestResponseStructsIgnoreUnknownFields(t *testing.T) {
	t.Parallel()
	var contact Contact
	if err := json.Unmarshal([]byte(contactJSON), &contact); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	encoded, err := json.Marshal(contact)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("json.Unmarshal(encoded) error = %v", err)
	}
	if _, exists := raw["future_field"]; exists {
		t.Fatalf("unknown field was preserved: %v", raw)
	}
	if !reflect.DeepEqual(contact.Attributes, Attributes{"email": "person@example.test", "first_name": "Person"}) {
		t.Fatalf("attributes = %#v", contact.Attributes)
	}
}

func TestNullableResponseFields(t *testing.T) {
	t.Parallel()

	var event ContactEvent
	if err := json.Unmarshal([]byte(`{"uuid":"event-uuid","event_name":"login","attributes":null,"created_at":"2026-01-03T12:00:00Z"}`), &event); err != nil {
		t.Fatalf("json.Unmarshal(event) error = %v", err)
	}
	if event.Attributes != nil {
		t.Fatalf("event attributes = %#v, want nil", event.Attributes)
	}

	var address DomainAddress
	if err := json.Unmarshal([]byte(`{"uuid":"address-uuid","domain_uuid":"domain-uuid","address":"sender","full_address":"sender@example.test","provider":null,"display_name":"Sender","verification":"pending","created_at":"2026-01-01T12:00:00Z","updated_at":"2026-01-02T12:00:00Z"}`), &address); err != nil {
		t.Fatalf("json.Unmarshal(address) error = %v", err)
	}
	if address.Provider != nil {
		t.Fatalf("address provider = %#v, want nil", address.Provider)
	}

	var field Field
	if err := json.Unmarshal([]byte(`{"uuid":"field-uuid","name":"score","type":"integer","format":null,"created_at":"2026-01-01T12:00:00Z"}`), &field); err != nil {
		t.Fatalf("json.Unmarshal(field) error = %v", err)
	}
	if field.Format != nil {
		t.Fatalf("field format = %#v, want nil", field.Format)
	}
}
