package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"reflect"
)

// APIError represents a structured Dakota Platform API error response.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
	Details    map[string]any
	RequestID  string
	RawBody    []byte
	Retryable  bool
}

func (e *APIError) Error() string {
	if e == nil {
		return ""
	}
	if e.Code == "" {
		return fmt.Sprintf("platform API error (%d): %s", e.StatusCode, e.Message)
	}
	return fmt.Sprintf(
		"platform API error (%d %s): %s",
		e.StatusCode,
		e.Code,
		e.Message,
	)
}

// TransportError wraps non-HTTP failures (network, context, decode, etc.).
type TransportError struct {
	Err error
}

func (e *TransportError) Error() string {
	if e == nil || e.Err == nil {
		return "platform API transport error"
	}
	return fmt.Sprintf("platform API transport error: %v", e.Err)
}

func (e *TransportError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

// CheckResponse maps call/response failures into typed SDK errors.
//
// Usage:
//
//	resp, err := client.CheckResponse(c.Raw().ListCustomersWithResponse(ctx, params))
func CheckResponse[T interface{ StatusCode() int }](resp T, err error) (T, error) {
	if err != nil {
		var zero T
		return zero, &TransportError{Err: err}
	}
	if apiErr := ResponseError(resp); apiErr != nil {
		var zero T
		return zero, apiErr
	}
	return resp, nil
}

// ResponseError converts a generated response object into an APIError when
// the HTTP status indicates failure. Returns nil for non-error responses.
func ResponseError(resp any) error {
	if resp == nil {
		return nil
	}

	v := reflect.ValueOf(resp)
	if !v.IsValid() {
		return nil
	}
	if v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}

	statusCarrier, ok := resp.(interface{ StatusCode() int })
	if !ok {
		return nil
	}
	status := statusCarrier.StatusCode()
	if status < 400 {
		return nil
	}

	if v.Kind() == reflect.Pointer {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	body := extractBody(v)
	code, message, details := extractErrorFields(v, status)
	if fallbackCode, fallbackMessage, fallbackDetails, ok := parseBodyError(body); ok {
		if code == "" {
			code = fallbackCode
		}
		if message == "" {
			message = fallbackMessage
		}
		if details == nil {
			details = fallbackDetails
		}
	}

	if code == "" {
		code = fmt.Sprintf("http_%d", status)
	}
	if message == "" {
		message = http.StatusText(status)
	}

	requestID := ""
	if httpResp := extractHTTPResponse(v); httpResp != nil {
		requestID = httpResp.Header.Get("X-Request-Id")
	}

	return &APIError{
		StatusCode: status,
		Code:       code,
		Message:    message,
		Details:    details,
		RequestID:  requestID,
		RawBody:    body,
		Retryable:  isRetryableStatus(status),
	}
}

func extractBody(v reflect.Value) []byte {
	field := v.FieldByName("Body")
	if !field.IsValid() || field.Kind() != reflect.Slice {
		return nil
	}
	if field.Type().Elem().Kind() != reflect.Uint8 {
		return nil
	}
	body := make([]byte, field.Len())
	reflect.Copy(reflect.ValueOf(body), field)
	return body
}

func extractHTTPResponse(v reflect.Value) *http.Response {
	field := v.FieldByName("HTTPResponse")
	if !field.IsValid() || field.IsNil() {
		return nil
	}
	httpResp, ok := field.Interface().(*http.Response)
	if !ok {
		return nil
	}
	return httpResp
}

func extractErrorFields(
	v reflect.Value,
	status int,
) (string, string, map[string]any) {
	for _, name := range []string{fmt.Sprintf("JSON%d", status), "JSONDefault"} {
		field := v.FieldByName(name)
		if !field.IsValid() || (field.Kind() == reflect.Pointer && field.IsNil()) {
			continue
		}
		code, message, details, ok := parseErrorModel(field)
		if ok {
			return code, message, details
		}
	}
	return "", "", nil
}

func parseErrorModel(
	field reflect.Value,
) (string, string, map[string]any, bool) {
	for field.Kind() == reflect.Pointer {
		if field.IsNil() {
			return "", "", nil, false
		}
		field = field.Elem()
	}
	if field.Kind() != reflect.Struct {
		return "", "", nil, false
	}

	code := stringField(field, "Code")
	message := stringField(field, "Message")
	details := mapField(field, "Details")
	if code == "" && message == "" && details == nil {
		return "", "", nil, false
	}

	return code, message, details, true
}

func stringField(v reflect.Value, name string) string {
	f := v.FieldByName(name)
	if !f.IsValid() {
		return ""
	}
	if f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return ""
		}
		f = f.Elem()
	}
	if f.Kind() != reflect.String {
		return ""
	}
	return f.String()
}

func mapField(v reflect.Value, name string) map[string]any {
	f := v.FieldByName(name)
	if !f.IsValid() {
		return nil
	}
	for f.Kind() == reflect.Pointer {
		if f.IsNil() {
			return nil
		}
		f = f.Elem()
	}
	if f.Kind() != reflect.Map || f.Type().Key().Kind() != reflect.String {
		return nil
	}
	out := make(map[string]any, f.Len())
	iter := f.MapRange()
	for iter.Next() {
		out[iter.Key().String()] = iter.Value().Interface()
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func parseBodyError(body []byte) (string, string, map[string]any, bool) {
	if len(body) == 0 {
		return "", "", nil, false
	}
	var parsed struct {
		Code    string         `json:"code"`
		Message string         `json:"message"`
		Details map[string]any `json:"details"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", "", nil, false
	}
	if parsed.Code == "" && parsed.Message == "" && len(parsed.Details) == 0 {
		return "", "", nil, false
	}
	if len(parsed.Details) == 0 {
		parsed.Details = nil
	}
	return parsed.Code, parsed.Message, parsed.Details, true
}
