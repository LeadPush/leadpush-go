package leadpush

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// HTTPClient is implemented by *http.Client and compatible custom transports.
type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type clientConfig struct {
	baseURL    string
	timeout    time.Duration
	headers    map[string]string
	userAgent  string
	httpClient HTTPClient
}

// Option configures a Client.
type Option func(*clientConfig)

// WithBaseURL overrides the Leadpush API base URL.
func WithBaseURL(baseURL string) Option {
	return func(config *clientConfig) {
		config.baseURL = baseURL
	}
}

// WithTimeout overrides the per-request SDK timeout. A zero duration disables
// the SDK timeout. Negative durations are rejected by New.
func WithTimeout(timeout time.Duration) Option {
	return func(config *clientConfig) {
		config.timeout = timeout
	}
}

// WithHeaders adds headers to every request. The input map is copied. SDK-owned
// authentication, version, user-agent, and content headers take precedence.
func WithHeaders(headers map[string]string) Option {
	cloned := cloneHeaders(headers)
	return func(config *clientConfig) {
		for name, value := range cloned {
			config.headers[name] = value
		}
	}
}

// WithUserAgent overrides the default SDK user agent.
func WithUserAgent(userAgent string) Option {
	return func(config *clientConfig) {
		config.userAgent = userAgent
	}
}

// WithHTTPClient uses client for requests. The client remains caller-owned.
func WithHTTPClient(client HTTPClient) Option {
	return func(config *clientConfig) {
		config.httpClient = client
	}
}

// Client is a Leadpush API client.
type Client struct {
	apiKey     string
	baseURL    string
	timeout    time.Duration
	headers    map[string]string
	userAgent  string
	httpClient HTTPClient

	Contacts     *ContactsService
	Domains      *DomainsService
	Emails       *EmailsService
	Fields       *FieldsService
	Suppressions *SuppressionsService
}

// New creates a Leadpush API client.
func New(apiKey string, options ...Option) (*Client, error) {
	config := clientConfig{
		baseURL:    DefaultBaseURL,
		timeout:    DefaultTimeout,
		headers:    make(map[string]string),
		userAgent:  DefaultUserAgent,
		httpClient: http.DefaultClient,
	}

	for _, option := range options {
		if option != nil {
			option(&config)
		}
	}

	if strings.TrimSpace(apiKey) == "" {
		return nil, errors.New("leadpush: API key must not be empty")
	}
	if config.timeout < 0 {
		return nil, errors.New("leadpush: timeout must not be negative")
	}
	if config.httpClient == nil {
		return nil, errors.New("leadpush: HTTP client must not be nil")
	}

	baseURL, err := validateBaseURL(config.baseURL)
	if err != nil {
		return nil, err
	}

	client := &Client{
		apiKey:     apiKey,
		baseURL:    baseURL,
		timeout:    config.timeout,
		headers:    cloneHeaders(config.headers),
		userAgent:  config.userAgent,
		httpClient: config.httpClient,
	}
	client.Contacts = &ContactsService{client: client}
	client.Domains = &DomainsService{client: client}
	client.Emails = &EmailsService{client: client}
	client.Fields = &FieldsService{client: client}
	client.Suppressions = &SuppressionsService{client: client}

	return client, nil
}

// Request describes a low-level Leadpush API request.
type Request struct {
	Method string
	Path   []string
	Query  map[string]any
	Body   any
}

// Do performs a low-level request and decodes the response into result. Path
// entries are always treated as individual URL path segments. Pass nil for
// result when no response body is expected.
func (c *Client) Do(ctx context.Context, request Request, result any) error {
	if ctx == nil {
		return errors.New("leadpush: context must not be nil")
	}

	requestURL, err := c.requestURL(request.Path, request.Query)
	if err != nil {
		return err
	}

	var body io.Reader
	if request.Body != nil {
		encoded, marshalErr := json.Marshal(request.Body)
		if marshalErr != nil {
			return fmt.Errorf("leadpush: encode request body: %w", marshalErr)
		}
		body = bytes.NewReader(encoded)
	}

	requestContext := ctx
	cancel := func() {}
	sdkTimeout := c.shouldApplyTimeout(ctx)
	if sdkTimeout {
		requestContext, cancel = context.WithTimeout(ctx, c.timeout)
	}
	defer cancel()

	httpRequest, err := http.NewRequestWithContext(requestContext, request.Method, requestURL, body)
	if err != nil {
		return fmt.Errorf("leadpush: create HTTP request: %w", err)
	}
	c.setHeaders(httpRequest, request.Body != nil)

	response, err := c.httpClient.Do(httpRequest)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if sdkTimeout && errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return &TimeoutError{Timeout: c.timeout, Err: context.DeadlineExceeded}
		}
		return err
	}
	defer response.Body.Close()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if sdkTimeout && errors.Is(requestContext.Err(), context.DeadlineExceeded) {
			return &TimeoutError{Timeout: c.timeout, Err: context.DeadlineExceeded}
		}
		return fmt.Errorf("leadpush: read response body: %w", err)
	}

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return newAPIError(response.StatusCode, responseBody)
	}

	return decodeResponse(responseBody, result)
}

// Get performs a low-level GET request.
func (c *Client) Get(ctx context.Context, path []string, query map[string]any, result any) error {
	return c.Do(ctx, Request{Method: http.MethodGet, Path: path, Query: query}, result)
}

// Post performs a low-level POST request.
func (c *Client) Post(ctx context.Context, path []string, body any, query map[string]any, result any) error {
	return c.Do(ctx, Request{Method: http.MethodPost, Path: path, Query: query, Body: body}, result)
}

// Delete performs a low-level DELETE request.
func (c *Client) Delete(ctx context.Context, path []string, query map[string]any, result any) error {
	return c.Do(ctx, Request{Method: http.MethodDelete, Path: path, Query: query}, result)
}

func (c *Client) shouldApplyTimeout(ctx context.Context) bool {
	if c.timeout == 0 {
		return false
	}
	if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) <= c.timeout {
		return false
	}
	return true
}

func (c *Client) requestURL(path []string, query map[string]any) (string, error) {
	var builder strings.Builder
	builder.WriteString(c.baseURL)
	for _, segment := range path {
		if segment == "" {
			continue
		}
		builder.WriteByte('/')
		builder.WriteString(escapePathSegment(segment))
	}

	values := url.Values{}
	for name, value := range query {
		encoded, include, err := encodeQueryValue(value)
		if err != nil {
			return "", fmt.Errorf("leadpush: encode query parameter %q: %w", name, err)
		}
		if include {
			values.Set(name, encoded)
		}
	}
	if encodedQuery := values.Encode(); encodedQuery != "" {
		builder.WriteByte('?')
		builder.WriteString(encodedQuery)
	}

	return builder.String(), nil
}

func (c *Client) setHeaders(request *http.Request, hasBody bool) {
	for name, value := range c.headers {
		request.Header.Set(name, value)
	}

	request.Header.Set("Accept", "application/json")
	request.Header.Set("Authorization", "Bearer "+c.apiKey)
	request.Header.Set("User-Agent", c.userAgent)
	request.Header.Set("X-Leadpush-API-Version", APIVersion)
	request.Header.Set("X-Leadpush-SDK", SDKName)
	request.Header.Set("X-Leadpush-SDK-Version", SDKVersion)
	if hasBody {
		request.Header.Set("Content-Type", "application/json")
	}
}

func validateBaseURL(value string) (string, error) {
	parsed, err := url.Parse(value)
	if err != nil {
		return "", fmt.Errorf("leadpush: invalid base URL: %w", err)
	}
	if (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return "", errors.New("leadpush: base URL must be an absolute HTTP or HTTPS URL")
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return "", errors.New("leadpush: base URL must not contain a query or fragment")
	}

	return strings.TrimRight(parsed.String(), "/"), nil
}

func cloneHeaders(headers map[string]string) map[string]string {
	cloned := make(map[string]string, len(headers))
	for name, value := range headers {
		cloned[name] = value
	}
	return cloned
}

func escapePathSegment(segment string) string {
	const hexadecimal = "0123456789ABCDEF"
	var builder strings.Builder

	for _, value := range []byte(segment) {
		if isURIComponentSafe(value) {
			builder.WriteByte(value)
			continue
		}

		builder.WriteByte('%')
		builder.WriteByte(hexadecimal[value>>4])
		builder.WriteByte(hexadecimal[value&15])
	}

	return builder.String()
}

func isURIComponentSafe(value byte) bool {
	return value >= 'a' && value <= 'z' ||
		value >= 'A' && value <= 'Z' ||
		value >= '0' && value <= '9' ||
		strings.ContainsRune("-_.!~*'()", rune(value))
}

func encodeQueryValue(value any) (string, bool, error) {
	if value == nil {
		return "", false, nil
	}

	reflected := reflect.ValueOf(value)
	if reflected.Kind() == reflect.Pointer {
		if reflected.IsNil() {
			return "", false, nil
		}
		return encodeQueryValue(reflected.Elem().Interface())
	}

	switch reflected.Kind() {
	case reflect.String:
		return reflected.String(), true, nil
	case reflect.Bool:
		return strconv.FormatBool(reflected.Bool()), true, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return strconv.FormatInt(reflected.Int(), 10), true, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return strconv.FormatUint(reflected.Uint(), 10), true, nil
	case reflect.Float32, reflect.Float64:
		return strconv.FormatFloat(reflected.Float(), 'g', -1, reflected.Type().Bits()), true, nil
	default:
		encoded, err := json.Marshal(value)
		return string(encoded), true, err
	}
}

func decodeResponse(body []byte, result any) error {
	if result == nil || len(body) == 0 {
		return nil
	}

	switch destination := result.(type) {
	case *string:
		*destination = string(body)
		return nil
	case *[]byte:
		*destination = append((*destination)[:0], body...)
		return nil
	case *any:
		var payload any
		if err := json.Unmarshal(body, &payload); err != nil {
			payload = string(body)
		}
		*destination = payload
		return nil
	default:
		if err := json.Unmarshal(body, result); err != nil {
			return fmt.Errorf("leadpush: decode response body: %w", err)
		}
		return nil
	}
}

func newAPIError(statusCode int, body []byte) *APIError {
	var payload any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &payload); err != nil {
			payload = string(body)
		}
	}

	return &APIError{
		StatusCode: statusCode,
		Payload:    payload,
		RawBody:    append([]byte(nil), body...),
	}
}

type resourceResponse[T any] struct {
	Data T `json:"data"`
}
