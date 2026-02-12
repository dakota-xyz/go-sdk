package client

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

func TestResponseError_FromGeneratedErrorPayload(t *testing.T) {
	details := map[string]any{"field": "amount"}
	resp := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"X-Request-Id": []string{"req_123"}},
		},
		JSON400: &gen.Error{
			Code:    "invalid_request",
			Message: "amount is required",
			Details: &details,
		},
		Body: []byte(`{"code":"invalid_request","message":"amount is required"}`),
	}

	err := ResponseError(resp)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", apiErr.StatusCode, http.StatusBadRequest)
	}
	if apiErr.Code != "invalid_request" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "invalid_request")
	}
	if apiErr.RequestID != "req_123" {
		t.Fatalf("request ID = %q, want %q", apiErr.RequestID, "req_123")
	}
	if apiErr.Details["field"] != "amount" {
		t.Fatalf("details[field] = %v, want %q", apiErr.Details["field"], "amount")
	}
}

func TestResponseError_FallsBackToBody(t *testing.T) {
	resp := &gen.ListCustomersResponse{
		HTTPResponse: &http.Response{
			StatusCode: http.StatusServiceUnavailable,
			Header:     make(http.Header),
		},
		Body: []byte(`{"code":"upstream_unavailable","message":"please retry later","details":{"region":"us"}}`),
	}

	err := ResponseError(resp)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.Retryable {
		t.Fatal("expected retryable=true")
	}
	if apiErr.Code != "upstream_unavailable" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "upstream_unavailable")
	}
	if apiErr.Message != "please retry later" {
		t.Fatalf("message = %q, want %q", apiErr.Message, "please retry later")
	}
}

func TestResponseError_4xxMapping(t *testing.T) {
	tests := []struct {
		name       string
		resp       any
		wantStatus int
		wantCode   string
	}{
		{
			name: "400",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header)},
				JSON400:      &gen.Error{Code: "bad_request", Message: "bad request"},
			},
			wantStatus: http.StatusBadRequest,
			wantCode:   "bad_request",
		},
		{
			name: "401",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header)},
				JSON401:      &gen.Error{Code: "unauthorized", Message: "unauthorized"},
			},
			wantStatus: http.StatusUnauthorized,
			wantCode:   "unauthorized",
		},
		{
			name: "403",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header)},
				JSON403:      &gen.Error{Code: "forbidden", Message: "forbidden"},
			},
			wantStatus: http.StatusForbidden,
			wantCode:   "forbidden",
		},
		{
			name: "404",
			resp: &gen.GetApplicationResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header)},
				JSON404:      &gen.Error{Code: "not_found", Message: "not found"},
			},
			wantStatus: http.StatusNotFound,
			wantCode:   "not_found",
		},
		{
			name: "422",
			resp: &gen.EstimateTransactionResponse{
				HTTPResponse: &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: make(http.Header)},
				JSON422:      &gen.Error{Code: "invalid_entity", Message: "invalid"},
			},
			wantStatus: http.StatusUnprocessableEntity,
			wantCode:   "invalid_entity",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ResponseError(tt.resp)
			var apiErr *APIError
			if !errors.As(err, &apiErr) {
				t.Fatalf("expected APIError, got %T", err)
			}
			if apiErr.StatusCode != tt.wantStatus {
				t.Fatalf("status = %d, want %d", apiErr.StatusCode, tt.wantStatus)
			}
			if apiErr.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", apiErr.Code, tt.wantCode)
			}
		})
	}
}

func TestResponseError_EdgeCases(t *testing.T) {
	var nilTypedResp *gen.ListApplicationsResponse
	if err := ResponseError(nilTypedResp); err != nil {
		t.Fatalf("expected nil error for typed nil response, got %v", err)
	}

	if err := ResponseError(42); err != nil {
		t.Fatalf("expected nil error for non-response type, got %v", err)
	}

	okResp := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)},
	}
	if err := ResponseError(okResp); err != nil {
		t.Fatalf("expected nil error for 2xx, got %v", err)
	}

	serverErr := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusInternalServerError, Header: make(http.Header)},
		Body:         []byte("not-json"),
	}
	err := ResponseError(serverErr)
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if apiErr.Code != "http_500" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "http_500")
	}
	if apiErr.Message != http.StatusText(http.StatusInternalServerError) {
		t.Fatalf("message = %q, want %q", apiErr.Message, http.StatusText(http.StatusInternalServerError))
	}
	if !apiErr.Retryable {
		t.Fatal("expected retryable=true for 500")
	}
}

func TestCheckResponse_TransportError(t *testing.T) {
	_, err := CheckResponse[*gen.ListApplicationsResponse](nil, io.EOF)
	if err == nil {
		t.Fatal("expected error")
	}

	var transportErr *TransportError
	if !errors.As(err, &transportErr) {
		t.Fatalf("expected TransportError, got %T", err)
	}
}

func TestCheckResponse_APIError(t *testing.T) {
	resp := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusTooManyRequests, Header: make(http.Header)},
		Body:         []byte(`{"code":"rate_limited","message":"slow down"}`),
	}

	_, err := CheckResponse(resp, nil)
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T", err)
	}
	if !apiErr.Retryable {
		t.Fatal("expected retryable=true")
	}
}

func TestCheckResponse_Success(t *testing.T) {
	resp := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{StatusCode: http.StatusOK, Header: make(http.Header)},
	}
	got, err := CheckResponse(resp, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != resp {
		t.Fatal("expected original response to be returned")
	}
}

func TestAPIErrorErrorString(t *testing.T) {
	e := &APIError{StatusCode: 400, Code: "bad_request", Message: "invalid"}
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty error string")
	}

	e2 := &APIError{StatusCode: 500, Message: "boom"}
	if got := e2.Error(); got == "" {
		t.Fatal("expected non-empty error string for code-less error")
	}
}

func TestTransportErrorErrorStringAndUnwrap(t *testing.T) {
	root := io.EOF
	e := &TransportError{Err: root}
	if got := e.Error(); got == "" {
		t.Fatal("expected non-empty error string")
	}
	if !errors.Is(e, root) {
		t.Fatal("expected TransportError to unwrap to root error")
	}

	var nilErr *TransportError
	if got := nilErr.Error(); got == "" {
		t.Fatal("expected nil TransportError string fallback")
	}
}
