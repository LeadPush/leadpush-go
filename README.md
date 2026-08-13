# leadpush-go

Official Go SDK for the Leadpush API.

Create a Leadpush account at [leadpush.io](https://leadpush.io).

## Installation

```sh
go get github.com/LeadPush/leadpush-go@v1.0.0
```

Requirements:

- Go 1.25 or newer
- A Leadpush API key

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	leadpush "github.com/LeadPush/leadpush-go"
)

func main() {
	client, err := leadpush.New(os.Getenv("LEADPUSH_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}

	contacts, err := client.Contacts.List(context.Background(), &leadpush.ContactListParams{
		Page:    1,
		PerPage: 10,
	})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(contacts.Data)
}
```

## Configuration

```go
client, err := leadpush.New(
	"leadpush_api_key",
	leadpush.WithBaseURL("https://api.leadpush.io/v1"),
	leadpush.WithTimeout(30*time.Second),
	leadpush.WithHeaders(map[string]string{"X-App-Name": "my-app"}),
	leadpush.WithUserAgent("my-app/1.0"),
	leadpush.WithHTTPClient(customHTTPClient),
)
```

Defaults:

- `baseURL`: `https://api.leadpush.io/v1`
- `timeout`: 30 seconds; use `WithTimeout(0)` to disable the SDK timeout
- `userAgent`: `leadpush-go/1.0.0 (api=v1)`
- `httpClient`: `http.DefaultClient`

An HTTP client passed with `WithHTTPClient` remains caller-owned.

## Contacts

Contact identifiers may be either a contact UUID or the workspace identity value, such as an email address.

```go
contact, err := client.Contacts.Get(ctx, "person@example.com")

created, err := client.Contacts.Create(ctx, leadpush.CreateContactParams{
	Attributes: leadpush.Attributes{
		"email":      "person@example.com",
		"first_name": "Person",
	},
	Subscribed: leadpush.Ptr(true),
})

updated, err := client.Contacts.Update(ctx, contact.UUID, leadpush.UpdateContactParams{
	Attributes: leadpush.Attributes{"first_name": "Updated"},
	Subscribed: leadpush.Ptr(false),
})

contact, err = client.Contacts.Subscribe(ctx, "person@example.com")
contact, err = client.Contacts.Unsubscribe(ctx, "person@example.com")
```

### Contact events

```go
events := client.Contacts.Events("person@example.com")

page, err := events.List(ctx, &leadpush.ContactEventListParams{
	Search: "purchase",
})

err = events.Create(ctx, leadpush.CreateContactEventParams{
	EventName:  "purchase",
	Attributes: leadpush.ContactEventAttributes{"plan": "enterprise"},
})
```

Event creation returns only an error because the API does not return the created event.

## Pagination

List methods retrieve one page. A pager advances using the API's `has_next` metadata and preserves request errors:

```go
pager := client.Contacts.NewPager(&leadpush.ContactListParams{PerPage: 100})

for pager.Next(ctx) {
	for _, contact := range pager.Page().Data {
		fmt.Println(contact.UUID)
	}
}

if err := pager.Err(); err != nil {
	log.Fatal(err)
}
```

Setting `Page` on the list parameters controls the pager's starting page.

## Domains and addresses

```go
domain, err := client.Domains.Create(ctx, leadpush.CreateDomainParams{
	Name:              "example.com",
	DKIMSelectors:     leadpush.Some([]string{"default"}),
	TrackingSubdomain: leadpush.Some("click"),
	TrackingMode:      leadpush.Some(leadpush.DomainTrackingModeCloudflare),
})

domain, err = client.Domains.Verify(ctx, domain.UUID)

addresses := client.Domains.Addresses(domain.UUID)
address, err := addresses.Create(ctx, leadpush.CreateDomainAddressParams{
	Address:        "sender",
	DisplayName:    "Sender Name",
	ReplyTo:        "reply@example.com",
	CompanyAddress: "123 Main St",
	CompanyCity:    "New York",
	CompanyState:   "NY",
	CompanyZIP:     "10001",
	CompanyCountry: "US",
})

err = addresses.Delete(ctx, address.UUID)
err = client.Domains.Delete(ctx, domain.UUID)
```

`Optional[T]` distinguishes omitted, explicit values, and explicit `null`:

```go
leadpush.Some("click")
leadpush.Null[string]()
```

Use `Ptr` for pointer request fields when a zero value, such as `false` or an empty string, must be included.

## Emails

```go
send, err := client.Emails.Send(ctx, leadpush.SendEmailParams{
	From:    "sender@example.com",
	Subject: "Developer API email",
	HTML:    leadpush.Ptr("<p>Hello world</p>"),
	Text:    leadpush.Ptr("Hello world"),
	To:      []string{"known@example.com", "other@example.com"},
	BCC:     []string{"audit@example.com"},
	ReplyTo: leadpush.Ptr("reply@example.com"),
	Headers: map[string]string{"X-Correlation-ID": "abc-123"},
})

fmt.Println(send.Accepted, send.MessageCount)
```

The sender must be a verified sendable address in the API key's workspace. Provide HTML, text, or both, and at least one recipient across `To` and `BCC`.

## Fields and suppressions

```go
fields, err := client.Fields.List(ctx, &leadpush.FieldListParams{
	Search: "company",
	Filters: []leadpush.FieldFilter{{
		ID:    leadpush.FieldFilterIDType,
		Value: []leadpush.FieldType{leadpush.FieldTypeText},
	}},
})

field, err := client.Fields.Create(ctx, leadpush.CreateFieldParams{
	Name: "company_name",
	Type: leadpush.FieldTypeText,
	Format: leadpush.Some(leadpush.FieldFormat{
		Text: leadpush.Ptr(leadpush.FieldTextFormatURL),
	}),
})

suppression, err := client.Suppressions.Create(ctx, leadpush.CreateSuppressionParams{
	Email: "blocked@example.com",
	Type:  leadpush.Ptr(leadpush.SuppressionTypeManual),
})
```

Suppressions intentionally do not expose an update method because the API endpoint is unsupported.

## Low-level requests

Use `Get`, `Post`, `Delete`, or `Do` for endpoints without a typed resource method:

```go
var response any
err := client.Get(
	ctx,
	[]string{"contacts", "person@example.com", "events"},
	map[string]any{"page": 1},
	&response,
)
```

Each path slice entry is escaped as one segment, so identity values containing `/`, `@`, or spaces are preserved. Arrays and objects in query values are encoded as compact JSON.

## Errors

```go
contact, err := client.Contacts.Get(ctx, "missing-contact")
if err != nil {
	var apiError *leadpush.APIError
	if errors.As(err, &apiError) {
		fmt.Println(apiError.StatusCode, apiError.Payload)
	}

	if leadpush.IsNotFound(err) {
		fmt.Println("contact not found")
	}
}
```
