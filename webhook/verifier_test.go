package webhook_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/dakota-xyz/go-sdk/errors"
	"github.com/dakota-xyz/go-sdk/webhook"
)

// rotationKey is one Dakota signing key in a rotation test.
type rotationKey struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	hex  string
	id   string
}

func newRotationKey(t *testing.T) rotationKey {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("failed to generate key pair: %v", err)
	}
	h := hex.EncodeToString(pub)
	return rotationKey{pub: pub, priv: priv, hex: h, id: h[:8]}
}

// entry is one X-Dakota-Webhook-Signatures entry signed by k.
func (k rotationKey) entry(timestamp string, body []byte) string {
	return k.id + ":" + webhook.ComputeSignature(timestamp, body, k.priv)
}

// zeroSignature is a well-formed signature (64 bytes, base64) that no key
// produced.
var zeroSignature = base64.StdEncoding.EncodeToString(
	make([]byte, ed25519.SignatureSize),
)

func newVerifier(t *testing.T, keys ...string) *webhook.Verifier {
	t.Helper()
	v, err := webhook.NewVerifier(keys...)
	if err != nil {
		t.Fatalf("NewVerifier: %v", err)
	}
	return v
}

// The golden vector from platform's webhook_signer_rotation_test.go
// (TestPrimarySignatureIsUnchangedByteForByte, platform#2668). The values are
// literals computed by the platform signer, not recomputed here.
const (
	goldenPrivateKeyHex = "0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20" +
		"79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664"
	goldenPublicKeyHex = "79b5562e8fe654f94078b112e8a98ba7901f853ae695bed7e0e3910bad049664"
	goldenKeyID        = "79b5562e"
	goldenTimestamp    = "1795430000"
	goldenBody         = `{"id":"evt_golden"}`
	goldenSignature    = "kIhEmnAagqhecIiFrM89I6C2n8J4m6VS8GI8Rf7R1EROAtsC1clJef+0BE9nTNwzB3OmR8wkVpBqGcEcpXZuAA=="
)

func TestVerifier_GoldenVector(t *testing.T) {
	pub, err := webhook.ParsePublicKey(goldenPublicKeyHex)
	if err != nil {
		t.Fatalf("ParsePublicKey: %v", err)
	}
	if got := webhook.KeyID(pub); got != goldenKeyID {
		t.Errorf("KeyID = %q, want %q", got, goldenKeyID)
	}

	privBytes, err := hex.DecodeString(goldenPrivateKeyHex)
	if err != nil {
		t.Fatalf("decode private key: %v", err)
	}
	got := webhook.ComputeSignature(
		goldenTimestamp,
		[]byte(goldenBody),
		ed25519.PrivateKey(privBytes),
	)
	if got != goldenSignature {
		t.Errorf("ComputeSignature = %q, want platform's %q", got, goldenSignature)
	}

	v := newVerifier(t, goldenPublicKeyHex)
	if ids := v.KeyIDs(); len(ids) != 1 || ids[0] != goldenKeyID {
		t.Errorf("KeyIDs = %v, want [%s]", ids, goldenKeyID)
	}

	tests := []struct {
		name       string
		signatures string
		legacy     string
	}{
		{"legacy header only", "", goldenSignature},
		{"signatures header only", goldenKeyID + ":" + goldenSignature, ""},
		{"both headers", goldenKeyID + ":" + goldenSignature, goldenSignature},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := v.Verify(
				[]byte(goldenBody),
				goldenTimestamp,
				tt.signatures,
				tt.legacy,
			); err != nil {
				t.Errorf("Verify: %v", err)
			}
		})
	}

	if err := v.Verify(
		[]byte(`{"id":"evt_golden2"}`),
		goldenTimestamp,
		goldenKeyID+":"+goldenSignature,
		goldenSignature,
	); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("tampered body: got %v, want ErrInvalidSignature", err)
	}
}

// A receiver trusting the old and the new key verifies every delivery
// platform sends before, during and after a rotation.
func TestVerifier_RotationPhases(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_rotation"}`)
	oldSig := webhook.ComputeSignature(ts, body, oldKey.priv)
	newSig := webhook.ComputeSignature(ts, body, newKey.priv)

	tests := []struct {
		name       string
		signatures string
		legacy     string
	}{
		{"legacy header with the old key", "", oldSig},
		{"legacy header with the new key", "", newSig},
		{"signatures header with the new key", newKey.entry(ts, body), ""},
		{
			"signatures header with both keys",
			oldKey.entry(ts, body) + ", " + newKey.entry(ts, body),
			oldSig,
		},
		{
			"signatures header with both keys, new first, no spaces",
			newKey.entry(ts, body) + "," + oldKey.entry(ts, body),
			"",
		},
		{"cutover: both headers on the new key", newKey.entry(ts, body), newSig},
	}

	for _, trusted := range []struct {
		name string
		keys []string
	}{
		{"two arguments", []string{oldKey.hex, newKey.hex}},
		{"one comma-separated list", []string{oldKey.hex + ", " + newKey.hex}},
	} {
		v := newVerifier(t, trusted.keys...)
		for _, tt := range tests {
			t.Run(trusted.name+"/"+tt.name, func(t *testing.T) {
				if err := v.Verify(body, ts, tt.signatures, tt.legacy); err != nil {
					t.Errorf("Verify: %v", err)
				}
			})
		}
	}
}

// A receiver that trusts only the old key stops verifying at cutover, and one
// that trusts only the new key verifies from the overlap on.
func TestVerifier_SingleTrustedKeyAcrossRotation(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_rotation"}`)
	oldSig := webhook.ComputeSignature(ts, body, oldKey.priv)
	newSig := webhook.ComputeSignature(ts, body, newKey.priv)
	overlap := oldKey.entry(ts, body) + ", " + newKey.entry(ts, body)

	onlyOld := newVerifier(t, oldKey.hex)
	if err := onlyOld.Verify(body, ts, overlap, oldSig); err != nil {
		t.Errorf("old-key receiver during overlap: %v", err)
	}
	if err := onlyOld.Verify(
		body, ts, newKey.entry(ts, body), newSig,
	); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("old-key receiver after cutover: got %v, want ErrInvalidSignature", err)
	}

	onlyNew := newVerifier(t, newKey.hex)
	if err := onlyNew.Verify(body, ts, overlap, oldSig); err != nil {
		t.Errorf("new-key receiver during overlap: %v", err)
	}
}

func TestVerifier_RepeatedKeyIDIsCheckedOnce(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_repeat"}`)
	good := key.entry(ts, body)
	bad := key.id + ":" + zeroSignature
	v := newVerifier(t, key.hex)

	// The first entry naming a trusted key is the one checked for it; a later
	// entry for the same key is skipped, so an attacker cannot make the
	// receiver run more than one check per trusted key.
	if err := v.Verify(body, ts, bad+", "+good, ""); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("bad then good: got %v, want ErrInvalidSignature", err)
	}
	if err := v.Verify(body, ts, good+", "+bad, ""); err != nil {
		t.Errorf("good then bad: %v", err)
	}

	// Comparison is case-insensitive, so an upper-case repeat is the same key.
	upper := strings.ToUpper(key.id) + ":" + zeroSignature
	if err := v.Verify(body, ts, upper+", "+good, ""); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("upper-case bad then good: got %v, want ErrInvalidSignature", err)
	}
}

func TestVerifier_UnknownKeyIDsAreSkipped(t *testing.T) {
	key, stranger := newRotationKey(t), newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_unknown"}`)
	v := newVerifier(t, key.hex)

	header := "deadbeef:" + zeroSignature + ", " +
		stranger.entry(ts, body) + ", " + key.entry(ts, body)
	if err := v.Verify(body, ts, header, ""); err != nil {
		t.Errorf("Verify: %v", err)
	}

	// Only untrusted entries: rejected, even though they are validly signed.
	if err := v.Verify(
		body, ts, stranger.entry(ts, body), "",
	); !errors.Is(err, errors.ErrInvalidSignature) {
		t.Errorf("untrusted only: got %v, want ErrInvalidSignature", err)
	}
}

func TestVerifier_UpperCaseKeyIDMatches(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_case"}`)
	v := newVerifier(t, strings.ToUpper(key.hex))

	header := strings.ToUpper(key.id) + ":" + webhook.ComputeSignature(ts, body, key.priv)
	if err := v.Verify(body, ts, header, ""); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

func TestVerifier_MalformedEntriesNeverPanic(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_malformed"}`)
	v := newVerifier(t, key.hex)

	malformed := []string{
		"garbage",
		":",
		",,,",
		key.id,
		key.id + ":",
		key.id + ":!!!not-base64!!!",
		key.id + ":" + base64.StdEncoding.EncodeToString([]byte("short")),
		":" + zeroSignature,
		key.id + "::" + zeroSignature,
		strings.Repeat(",", 10000),
		"\x00\xff:\xff",
	}
	for _, header := range malformed {
		t.Run(fmt.Sprintf("%.24q", header), func(t *testing.T) {
			err := v.Verify(body, ts, header, "")
			if !errors.Is(err, errors.ErrInvalidSignature) {
				t.Errorf("got %v, want ErrInvalidSignature", err)
			}
		})
	}

	// Malformed entries for other key ids don't stop a good one.
	header := "garbage, :" + zeroSignature + ", deadbeef:!!!, " + key.entry(ts, body)
	if err := v.Verify(body, ts, header, ""); err != nil {
		t.Errorf("malformed then good: %v", err)
	}
}

// When X-Dakota-Webhook-Signatures is present, X-Webhook-Signature is not
// consulted, even if it would verify.
func TestVerifier_NoFallbackWhenSignaturesHeaderFails(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_nofallback"}`)
	legacy := webhook.ComputeSignature(ts, body, key.priv)
	v := newVerifier(t, key.hex)

	for _, header := range []string{key.id + ":" + zeroSignature, "garbage"} {
		if err := v.Verify(body, ts, header, legacy); !errors.Is(err, errors.ErrInvalidSignature) {
			t.Errorf("header %q: got %v, want ErrInvalidSignature", header, err)
		}
	}
}

func TestVerifier_BlankSignaturesHeaderFallsBack(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_blank"}`)
	legacy := webhook.ComputeSignature(ts, body, key.priv)
	v := newVerifier(t, key.hex)

	for _, header := range []string{"", "   ", " \t "} {
		if err := v.Verify(body, ts, header, legacy); err != nil {
			t.Errorf("header %q: %v", header, err)
		}
	}
}

func TestVerifier_MissingSignatures(t *testing.T) {
	key := newRotationKey(t)
	v := newVerifier(t, key.hex)

	err := v.Verify([]byte(`{}`), "1795430000", " ", "")
	if !errors.Is(err, errors.ErrMissingHeader) {
		t.Errorf("got %v, want ErrMissingHeader", err)
	}
}

// Two trusted keys can share a key id (the first 8 hex characters). Both are
// checked for an entry naming that id.
func TestVerifier_TrustedKeysSharingAKeyID(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_shared"}`)

	// Same first 4 bytes as key, different rest.
	twin := make([]byte, ed25519.PublicKeySize)
	copy(twin, key.pub)
	for i := 4; i < len(twin); i++ {
		twin[i] ^= 0xff
	}
	twinHex := hex.EncodeToString(twin)

	v := newVerifier(t, twinHex, key.hex)
	if err := v.Verify(body, ts, key.entry(ts, body), ""); err != nil {
		t.Errorf("Verify: %v", err)
	}
	if ids := v.KeyIDs(); len(ids) != 2 || ids[0] != key.id || ids[1] != key.id {
		t.Errorf("KeyIDs = %v, want [%s %s]", ids, key.id, key.id)
	}
}

func TestVerifier_VerifyHeaders(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_headers"}`)
	v := newVerifier(t, oldKey.hex)

	// A proxy may split the list across several header lines.
	h := http.Header{}
	h.Set(webhook.TimestampHeader, ts)
	h.Add(webhook.SignaturesHeader, newKey.entry(ts, body))
	h.Add(webhook.SignaturesHeader, oldKey.entry(ts, body))
	if err := v.VerifyHeaders(body, h); err != nil {
		t.Errorf("split header: %v", err)
	}

	legacyOnly := http.Header{}
	legacyOnly.Set(webhook.TimestampHeader, ts)
	legacyOnly.Set(webhook.SignatureHeader, webhook.ComputeSignature(ts, body, oldKey.priv))
	if err := v.VerifyHeaders(body, legacyOnly); err != nil {
		t.Errorf("legacy only: %v", err)
	}

	noTimestamp := http.Header{}
	noTimestamp.Set(webhook.SignaturesHeader, oldKey.entry(ts, body))
	if err := v.VerifyHeaders(body, noTimestamp); !errors.Is(err, errors.ErrMissingHeader) {
		t.Errorf("no timestamp: got %v, want ErrMissingHeader", err)
	}
}

func TestVerifier_ConstructEvent(t *testing.T) {
	oldKey, newKey := newRotationKey(t), newRotationKey(t)
	v := newVerifier(t, oldKey.hex, newKey.hex)
	body := []byte(`{"id":"evt_1","type":"customer.created","data":{}}`)

	now := fmt.Sprintf("%d", time.Now().Unix())
	h := http.Header{}
	h.Set(webhook.TimestampHeader, now)
	h.Set(webhook.SignaturesHeader, newKey.entry(now, body))
	event, err := v.ConstructEvent(body, h)
	if err != nil {
		t.Fatalf("ConstructEvent: %v", err)
	}
	if event.ID != "evt_1" {
		t.Errorf("event.ID = %q, want evt_1", event.ID)
	}

	old := fmt.Sprintf("%d", time.Now().Add(-time.Hour).Unix())
	h.Set(webhook.TimestampHeader, old)
	h.Set(webhook.SignaturesHeader, newKey.entry(old, body))
	if _, err := v.ConstructEvent(body, h); !errors.Is(err, errors.ErrSignatureExpired) {
		t.Errorf("stale timestamp: got %v, want ErrSignatureExpired", err)
	}
	if _, err := v.ConstructEvent(body, h, webhook.IgnoringTolerance()); err != nil {
		t.Errorf("stale timestamp, tolerance ignored: %v", err)
	}
}

func TestParsePublicKeys(t *testing.T) {
	a, b := newRotationKey(t), newRotationKey(t)

	keys, err := webhook.ParsePublicKeys(" "+a.hex+" ,, "+strings.ToUpper(b.hex)+", ", a.hex)
	if err != nil {
		t.Fatalf("ParsePublicKeys: %v", err)
	}
	if len(keys) != 2 || !keys[0].Equal(a.pub) || !keys[1].Equal(b.pub) {
		t.Errorf("got %d keys, want a then b, duplicates dropped", len(keys))
	}

	bad := map[string]string{
		"empty":         "",
		"blank list":    " , ,",
		"not hex":       a.hex + ",zz" + b.hex[2:],
		"too short":     a.hex[:62],
		"private key":   hex.EncodeToString(a.priv),
		"hex with junk": a.hex + "x",
	}
	for name, value := range bad {
		t.Run(name, func(t *testing.T) {
			_, err := webhook.ParsePublicKeys(value)
			if !errors.Is(err, errors.ErrInvalidConfig) {
				t.Fatalf("got %v, want ErrInvalidConfig", err)
			}
			// A private key pasted by mistake must not reach logs.
			for _, k := range strings.Split(value, ",") {
				k = strings.TrimSpace(k)
				for i := 0; i+8 <= len(k); i++ {
					if strings.Contains(err.Error(), k[i:i+8]) {
						t.Fatalf("error quotes the value: %v", err)
					}
				}
			}
		})
	}

	if _, err := webhook.NewVerifier(""); !errors.Is(err, errors.ErrInvalidConfig) {
		t.Errorf("NewVerifier(\"\"): got %v, want ErrInvalidConfig", err)
	}
	if _, err := webhook.NewVerifier(); !errors.Is(err, errors.ErrInvalidConfig) {
		t.Errorf("NewVerifier(): got %v, want ErrInvalidConfig", err)
	}
}

// Both orders, so a verifier that checks only the first or only the last key
// sharing an id is caught.
func TestVerifier_TrustedKeysSharingAKeyID_EitherOrder(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_shared"}`)
	twin := make([]byte, ed25519.PublicKeySize)
	copy(twin, key.pub)
	for i := 4; i < len(twin); i++ {
		twin[i] ^= 0xff
	}
	twinHex := hex.EncodeToString(twin)

	for name, keys := range map[string][]string{
		"twin first": {twinHex, key.hex},
		"twin last":  {key.hex, twinHex},
	} {
		t.Run(name, func(t *testing.T) {
			v := newVerifier(t, keys...)
			if err := v.Verify(body, ts, key.entry(ts, body), ""); err != nil {
				t.Errorf("Verify: %v", err)
			}
		})
	}
}

func TestVerifier_KeyIDsOrder(t *testing.T) {
	a, b := newRotationKey(t), newRotationKey(t)
	ids := newVerifier(t, b.hex, a.hex).KeyIDs()
	if len(ids) != 2 || ids[0] != b.id || ids[1] != a.id {
		t.Errorf("KeyIDs = %v, want [%s %s]", ids, b.id, a.id)
	}
}

func TestKeyID_ShortKeyDoesNotPanic(t *testing.T) {
	if got := webhook.KeyID(nil); got != "" {
		t.Errorf("KeyID(nil) = %q, want empty", got)
	}
	if got := webhook.KeyID(ed25519.PublicKey{0xab}); got != "ab" {
		t.Errorf("KeyID(ab) = %q, want ab", got)
	}
}

// Spaces around the id or the signature inside an entry are tolerated.
func TestVerifier_SpacesInsideEntry(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_spaces"}`)
	v := newVerifier(t, key.hex)

	header := " " + key.id + " : " + webhook.ComputeSignature(ts, body, key.priv) + " "
	if err := v.Verify(body, ts, header, ""); err != nil {
		t.Errorf("Verify: %v", err)
	}
}

// Empty X-Dakota-Webhook-Signatures lines count as a missing header, so the
// legacy header is checked.
func TestVerifier_VerifyHeaders_EmptyLinesFallBack(t *testing.T) {
	key := newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_empty_lines"}`)
	v := newVerifier(t, key.hex)

	h := http.Header{}
	h.Set(webhook.TimestampHeader, ts)
	h.Add(webhook.SignaturesHeader, "")
	h.Add(webhook.SignaturesHeader, "  ")
	h.Set(webhook.SignatureHeader, webhook.ComputeSignature(ts, body, key.priv))
	if err := v.VerifyHeaders(body, h); err != nil {
		t.Errorf("VerifyHeaders: %v", err)
	}
}

func TestVerifier_ConstructEvent_CustomTolerance(t *testing.T) {
	key := newRotationKey(t)
	v := newVerifier(t, key.hex)
	body := []byte(`{"id":"evt_1","type":"customer.created","data":{}}`)

	ts := fmt.Sprintf("%d", time.Now().Add(-10*time.Minute).Unix())
	h := http.Header{}
	h.Set(webhook.TimestampHeader, ts)
	h.Set(webhook.SignaturesHeader, key.entry(ts, body))

	if _, err := v.ConstructEvent(body, h); !errors.Is(err, errors.ErrSignatureExpired) {
		t.Errorf("default tolerance: got %v, want ErrSignatureExpired", err)
	}
	if _, err := v.ConstructEvent(body, h, webhook.WithTolerance(time.Hour)); err != nil {
		t.Errorf("one-hour tolerance: %v", err)
	}
	if _, err := v.ConstructEvent(body, h, webhook.WithTolerance(time.Minute)); !errors.Is(err, errors.ErrSignatureExpired) {
		t.Errorf("one-minute tolerance: got %v, want ErrSignatureExpired", err)
	}
}

func FuzzVerifier_Verify(f *testing.F) {
	pub, _ := webhook.ParsePublicKey(goldenPublicKeyHex)
	id := webhook.KeyID(pub)
	f.Add(goldenTimestamp, id+":"+goldenSignature, goldenSignature, []byte(goldenBody))
	f.Add("", ",,: ,"+id+":", "", []byte{})
	f.Add("1", id+"::"+id+":!!", "%%%", []byte("x"))
	v, err := webhook.NewVerifier(goldenPublicKeyHex)
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, ts, sigs, legacy string, body []byte) {
		err := v.Verify(body, ts, sigs, legacy)
		if err == nil && (ts != goldenTimestamp || string(body) != goldenBody) {
			t.Fatalf("accepted a message the golden key never signed: ts=%q body=%q", ts, body)
		}
	})
}

// Rejections say why, so an operator can tell a missing trusted key from a
// failed signature. Only trusted key ids (safe to log) are named.
func TestVerifier_RejectionReasons(t *testing.T) {
	key, stranger := newRotationKey(t), newRotationKey(t)
	ts := "1795430000"
	body := []byte(`{"id":"evt_reasons"}`)
	v := newVerifier(t, key.hex)

	tests := []struct {
		name       string
		signatures string
		legacy     string
		want       string
	}{
		{"only untrusted ids", stranger.entry(ts, body), "", "names none of the trusted keys"},
		{"trusted key fails", key.id + ":" + zeroSignature, "", "did not verify"},
		{"legacy fails", "", zeroSignature, "did not verify"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Verify(body, ts, tt.signatures, tt.legacy)
			if !errors.Is(err, errors.ErrInvalidSignature) {
				t.Fatalf("got %v, want ErrInvalidSignature", err)
			}
			if !strings.Contains(err.Error(), tt.want) || !strings.Contains(err.Error(), key.id) {
				t.Errorf("error %q should contain %q and the trusted key id %s", err, tt.want, key.id)
			}
			if strings.Contains(err.Error(), stranger.id) {
				t.Errorf("error %q names an untrusted key id from the request", err)
			}
		})
	}
}
