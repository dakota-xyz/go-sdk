package client

import (
	"errors"
	"io"
	"net/http"
	"testing"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

func TestResponseError_FromGeneratedErrorPayload(t *testing.T) {
	title := "Invalid request"
	detail := "amount is required"
	typeURI := "https://docs.dakota.xyz/api-reference/errors#invalid-request"
	requestID := "req_123"
	resp := &gen.ListApplicationsResponse{
		HTTPResponse: &http.Response{
			StatusCode: http.StatusBadRequest,
			Header:     http.Header{"X-Request-Id": []string{requestID}},
		},
		ApplicationproblemJSON400: &gen.ProblemDetails{
			Title:     title,
			Detail:    &detail,
			Type:      typeURI,
			Status:    http.StatusBadRequest,
			RequestId: &requestID,
		},
		Body: []byte(`{"title":"Invalid request","detail":"amount is required","status":400}`),
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
	if apiErr.Code != "invalid-request" {
		t.Fatalf("code = %q, want %q", apiErr.Code, "invalid-request")
	}
	if apiErr.RequestID != "req_123" {
		t.Fatalf("request ID = %q, want %q", apiErr.RequestID, "req_123")
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
		name        string
		resp        any
		wantStatus  int
		wantMessage string
	}{
		{
			name: "400",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse:              &http.Response{StatusCode: http.StatusBadRequest, Header: make(http.Header)},
				ApplicationproblemJSON400: &gen.ProblemDetails{Title: "Bad request", Status: http.StatusBadRequest},
			},
			wantStatus:  http.StatusBadRequest,
			wantMessage: "Bad request",
		},
		{
			name: "401",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse:              &http.Response{StatusCode: http.StatusUnauthorized, Header: make(http.Header)},
				ApplicationproblemJSON401: &gen.ProblemDetails{Title: "Unauthorized", Status: http.StatusUnauthorized},
			},
			wantStatus:  http.StatusUnauthorized,
			wantMessage: "Unauthorized",
		},
		{
			name: "403",
			resp: &gen.ListApplicationsResponse{
				HTTPResponse:              &http.Response{StatusCode: http.StatusForbidden, Header: make(http.Header)},
				ApplicationproblemJSON403: &gen.ProblemDetails{Title: "Forbidden", Status: http.StatusForbidden},
			},
			wantStatus:  http.StatusForbidden,
			wantMessage: "Forbidden",
		},
		{
			name: "404",
			resp: &gen.GetApplicationResponse{
				HTTPResponse:              &http.Response{StatusCode: http.StatusNotFound, Header: make(http.Header)},
				ApplicationproblemJSON404: &gen.ProblemDetails{Title: "Not found", Status: http.StatusNotFound},
			},
			wantStatus:  http.StatusNotFound,
			wantMessage: "Not found",
		},
		{
			name: "422",
			resp: &gen.CreateTransactionResponse{
				HTTPResponse:              &http.Response{StatusCode: http.StatusUnprocessableEntity, Header: make(http.Header)},
				ApplicationproblemJSON422: &gen.ProblemDetails{Title: "Unprocessable entity", Status: http.StatusUnprocessableEntity},
			},
			wantStatus:  http.StatusUnprocessableEntity,
			wantMessage: "Unprocessable entity",
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
			if apiErr.Message != tt.wantMessage {
				t.Fatalf("message = %q, want %q", apiErr.Message, tt.wantMessage)
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
