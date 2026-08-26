package types_test

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
)

// decodeEvent decodes a webhook fixture through the SDK's public
// [webhook.EventDataAs] path and, in the same step, holds the fixture to two
// properties a lenient decoder cannot enforce:
//
//   - every key in the fixture is modeled by T, so a struct that has drifted
//     behind its platform emitter fails loudly instead of dropping the key;
//   - every non-omitempty field of T is present in the fixture, so a field the
//     platform never sends cannot sit in the struct decoding to a silent zero.
//
// The second property is the one this package exists to defend: a mis-tagged or
// invented field costs nothing at compile time and nothing at decode time — it
// simply hands the consumer "" or 0 forever. Requiring the fixture to carry
// every required field turns that into a test failure the moment it is written.
//
// Production decoding stays lenient on purpose: an unknown key must never break
// a live consumer when the platform adds one. The strictness here is a contract
// on fixtures, not on the wire.
func decodeEvent[T any](
	t *testing.T,
	id string,
	eventType webhook.EventType,
	payload string,
) T {
	t.Helper()

	return decodeEventIgnoring[T](t, id, eventType, payload)
}

// decodeEventIgnoring is decodeEvent for a payload that carries keys the SDK
// deliberately does not model — an emitter key held back for a later release,
// or one (an API-key hash, say) that has no business on a client-facing struct.
// Naming each such key at the call site keeps the fixture a faithful copy of
// what the platform sends while still refusing any key left unmodeled by
// accident. The full payload still goes through [webhook.EventDataAs], proving
// the lenient production path tolerates the extra keys.
func decodeEventIgnoring[T any](
	t *testing.T,
	id string,
	eventType webhook.EventType,
	payload string,
	unmodeled ...string,
) T {
	t.Helper()

	event := webhook.Event{
		ID:   id,
		Type: eventType,
		Data: webhook.EventData{Object: json.RawMessage(payload)},
	}

	data, err := webhook.EventDataAs[T](event)
	if err != nil {
		t.Fatalf("EventDataAs error: %v", err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &raw); err != nil {
		t.Fatalf("fixture is not a JSON object: %v", err)
	}
	for _, key := range unmodeled {
		if _, present := raw[key]; !present {
			t.Errorf(
				"fixture declares %q as deliberately unmodeled but does not "+
					"carry it; drop it from the call",
				key,
			)
		}
		delete(raw, key)
	}

	modeled, err := json.Marshal(raw)
	if err != nil {
		t.Fatalf("re-encoding fixture: %v", err)
	}

	var strict T
	dec := json.NewDecoder(bytes.NewReader(modeled))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&strict); err != nil {
		t.Fatalf(
			"fixture carries a key %T does not model: %v",
			strict,
			err,
		)
	}

	assertRequiredKeys(t, reflect.TypeOf(strict), raw, "")

	return data
}

// assertRequiredKeys fails when a non-omitempty field of structType has no
// corresponding key in raw, recursing into nested objects that are present.
func assertRequiredKeys(
	t *testing.T,
	structType reflect.Type,
	raw map[string]json.RawMessage,
	path string,
) {
	t.Helper()

	structType = deref(structType)
	if structType.Kind() != reflect.Struct {
		return
	}

	for i := range structType.NumField() {
		field := structType.Field(i)
		if !field.IsExported() {
			continue
		}

		name, optional, ok := jsonKey(field)
		if !ok {
			continue
		}

		value, present := raw[name]
		if !present {
			if !optional {
				t.Errorf(
					"%s.%s (json %q) is absent from the fixture, so it would "+
						"decode to a zero value: either the platform emits it "+
						"and the fixture is incomplete, or it never populates "+
						"and does not belong on the struct",
					structType.Name(),
					field.Name,
					path+name,
				)
			}
			continue
		}

		assertRequiredKeysIn(t, field.Type, value, path+name+".")
	}
}

// assertRequiredKeysIn descends into a present JSON value, applying
// assertRequiredKeys to any struct it contains (directly or through a slice).
func assertRequiredKeysIn(
	t *testing.T,
	fieldType reflect.Type,
	value json.RawMessage,
	path string,
) {
	t.Helper()

	fieldType = deref(fieldType)

	switch fieldType.Kind() {
	case reflect.Struct:
		var nested map[string]json.RawMessage
		if err := json.Unmarshal(value, &nested); err != nil {
			return
		}
		assertRequiredKeys(t, fieldType, nested, path)
	case reflect.Slice:
		elem := deref(fieldType.Elem())
		if elem.Kind() != reflect.Struct {
			return
		}
		var items []json.RawMessage
		if err := json.Unmarshal(value, &items); err != nil {
			return
		}
		for _, item := range items {
			assertRequiredKeysIn(t, elem, item, path)
		}
	default:
	}
}

func deref(t reflect.Type) reflect.Type {
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t
}

// jsonKey returns the wire name for a field, whether it is optional, and
// whether it is serialized at all.
//
// This must mirror encoding/json's own binding rules, not a convenient subset
// of them: any field the guard skips but encoding/json still binds is a field
// the guard silently stops protecting. encoding/json falls back to the Go field
// name when a field has no json tag or a tag that supplies only options
// ("`json:\",omitempty\"`"), so this does too. Only an explicit "-" means the
// field is genuinely not serialized.
func jsonKey(field reflect.StructField) (name string, optional, ok bool) {
	parts := strings.Split(field.Tag.Get("json"), ",")

	name = parts[0]
	if name == "-" && len(parts) == 1 {
		return "", false, false
	}

	for _, opt := range parts[1:] {
		if opt == "omitempty" {
			optional = true
		}
	}

	if name == "" {
		name = field.Name
	}

	return name, optional, true
}

// TestJSONKey pins jsonKey to encoding/json's binding rules. The untagged and
// options-only cases are the ones that matter: no type in this package has such
// a field today, and this test is what keeps the guard correct if one appears.
func TestJSONKey(t *testing.T) {
	type sample struct {
		Tagged     string `json:"tagged"`
		Optional   string `json:"optional,omitempty"`
		Untagged   string
		OnlyOption string `json:",omitempty"`
		Skipped    string `json:"-"`
	}

	tests := []struct {
		field    string
		wantName string
		wantOpt  bool
		wantOK   bool
	}{
		{field: "Tagged", wantName: "tagged", wantOK: true},
		{field: "Optional", wantName: "optional", wantOpt: true, wantOK: true},
		{field: "Untagged", wantName: "Untagged", wantOK: true},
		{
			field:    "OnlyOption",
			wantName: "OnlyOption",
			wantOpt:  true,
			wantOK:   true,
		},
		{field: "Skipped"},
	}

	typ := reflect.TypeOf(sample{})
	for _, tt := range tests {
		t.Run(
			tt.field, func(t *testing.T) {
				field, found := typ.FieldByName(tt.field)
				if !found {
					t.Fatalf("no field %q on sample", tt.field)
				}

				name, optional, ok := jsonKey(field)
				if ok != tt.wantOK {
					t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
				}
				if name != tt.wantName {
					t.Errorf("name = %q, want %q", name, tt.wantName)
				}
				if optional != tt.wantOpt {
					t.Errorf("optional = %v, want %v", optional, tt.wantOpt)
				}
			},
		)
	}
}
