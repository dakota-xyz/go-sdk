package client

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	sdkerrors "github.com/dakota-xyz/go-sdk/errors"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

type testItem struct {
	Id string
}

func TestIterator_NextTraversesPages(t *testing.T) {
	fetcher := func(
		_ context.Context,
		startingAfter *gen.StartingAfterParam,
		_ *gen.LimitParam,
	) (Page[testItem], error) {
		if startingAfter == nil {
			return Page[testItem]{
				Items: []testItem{{Id: "id_1"}, {Id: "id_2"}},
				Meta:  gen.Meta{HasMoreAfter: true},
			}, nil
		}
		if string(*startingAfter) == "id_2" {
			return Page[testItem]{
				Items: []testItem{{Id: "id_3"}},
				Meta:  gen.Meta{HasMoreAfter: false},
			}, nil
		}
		t.Fatalf("unexpected cursor: %q", string(*startingAfter))
		return Page[testItem]{}, nil
	}

	it := NewIterator(fetcher, nil, nil, nil)
	got := make([]string, 0, 3)
	for {
		item, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if !ok {
			break
		}
		got = append(got, item.Id)
	}

	if len(got) != 3 {
		t.Fatalf("len(got) = %d, want 3", len(got))
	}
	if got[0] != "id_1" || got[1] != "id_2" || got[2] != "id_3" {
		t.Fatalf("unexpected order: %#v", got)
	}
}

func TestIterator_ErrorsWhenCursorMissing(t *testing.T) {
	type noCursorItem struct {
		Name string
	}

	it := NewIterator(
		func(_ context.Context, _ *gen.StartingAfterParam, _ *gen.LimitParam) (Page[noCursorItem], error) {
			return Page[noCursorItem]{
				Items: []noCursorItem{{Name: "item_1"}},
				Meta:  gen.Meta{HasMoreAfter: true},
			}, nil
		},
		nil,
		nil,
		nil,
	)

	_, _, err := it.Next(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIterator_NilFetcher(t *testing.T) {
	it := NewIterator[testItem](nil, nil, nil, nil)
	_, _, err := it.Next(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestIterator_Stream(t *testing.T) {
	limit := gen.LimitParam(2)
	it := NewIterator(
		func(_ context.Context, startingAfter *gen.StartingAfterParam, _ *gen.LimitParam) (Page[testItem], error) {
			if startingAfter == nil {
				return Page[testItem]{
					Items: []testItem{{Id: "a"}, {Id: "b"}},
					Meta:  gen.Meta{HasMoreAfter: false},
				}, nil
			}
			t.Fatalf("unexpected cursor: %q", string(*startingAfter))
			return Page[testItem]{}, nil
		},
		nil,
		&limit,
		nil,
	)

	ch := it.Stream(context.Background(), 1)
	items := make([]string, 0, 2)
	for result := range ch {
		if result.Err != nil {
			t.Fatalf("stream error: %v", result.Err)
		}
		items = append(items, result.Item.Id)
	}

	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
	if items[0] != "a" || items[1] != "b" {
		t.Fatalf("unexpected stream items: %#v", items)
	}
}

func TestIterator_StreamStopsOnContextCancel(t *testing.T) {
	it := NewIterator(
		func(_ context.Context, _ *gen.StartingAfterParam, _ *gen.LimitParam) (Page[testItem], error) {
			return Page[testItem]{
				Items: []testItem{{Id: "a"}, {Id: "b"}, {Id: "c"}},
				Meta:  gen.Meta{HasMoreAfter: false},
			}, nil
		},
		nil,
		nil,
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())
	ch := it.Stream(ctx, 0)

	first, ok := <-ch
	if !ok {
		t.Fatal("expected first stream item")
	}
	if first.Err != nil {
		t.Fatalf("unexpected stream error: %v", first.Err)
	}

	cancel()
	select {
	case _, open := <-ch:
		if open {
			// It can still deliver one in-flight value before closing.
			for range ch {
			}
		}
	case <-time.After(time.Second):
		t.Fatal("stream did not stop after context cancellation")
	}
}

func TestIterator_StreamBlockedConsumerTimeout(t *testing.T) {
	oldTimeout := iteratorStreamSendTimeout
	iteratorStreamSendTimeout = 20 * time.Millisecond
	defer func() { iteratorStreamSendTimeout = oldTimeout }()

	it := NewIterator(
		func(_ context.Context, _ *gen.StartingAfterParam, _ *gen.LimitParam) (Page[testItem], error) {
			return Page[testItem]{
				Items: []testItem{{Id: "a"}},
				Meta:  gen.Meta{HasMoreAfter: false},
			}, nil
		},
		nil,
		nil,
		nil,
	)

	ch := it.Stream(context.Background(), 0)
	time.Sleep(80 * time.Millisecond)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel after blocked send timeout")
		}
	case <-time.After(time.Second):
		t.Fatal("stream goroutine did not exit on blocked send timeout")
	}
}

func TestIterator_CustomCursorExtractor(t *testing.T) {
	type customItem struct {
		Cursor string
		Value  string
	}

	fetcher := func(
		_ context.Context,
		startingAfter *gen.StartingAfterParam,
		_ *gen.LimitParam,
	) (Page[customItem], error) {
		if startingAfter == nil {
			return Page[customItem]{
				Items: []customItem{{Cursor: "c1", Value: "v1"}, {Cursor: "c2", Value: "v2"}},
				Meta:  gen.Meta{HasMoreAfter: true},
			}, nil
		}
		if string(*startingAfter) == "c2" {
			return Page[customItem]{
				Items: []customItem{{Cursor: "c3", Value: "v3"}},
				Meta:  gen.Meta{HasMoreAfter: false},
			}, nil
		}
		t.Fatalf("unexpected cursor: %q", string(*startingAfter))
		return Page[customItem]{}, nil
	}

	it := NewIterator(
		fetcher,
		nil,
		nil,
		func(item customItem) (string, bool) { return item.Cursor, item.Cursor != "" },
	)

	values := make([]string, 0, 3)
	for {
		item, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if !ok {
			break
		}
		values = append(values, item.Value)
	}

	if len(values) != 3 {
		t.Fatalf("len(values) = %d, want 3", len(values))
	}
}

func TestIterator_ConcurrentNext(t *testing.T) {
	items := make([]testItem, 200)
	for i := range items {
		items[i] = testItem{Id: fmt.Sprintf("item_%d", i)}
	}
	fetchCalls := 0
	it := NewIterator(
		func(_ context.Context, _ *gen.StartingAfterParam, _ *gen.LimitParam) (Page[testItem], error) {
			fetchCalls++
			return Page[testItem]{
				Items: items,
				Meta:  gen.Meta{HasMoreAfter: false},
			}, nil
		},
		nil,
		nil,
		nil,
	)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		seen  = make(map[string]struct{}, len(items))
		errCh = make(chan error, 1)
	)

	worker := func() {
		defer wg.Done()
		for {
			item, ok, err := it.Next(context.Background())
			if err != nil {
				select {
				case errCh <- err:
				default:
				}
				return
			}
			if !ok {
				return
			}
			mu.Lock()
			seen[item.Id] = struct{}{}
			mu.Unlock()
		}
	}

	for i := 0; i < 8; i++ {
		wg.Add(1)
		go worker()
	}
	wg.Wait()

	select {
	case err := <-errCh:
		t.Fatalf("unexpected error: %v", err)
	default:
	}

	if len(seen) != len(items) {
		t.Fatalf("seen = %d, want %d", len(seen), len(items))
	}
	if fetchCalls != 1 {
		t.Fatalf("fetchCalls = %d, want 1", fetchCalls)
	}
}

func TestCollect_NilIterator(t *testing.T) {
	_, err := Collect[testItem](context.Background(), nil)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCollect_Success(t *testing.T) {
	it := NewIterator(
		func(_ context.Context, _ *gen.StartingAfterParam, _ *gen.LimitParam) (Page[testItem], error) {
			return Page[testItem]{Items: []testItem{{Id: "a"}, {Id: "b"}}, Meta: gen.Meta{HasMoreAfter: false}}, nil
		},
		nil,
		nil,
		nil,
	)

	items, err := Collect(context.Background(), it)
	if err != nil {
		t.Fatalf("Collect error: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("len(items) = %d, want 2", len(items))
	}
}

// TestPaginationMetaFieldGuard pins the field set of gen.Meta.
//
// OneOffTransactionsIterator fills Page[T].Meta field-by-field (see
// pagination.go) rather than by whole-struct assignment, because the upstream
// response type it reads from is gen.TransactionListMeta — a distinct struct
// carrying gen.Meta's three fields plus its own TransactionType. A
// field-by-field copy has no compiler
// check that it stays exhaustive: if gen.Meta grows a field, the copy silently
// keeps dropping it. This guard fails loudly instead, the next time the spec
// is synced and gen.Meta's shape changes.
//
// Note this pins the DESTINATION type. TransactionType exists only on the
// source and is dropped deliberately, so adding it upstream will not fail
// this test.
func TestPaginationMetaFieldGuard(t *testing.T) {
	typ := reflect.TypeOf(gen.Meta{})

	wantFields := []string{"HasMoreAfter", "HasMoreBefore", "TotalCount"}
	if got := typ.NumField(); got != len(wantFields) {
		t.Fatalf("gen.Meta has %d fields, want %d (%v) — update the field-by-field "+
			"copy in OneOffTransactionsIterator (pagination.go) to match, then update this guard",
			got, len(wantFields), wantFields)
	}
	for _, name := range wantFields {
		if _, ok := typ.FieldByName(name); !ok {
			t.Fatalf("gen.Meta is missing expected field %q", name)
		}
	}
}

// TestOneOffTransactionsIterator_TraversesTwoPages proves the field-by-field
// Meta copy in OneOffTransactionsIterator preserves has_more_after, so the
// iterator still advances to and terminates on a second page. This is the
// one field-transposition mistake (has_more_after <-> has_more_before) the
// compiler cannot catch, since both are plain bools.
func TestOneOffTransactionsIterator_TraversesTwoPages(t *testing.T) {
	requestCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/transactions" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		if got := r.URL.Query().Get("transaction_type"); got != "one_off" {
			t.Fatalf("transaction_type = %q, want %q on every page", got, "one_off")
		}
		requestCount++
		w.Header().Set("Content-Type", "application/json")

		switch startingAfter := r.URL.Query().Get("starting_after"); startingAfter {
		case "":
			_, _ = w.Write([]byte(`{
				"data": [
					{"id": "txn_1"},
					{"id": "txn_2"}
				],
				"meta": {"total_count": 3, "has_more_after": true, "has_more_before": false, "transaction_type": "one_off"}
			}`))
		case "txn_2":
			_, _ = w.Write([]byte(`{
				"data": [
					{"id": "txn_3"}
				],
				"meta": {"total_count": 3, "has_more_after": false, "has_more_before": true, "transaction_type": "one_off"}
			}`))
		default:
			t.Fatalf("unexpected starting_after: %q", startingAfter)
		}
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.OneOffTransactionsIterator(nil)
	var got []string
	for {
		item, ok, err := it.Next(context.Background())
		if err != nil {
			t.Fatalf("Next error: %v", err)
		}
		if !ok {
			break
		}
		got = append(got, item.Id)
	}

	want := []string{"txn_1", "txn_2", "txn_3"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	if requestCount != 2 {
		t.Fatalf("requestCount = %d, want 2 (one per page)", requestCount)
	}
}

// TestOneOffTransactionsIterator_NamesFamilyWhenFilteringByCustomer pins the
// fix for the family-inference trap.
//
// GET /transactions serves three resource families. With transaction_type
// omitted the server infers one from the other filters, and customer_id ALONE
// infers auto_account. So the obvious spelling of "this customer's one-off
// transactions" used to return that customer's AUTO-ACCOUNT transactions,
// which unmarshal into OneOffTransaction with zeroed fields and no error --
// the caller could not tell. The iterator is typed to one family, so it must
// name that family on the wire rather than let the request be inferred.
func TestOneOffTransactionsIterator_NamesFamilyWhenFilteringByCustomer(t *testing.T) {
	var gotType, gotCustomer string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotType = r.URL.Query().Get("transaction_type")
		gotCustomer = r.URL.Query().Get("customer_id")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [],
			"meta": {"total_count": 0, "has_more_after": false, "has_more_before": false, "transaction_type": "one_off"}
		}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	customer := gen.KSUID("2B5J8KZ9N7M1K3P6Q8R4T7V9")
	it := c.OneOffTransactionsIterator(&gen.ListTransactionsParams{CustomerId: &customer})
	if _, _, err := it.Next(context.Background()); err != nil {
		t.Fatalf("Next error: %v", err)
	}

	if gotType != "one_off" {
		t.Fatalf("transaction_type = %q, want %q -- the server would otherwise infer auto_account from customer_id alone", gotType, "one_off")
	}
	if gotCustomer != string(customer) {
		t.Fatalf("customer_id = %q, want %q", gotCustomer, string(customer))
	}
}

// TestOneOffTransactionsIterator_RejectsAnotherFamily: asking a one-off-typed
// iterator for a different family is incoherent, so say so rather than
// silently overriding the caller's explicit choice.
func TestOneOffTransactionsIterator_RejectsAnotherFamily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("no request should reach the server")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	wallet := gen.TransactionResourceTypeWallet
	it := c.OneOffTransactionsIterator(&gen.ListTransactionsParams{TransactionType: &wallet})
	_, _, err = it.Next(context.Background())
	if err == nil {
		t.Fatal("want an error for a mismatched TransactionType, got nil")
	}
	if !strings.Contains(err.Error(), "wallet") {
		t.Fatalf("error should name the offending family, got: %v", err)
	}
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("want a *sdkerrors.Error, got %T", err)
	}
	if sdkErr.Code != sdkerrors.CodeInvalidConfig {
		t.Fatalf("code = %q, want %q — a caller misconfiguration, not a server fault", sdkErr.Code, sdkerrors.CodeInvalidConfig)
	}
}

// TestOneOffTransactionsIterator_RejectsMismatchedResponseFamily: pinning the
// request should make this unreachable, but the failure it guards is silent,
// so the iterator verifies what the response says it served.
func TestOneOffTransactionsIterator_RejectsMismatchedResponseFamily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{"id": "txn_1"}],
			"meta": {"total_count": 1, "has_more_after": false, "has_more_before": false, "transaction_type": "auto_account"}
		}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.OneOffTransactionsIterator(nil)
	_, _, err = it.Next(context.Background())
	if err == nil {
		t.Fatal("want an error when the response reports another family, got nil")
	}
	if !strings.Contains(err.Error(), "auto_account") {
		t.Fatalf("error should name the family actually served, got: %v", err)
	}
	var sdkErr *sdkerrors.Error
	if !errors.As(err, &sdkErr) {
		t.Fatalf("want a *sdkerrors.Error, got %T", err)
	}
	if sdkErr.Code != sdkerrors.CodeInternal {
		t.Fatalf("code = %q, want %q — the server routed against an explicit request", sdkErr.Code, sdkerrors.CodeInternal)
	}
}

// TestOneOffTransactionsIterator_ToleratesUnnamedResponseFamily: an empty
// transaction_type means the response did not say which family it served,
// which is not evidence that it served the wrong one. The request is already
// pinned to one_off, so treating silence as a mismatch would only invent a
// failure mode for servers that omit the field.
func TestOneOffTransactionsIterator_ToleratesUnnamedResponseFamily(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [{"id": "txn_1"}],
			"meta": {"total_count": 1, "has_more_after": false, "has_more_before": false}
		}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	it := c.OneOffTransactionsIterator(nil)
	item, ok, err := it.Next(context.Background())
	if err != nil {
		t.Fatalf("an unnamed family should not be an error: %v", err)
	}
	if !ok || item.Id != "txn_1" {
		t.Fatalf("got (%v, %v), want txn_1", item.Id, ok)
	}
}

// TestOneOffTransactionsIterator_DoesNotMutateCallerParams pins the guarantee
// OneOffTransactionsIterator's godoc makes. The iterator sets TransactionType
// on its own copy of the params for every fetch; a future edit that wrote
// through the caller's pointer instead would be invisible until someone reused
// a params struct across iterators.
func TestOneOffTransactionsIterator_DoesNotMutateCallerParams(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"data": [],
			"meta": {"total_count": 0, "has_more_after": false, "has_more_before": false, "transaction_type": "one_off"}
		}`))
	}))
	defer srv.Close()

	c, err := New(WithBaseURL(srv.URL), WithAPIKey("test_key"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	customer := gen.KSUID("2B5J8KZ9N7M1K3P6Q8R4T7V9")
	params := &gen.ListTransactionsParams{CustomerId: &customer}

	it := c.OneOffTransactionsIterator(params)
	if _, _, err := it.Next(context.Background()); err != nil {
		t.Fatalf("Next error: %v", err)
	}

	if params.TransactionType != nil {
		t.Fatalf("iterator wrote TransactionType=%q back into the caller's params", *params.TransactionType)
	}
	if params.StartingAfter != nil {
		t.Fatalf("iterator wrote StartingAfter back into the caller's params")
	}
	if params.CustomerId != &customer {
		t.Fatal("iterator replaced the caller's CustomerId pointer")
	}
}
