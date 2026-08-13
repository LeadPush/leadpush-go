package leadpush

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"
)

type httpClientFunc func(*http.Request) (*http.Response, error)

func (function httpClientFunc) Do(request *http.Request) (*http.Response, error) {
	return function(request)
}

func TestNewValidationAndServices(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		key     string
		options []Option
	}{
		{name: "empty API key", key: ""},
		{name: "relative base URL", key: "key", options: []Option{WithBaseURL("/v1")}},
		{name: "base URL query", key: "key", options: []Option{WithBaseURL("https://example.test/v1?x=1")}},
		{name: "negative timeout", key: "key", options: []Option{WithTimeout(-time.Second)}},
		{name: "nil HTTP client", key: "key", options: []Option{WithHTTPClient(nil)}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := New(test.key, test.options...); err == nil {
				t.Fatal("New() error = nil, want validation error")
			}
		})
	}

	client, err := New("key")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if client.Contacts == nil || client.Domains == nil || client.Emails == nil || client.Fields == nil || client.Suppressions == nil {
		t.Fatal("New() did not initialize every resource service")
	}
	if client.baseURL != DefaultBaseURL || client.timeout != DefaultTimeout || client.userAgent != DefaultUserAgent || client.httpClient != http.DefaultClient {
		t.Fatalf("New() defaults = baseURL %q, timeout %s, userAgent %q, HTTP client %#v", client.baseURL, client.timeout, client.userAgent, client.httpClient)
	}
}

func TestLowLevelRequestDefaultsOverridesAndEncoding(t *testing.T) {
	t.Parallel()

	customHeaders := map[string]string{
		"X-App-Name":             "test-app",
		"Authorization":          "unsafe override",
		"X-Leadpush-SDK-Version": "unsafe override",
		"Content-Type":           "text/plain",
	}

	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPost {
			t.Errorf("method = %q, want POST", request.Method)
		}
		if request.URL.EscapedPath() != "/v1/contacts/person%2Fname%40example.test/space%20value" {
			t.Errorf("escaped path = %q", request.URL.EscapedPath())
		}
		if request.URL.Query().Get("page") != "2" || request.URL.Query().Get("active") != "false" {
			t.Errorf("query = %v", request.URL.Query())
		}
		if request.URL.Query().Get("filters") != `[{"id":"type","value":["text"]}]` {
			t.Errorf("filters = %q", request.URL.Query().Get("filters"))
		}

		wantHeaders := map[string]string{
			"Accept":                 "application/json",
			"Authorization":          "Bearer test-key",
			"Content-Type":           "application/json",
			"User-Agent":             "test-agent/1",
			"X-App-Name":             "test-app",
			"X-Leadpush-API-Version": APIVersion,
			"X-Leadpush-SDK":         SDKName,
			"X-Leadpush-SDK-Version": SDKVersion,
		}
		for name, value := range wantHeaders {
			if actual := request.Header.Get(name); actual != value {
				t.Errorf("header %s = %q, want %q", name, actual, value)
			}
		}

		body, err := io.ReadAll(request.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
		}
		assertJSONEqual(t, body, `{"subscribed":false}`)

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	client, err := New(
		"test-key",
		WithBaseURL(server.URL+"/v1/"),
		WithTimeout(0),
		WithHeaders(customHeaders),
		WithUserAgent("test-agent/1"),
	)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	customHeaders["X-App-Name"] = "mutated"

	var response struct {
		OK bool `json:"ok"`
	}
	err = client.Post(
		context.Background(),
		[]string{"contacts", "person/name@example.test", "space value"},
		map[string]any{"subscribed": false},
		map[string]any{
			"page":    2,
			"active":  false,
			"ignored": nil,
			"filters": []map[string]any{{"id": "type", "value": []string{"text"}}},
		},
		&response,
	)
	if err != nil {
		t.Fatalf("Post() error = %v", err)
	}
	if !response.OK {
		t.Fatal("Post() did not decode response")
	}
}

func TestLowLevelResponseFormsAndDelete(t *testing.T) {
	t.Parallel()

	responses := []struct {
		status int
		body   string
	}{
		{status: http.StatusOK, body: `{"value":1}`},
		{status: http.StatusOK, body: "plain text"},
		{status: http.StatusNoContent},
	}
	index := 0
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodDelete {
			t.Errorf("method = %q, want DELETE", request.Method)
		}
		response := responses[index]
		index++
		writer.WriteHeader(response.status)
		_, _ = writer.Write([]byte(response.body))
	}))
	defer server.Close()

	client := mustClient(t, server.URL)
	var payload any
	if err := client.Delete(context.Background(), []string{"one"}, nil, &payload); err != nil {
		t.Fatalf("Delete(JSON) error = %v", err)
	}
	if !reflect.DeepEqual(payload, map[string]any{"value": float64(1)}) {
		t.Fatalf("JSON payload = %#v", payload)
	}

	if err := client.Delete(context.Background(), []string{"two"}, nil, &payload); err != nil {
		t.Fatalf("Delete(text) error = %v", err)
	}
	if payload != "plain text" {
		t.Fatalf("text payload = %#v", payload)
	}

	if err := client.Delete(context.Background(), []string{"three"}, nil, nil); err != nil {
		t.Fatalf("Delete(empty) error = %v", err)
	}
}

func TestAPIErrorStatusesAndPayloads(t *testing.T) {
	t.Parallel()

	tests := []struct {
		status int
		body   string
		helper func(error) bool
	}{
		{status: http.StatusUnauthorized, body: `{"message":"unauthorized"}`, helper: IsUnauthorized},
		{status: http.StatusForbidden, body: `{"message":"forbidden"}`, helper: IsForbidden},
		{status: http.StatusNotFound, body: "missing", helper: IsNotFound},
		{status: http.StatusUnprocessableEntity, body: `{"errors":{"email":["required"]}}`, helper: IsValidation},
		{status: http.StatusInternalServerError, body: "", helper: func(err error) bool {
			return !IsUnauthorized(err) && !IsForbidden(err) && !IsNotFound(err) && !IsValidation(err)
		}},
	}

	for _, test := range tests {
		t.Run(http.StatusText(test.status), func(t *testing.T) {
			t.Parallel()
			server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
				writer.WriteHeader(test.status)
				_, _ = writer.Write([]byte(test.body))
			}))
			defer server.Close()

			err := mustClient(t, server.URL).Get(context.Background(), []string{"failure"}, nil, nil)
			if err == nil {
				t.Fatal("Get() error = nil")
			}
			if !test.helper(err) {
				t.Fatalf("status helper returned false for %T: %v", err, err)
			}

			var apiError *APIError
			if !errors.As(err, &apiError) {
				t.Fatalf("errors.As(%T, *APIError) = false", err)
			}
			if apiError.StatusCode != test.status || string(apiError.RawBody) != test.body {
				t.Fatalf("APIError = %#v", apiError)
			}
			if test.body == "missing" && apiError.Payload != "missing" {
				t.Fatalf("plain text payload = %#v", apiError.Payload)
			}
			if test.body == "" && apiError.Payload != nil {
				t.Fatalf("empty payload = %#v", apiError.Payload)
			}
		})
	}
}

func TestSDKTimeoutAndCallerCancellation(t *testing.T) {
	t.Parallel()

	blockingClient := httpClientFunc(func(request *http.Request) (*http.Response, error) {
		<-request.Context().Done()
		return nil, request.Context().Err()
	})

	client, err := New("key", WithHTTPClient(blockingClient), WithTimeout(10*time.Millisecond))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	err = client.Get(context.Background(), []string{"slow"}, nil, nil)
	var timeoutError *TimeoutError
	if !errors.As(err, &timeoutError) || timeoutError.Timeout != 10*time.Millisecond {
		t.Fatalf("Get() error = %#v, want TimeoutError", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("errors.Is(%v, DeadlineExceeded) = false", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = client.Get(ctx, []string{"cancelled"}, nil, nil)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Get() error = %v, want context.Canceled", err)
	}
	if errors.As(err, &timeoutError) {
		t.Fatalf("caller cancellation mapped to TimeoutError: %v", err)
	}

	client, err = New("key", WithHTTPClient(blockingClient), WithTimeout(time.Second))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err = client.Get(ctx, []string{"caller-deadline"}, nil, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Get() error = %v, want context.DeadlineExceeded", err)
	}
	if errors.As(err, &timeoutError) {
		t.Fatalf("caller deadline mapped to TimeoutError: %v", err)
	}
}

func TestNilContextIsRejected(t *testing.T) {
	t.Parallel()

	err := mustClient(t, "https://example.test").Get(nil, []string{"contacts"}, nil, nil)
	if err == nil || !strings.Contains(err.Error(), "context") {
		t.Fatalf("Get(nil) error = %v", err)
	}
}

func TestTransportErrorsPropagate(t *testing.T) {
	t.Parallel()

	want := errors.New("transport failure")
	client, err := New("key", WithHTTPClient(httpClientFunc(func(*http.Request) (*http.Response, error) {
		return nil, want
	})))
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if actual := client.Get(context.Background(), []string{"failure"}, nil, nil); !errors.Is(actual, want) {
		t.Fatalf("Get() error = %v, want transport error", actual)
	}
}

func mustClient(t *testing.T, baseURL string, options ...Option) *Client {
	t.Helper()
	options = append([]Option{WithBaseURL(baseURL), WithTimeout(0)}, options...)
	client, err := New("test-key", options...)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return client
}

func assertJSONEqual(t *testing.T, actual []byte, expected string) {
	t.Helper()
	var actualValue any
	if err := json.Unmarshal(actual, &actualValue); err != nil {
		t.Fatalf("decode actual JSON %q: %v", string(actual), err)
	}
	var expectedValue any
	if err := json.Unmarshal([]byte(expected), &expectedValue); err != nil {
		t.Fatalf("decode expected JSON %q: %v", expected, err)
	}
	if !reflect.DeepEqual(actualValue, expectedValue) {
		t.Fatalf("JSON = %s, want %s", strings.TrimSpace(string(actual)), expected)
	}
}
