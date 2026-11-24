package webhook_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dakota-xyz/go-sdk/webhook"
)

// Integrators may ship handler logs to third parties, so a rejected delivery
// is logged with the reason only: never the body, a signature header, the
// timestamp header or any fragment of them.
func TestHandler_RejectedDeliveryLogsNoRequestContent(t *testing.T) {
	key, stranger := newRotationKey(t), newRotationKey(t)
	const validTS = "" // replaced by the current time

	cases := []struct {
		name string
		ts   string
		body string
		// signer signs the delivery; stranger's signature fails verification.
		signer rotationKey
		want   int
		// secrets must not appear in any log line.
		secrets []string
	}{
		{
			name:    "untrusted signature",
			body:    `{"id":"evt_1","type":"customer.created","data":{"object":{"ssn":"SECRET-111"}}}`,
			signer:  stranger,
			want:    http.StatusUnauthorized,
			secrets: []string{"SECRET-111"},
		},
		{
			name:    "malformed JSON",
			body:    `{"id":"evt_1","type":"customer.created","x":Q}`,
			signer:  key,
			want:    http.StatusBadRequest,
			secrets: []string{"'Q'"},
		},
		{
			name:    "number out of range",
			body:    `{"id":"evt_1","type":"customer.created","created":98765432109876543210987}`,
			signer:  key,
			want:    http.StatusBadRequest,
			secrets: []string{"98765432109876543210987"},
		},
		{
			name:    "malformed timestamp",
			ts:      "1700000000x",
			body:    `{"id":"evt_1","type":"customer.created","data":{}}`,
			signer:  key,
			want:    http.StatusUnauthorized,
			secrets: []string{"1700000000x"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			logOpt, logs := recordLogs()
			handler, err := webhook.NewHandler(webhook.WithPublicKeys(key.hex), logOpt)
			if err != nil {
				t.Fatalf("NewHandler: %v", err)
			}

			var req *http.Request
			if c.ts == validTS {
				req = rotationRequest(t, c.body, &c.signer, c.signer)
			} else {
				req = httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(c.body))
				req.Header.Set(webhook.TimestampHeader, c.ts)
				req.Header.Set(webhook.SignatureHeader,
					webhook.ComputeSignature(c.ts, []byte(c.body), c.signer.priv))
				req.Header.Set(webhook.SignaturesHeader, c.signer.entry(c.ts, []byte(c.body)))
			}
			secrets := append([]string{
				c.body,
				req.Header.Get(webhook.SignatureHeader),
				req.Header.Get(webhook.SignaturesHeader),
			}, c.secrets...)

			if got := serve(t, handler, req); got != c.want {
				t.Fatalf("got status %d, want %d", got, c.want)
			}
			records := logs()
			if len(records) == 0 {
				t.Fatal("rejected delivery logged nothing")
			}
			for _, r := range records {
				for _, s := range secrets {
					if strings.Contains(r.text, s) {
						t.Errorf("log line %q contains request content %q", r.text, s)
					}
				}
			}
		})
	}
}
