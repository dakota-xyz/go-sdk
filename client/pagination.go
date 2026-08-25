package client

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	sdkerrors "github.com/dakota-xyz/go-sdk/errors"

	"github.com/dakota-xyz/go-sdk/client/gen"
)

// Page is a typed cursor page returned by list endpoints.
type Page[T any] struct {
	Items []T
	Meta  gen.Meta
}

// PageFetcher fetches one page using cursor pagination.
type PageFetcher[T any] func(
	ctx context.Context,
	startingAfter *gen.StartingAfterParam,
	limit *gen.LimitParam,
) (Page[T], error)

// CursorExtractor extracts a cursor value from an item.
type CursorExtractor[T any] func(item T) (string, bool)

// StreamResult is one async item or terminal iterator error.
type StreamResult[T any] struct {
	Item T
	Err  error
}

// Iterator provides pull-based cursor pagination.
type Iterator[T any] struct {
	fetcher   PageFetcher[T]
	extractor CursorExtractor[T]
	cursor    *gen.StartingAfterParam
	limit     *gen.LimitParam
	items     []T
	itemIndex int
	hasMore   bool
	started   bool
	completed bool
	mu        sync.Mutex
}

const defaultIteratorStreamSendTimeout = 30 * time.Second

var iteratorStreamSendTimeout = defaultIteratorStreamSendTimeout

// NewIterator constructs a cursor iterator.
func NewIterator[T any](
	fetcher PageFetcher[T],
	initialCursor *gen.StartingAfterParam,
	limit *gen.LimitParam,
	extractor CursorExtractor[T],
) *Iterator[T] {
	if extractor == nil {
		extractor = DefaultCursorExtractor[T]
	}
	return &Iterator[T]{
		fetcher:   fetcher,
		extractor: extractor,
		cursor:    cloneStartingAfter(initialCursor),
		limit:     cloneLimit(limit),
	}
}

// Next returns the next item from the iterator.
//
// Next is safe for concurrent use by multiple goroutines.
func (it *Iterator[T]) Next(ctx context.Context) (T, bool, error) {
	it.mu.Lock()
	defer it.mu.Unlock()

	var zero T
	if it.fetcher == nil {
		return zero, false, sdkerrors.New(
			sdkerrors.CodeInvalidConfig,
			"iterator fetcher is required",
		)
	}
	if it.completed {
		return zero, false, nil
	}

	for {
		if it.itemIndex < len(it.items) {
			item := it.items[it.itemIndex]
			it.itemIndex++
			return item, true, nil
		}

		if it.started && !it.hasMore {
			it.completed = true
			return zero, false, nil
		}

		page, err := it.fetcher(ctx, cloneStartingAfter(it.cursor), cloneLimit(it.limit))
		if err != nil {
			return zero, false, err
		}

		it.started = true
		it.items = page.Items
		it.itemIndex = 0
		it.hasMore = page.Meta.HasMoreAfter

		if len(page.Items) == 0 {
			if !it.hasMore {
				it.completed = true
				return zero, false, nil
			}
			return zero, false, sdkerrors.New(
				sdkerrors.CodeInternal,
				"pagination cursor could not advance: empty page with has_more_after=true",
			)
		}

		if it.hasMore {
			last := page.Items[len(page.Items)-1]
			nextCursor, ok := it.extractor(last)
			if !ok || nextCursor == "" {
				return zero, false, sdkerrors.New(
					sdkerrors.CodeInternal,
					"pagination cursor extraction failed for last item",
				)
			}
			cursor := gen.StartingAfterParam(nextCursor)
			it.cursor = &cursor
		}
	}
}

// Stream emits iterator results on a channel until completion or error.
//
// If the consumer stops reading, callers should cancel the context to stop the
// internal goroutine immediately. As a safety net, blocked sends are abandoned
// after an internal timeout to avoid goroutine leaks.
func (it *Iterator[T]) Stream(
	ctx context.Context,
	buffer int,
) <-chan StreamResult[T] {
	if buffer < 0 {
		buffer = 0
	}
	out := make(chan StreamResult[T], buffer)

	go func() {
		defer close(out)
		for {
			item, ok, err := it.Next(ctx)
			if err != nil {
				if !sendStreamResult(ctx, out, StreamResult[T]{Err: err}) {
					return
				}
			}
			if !ok {
				return
			}
			if !sendStreamResult(ctx, out, StreamResult[T]{Item: item}) {
				return
			}
		}
	}()

	return out
}

func sendStreamResult[T any](
	ctx context.Context,
	out chan<- StreamResult[T],
	result StreamResult[T],
) bool {
	timeout := iteratorStreamSendTimeout
	if timeout <= 0 {
		select {
		case <-ctx.Done():
			return false
		case out <- result:
			return true
		}
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case out <- result:
		return true
	case <-timer.C:
		return false
	}
}

// DefaultCursorExtractor extracts common cursor fields (`id`, `application_id`,
// `event_id`) from generated models.
func DefaultCursorExtractor[T any](item T) (string, bool) {
	return extractCursor(item,
		"Id",
		"ID",
		"ApplicationId",
		"ApplicationID",
		"EventId",
		"EventID",
	)
}

func extractCursor(item any, fieldNames ...string) (string, bool) {
	v := reflect.ValueOf(item)
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return "", false
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return "", false
	}

	for _, fieldName := range fieldNames {
		f := v.FieldByName(fieldName)
		if !f.IsValid() {
			continue
		}
		for f.Kind() == reflect.Pointer {
			if f.IsNil() {
				break
			}
			f = f.Elem()
		}
		if f.Kind() == reflect.String {
			cursor := f.String()
			if cursor != "" {
				return cursor, true
			}
		}
	}

	return "", false
}

func cloneStartingAfter(
	v *gen.StartingAfterParam,
) *gen.StartingAfterParam {
	if v == nil {
		return nil
	}
	copyValue := *v
	return &copyValue
}

func cloneLimit(v *gen.LimitParam) *gen.LimitParam {
	if v == nil {
		return nil
	}
	copyValue := *v
	return &copyValue
}

// ApplicationsIterator returns a cursor iterator over applications.
func (c *Client) ApplicationsIterator(
	params *gen.ListApplicationsParams,
) *Iterator[gen.ApplicationListItem] {
	baseParams := gen.ListApplicationsParams{}
	if params != nil {
		baseParams = *params
	}

	return NewIterator(
		func(ctx context.Context, cursor *gen.StartingAfterParam, limit *gen.LimitParam) (Page[gen.ApplicationListItem], error) {
			p := baseParams
			p.StartingAfter = cloneStartingAfter(cursor)
			p.Limit = cloneLimit(limit)

			resp, err := CheckResponse(c.api.ListApplicationsWithResponse(ctx, &p))
			if err != nil {
				return Page[gen.ApplicationListItem]{}, err
			}
			if resp.JSON200 == nil {
				return Page[gen.ApplicationListItem]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					"list applications: missing success payload",
				)
			}
			return Page[gen.ApplicationListItem]{
				Items: resp.JSON200.Data,
				Meta:  resp.JSON200.Meta,
			}, nil
		},
		cloneStartingAfter(baseParams.StartingAfter),
		cloneLimit(baseParams.Limit),
		func(item gen.ApplicationListItem) (string, bool) {
			if item.ApplicationId == "" {
				return "", false
			}
			return item.ApplicationId, true
		},
	)
}

// CustomersIterator returns a cursor iterator over customers.
func (c *Client) CustomersIterator(
	params *gen.ListCustomersParams,
) *Iterator[gen.Customer] {
	baseParams := gen.ListCustomersParams{}
	if params != nil {
		baseParams = *params
	}

	return NewIterator(
		func(ctx context.Context, cursor *gen.StartingAfterParam, limit *gen.LimitParam) (Page[gen.Customer], error) {
			p := baseParams
			p.StartingAfter = cloneStartingAfter(cursor)
			p.Limit = cloneLimit(limit)

			resp, err := CheckResponse(c.api.ListCustomersWithResponse(ctx, &p))
			if err != nil {
				return Page[gen.Customer]{}, err
			}
			if resp.JSON200 == nil {
				return Page[gen.Customer]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					"list customers: missing success payload",
				)
			}
			return Page[gen.Customer]{
				Items: resp.JSON200.Data,
				Meta:  resp.JSON200.Meta,
			}, nil
		},
		cloneStartingAfter(baseParams.StartingAfter),
		cloneLimit(baseParams.Limit),
		nil,
	)
}

// OneOffTransactionsIterator returns a cursor iterator over one-off transactions.
func (c *Client) OneOffTransactionsIterator(
	params *gen.ListTransactionsParams,
) *Iterator[gen.OneOffTransaction] {
	baseParams := gen.ListTransactionsParams{}
	if params != nil {
		baseParams = *params
	}

	return NewIterator(
		func(ctx context.Context, cursor *gen.StartingAfterParam, limit *gen.LimitParam) (Page[gen.OneOffTransaction], error) {
			p := baseParams
			p.StartingAfter = cloneStartingAfter(cursor)
			p.Limit = cloneLimit(limit)

			// GET /transactions serves three resource families from one path.
			// With transaction_type omitted the server INFERS the family from
			// the other filters, and customer_id alone infers auto_account —
			// so the obvious call for "this customer's one-off transactions"
			// would come back as their AUTO-ACCOUNT transactions, unmarshalled
			// into OneOffTransaction without complaint. This iterator is typed
			// to one family, so it always names that family rather than
			// letting the request be inferred into a different one.
			switch {
			case p.TransactionType == nil:
				oneOff := gen.TransactionResourceTypeOneOff
				p.TransactionType = &oneOff
			case *p.TransactionType != gen.TransactionResourceTypeOneOff:
				return Page[gen.OneOffTransaction]{}, sdkerrors.New(
					sdkerrors.CodeInvalidConfig,
					fmt.Sprintf(
						"list transactions: OneOffTransactionsIterator yields one-off transactions, but TransactionType is %q; "+
							"reach that family through Raw().ListTransactionsWithResponse",
						*p.TransactionType,
					),
				)
			}

			resp, err := CheckResponse(
				c.api.ListTransactionsWithResponse(ctx, &p),
			)
			if err != nil {
				return Page[gen.OneOffTransaction]{}, err
			}
			if resp.JSON200 == nil {
				return Page[gen.OneOffTransaction]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					"list transactions: missing success payload",
				)
			}
			// Parse union type as one-off transactions
			oneOffResp, err := resp.JSON200.AsPaginatedOneOffTransactionResponse()
			if err != nil {
				return Page[gen.OneOffTransaction]{}, sdkerrors.Wrap(
					sdkerrors.CodeInternal,
					"list transactions: failed to parse as one-off transactions",
					err,
				)
			}
			// The response names the family it actually served. Pinning the
			// request above should make a mismatch unreachable; check it
			// anyway, because the failure it guards is silent — rows of another
			// family unmarshal into OneOffTransaction with zeroed fields rather
			// than erroring, and a caller cannot tell from the result.
			//
			// Only a POSITIVE mismatch is an error. An empty value means the
			// response did not say, which is not evidence of the wrong family:
			// treating it as one would turn this backstop into a new failure
			// mode the moment a server omits the field.
			if oneOffResp.Meta.TransactionType != "" &&
				oneOffResp.Meta.TransactionType != gen.TransactionResourceTypeOneOff {
				return Page[gen.OneOffTransaction]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					fmt.Sprintf(
						"list transactions: requested the one_off family but the response reports %q",
						oneOffResp.Meta.TransactionType,
					),
				)
			}

			// Field-by-field, not whole-struct: transaction lists carry their
			// own gen.TransactionListMeta, which shares these three fields with
			// gen.Meta but is a distinct type and assignable to neither.
			//
			// TransactionType is consumed by the check above rather than
			// forwarded. Page is generic over five iterators — applications,
			// customers, transactions, recipients, events — and a transaction
			// family means nothing to the other four, so putting it on Page
			// needs a decision about how a caller reaches page metadata at
			// all: Iterator exposes only Next and Stream and never hands back
			// a Page.
			//
			// See TestPaginationMetaFieldGuard, which fails loudly if gen.Meta
			// grows a field this copy would silently drop.
			return Page[gen.OneOffTransaction]{
				Items: oneOffResp.Data,
				Meta: gen.Meta{
					TotalCount:    oneOffResp.Meta.TotalCount,
					HasMoreAfter:  oneOffResp.Meta.HasMoreAfter,
					HasMoreBefore: oneOffResp.Meta.HasMoreBefore,
				},
			}, nil
		},
		cloneStartingAfter(baseParams.StartingAfter),
		cloneLimit(baseParams.Limit),
		nil,
	)
}

// RecipientsIterator returns a cursor iterator over recipients.
func (c *Client) RecipientsIterator(
	customerID gen.KSUID,
	params *gen.ListRecipientsParams,
) *Iterator[gen.RecipientResponse] {
	baseParams := gen.ListRecipientsParams{}
	if params != nil {
		baseParams = *params
	}

	return NewIterator(
		func(ctx context.Context, cursor *gen.StartingAfterParam, limit *gen.LimitParam) (Page[gen.RecipientResponse], error) {
			p := baseParams
			p.StartingAfter = cloneStartingAfter(cursor)
			p.Limit = cloneLimit(limit)

			resp, err := CheckResponse(
				c.api.ListRecipientsWithResponse(ctx, customerID, &p),
			)
			if err != nil {
				return Page[gen.RecipientResponse]{}, err
			}
			if resp.JSON200 == nil {
				return Page[gen.RecipientResponse]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					"list recipients: missing success payload",
				)
			}
			return Page[gen.RecipientResponse]{
				Items: resp.JSON200.Data,
				Meta:  resp.JSON200.Meta,
			}, nil
		},
		cloneStartingAfter(baseParams.StartingAfter),
		cloneLimit(baseParams.Limit),
		nil,
	)
}

// EventsIterator returns a cursor iterator over events.
func (c *Client) EventsIterator(
	params *gen.ListEventsParams,
) *Iterator[gen.Event] {
	baseParams := gen.ListEventsParams{}
	if params != nil {
		baseParams = *params
	}

	return NewIterator(
		func(ctx context.Context, cursor *gen.StartingAfterParam, limit *gen.LimitParam) (Page[gen.Event], error) {
			p := baseParams
			p.StartingAfter = cloneStartingAfter(cursor)
			p.Limit = cloneLimit(limit)

			resp, err := CheckResponse(c.api.ListEventsWithResponse(ctx, &p))
			if err != nil {
				return Page[gen.Event]{}, err
			}
			if resp.JSON200 == nil {
				return Page[gen.Event]{}, sdkerrors.New(
					sdkerrors.CodeInternal,
					"list events: missing success payload",
				)
			}
			return Page[gen.Event]{
				Items: resp.JSON200.Data,
				Meta:  resp.JSON200.Meta,
			}, nil
		},
		cloneStartingAfter(baseParams.StartingAfter),
		cloneLimit(baseParams.Limit),
		func(item gen.Event) (string, bool) {
			if item.Id == "" {
				return "", false
			}
			return string(item.Id), true
		},
	)
}

// Collect consumes an iterator into memory. Intended for low-volume endpoints.
func Collect[T any](ctx context.Context, it *Iterator[T]) ([]T, error) {
	if it == nil {
		return nil, fmt.Errorf("iterator is nil")
	}
	items := make([]T, 0)
	for {
		item, ok, err := it.Next(ctx)
		if err != nil {
			return nil, err
		}
		if !ok {
			return items, nil
		}
		items = append(items, item)
	}
}
