package client

// Signers for governance intents. A governed change carries one entry in
// signatures[] per endorser, in either format policy-engine accepts:
//
//   - ES256: base64 ASN.1 DER ECDSA P-256 over SHA-256(canonical intent) — the
//     format the existing Signer / P256Signer already produce.
//   - WebAuthn (passkey): the authenticator signs a challenge that is the RAW
//     canonical intent bytes (base64url in clientDataJSON), NOT their hash. The
//     entry is base64url of the CredentialAssertionResponse JSON.
//
// The SDK never holds a passkey: the caller runs navigator.credentials.get (or
// its platform equivalent) and hands back the assertion. The SDK shapes the
// challenge, checks the assertion answers it, and packages it.

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// IntentSigner produces one signatures[] entry over a canonical governance
// intent (see AddSignersToGroupIntent and friends). Use ES256IntentSigner for a
// P-256 key and WebAuthnSigner for a passkey; implement it yourself for other
// custody.
type IntentSigner interface {
	SignIntent(ctx context.Context, canonicalIntent []byte) (string, error)
}

// IntentSignerFunc adapts a function to IntentSigner.
type IntentSignerFunc func(ctx context.Context, canonicalIntent []byte) (string, error)

// SignIntent calls f.
func (f IntentSignerFunc) SignIntent(ctx context.Context, canonicalIntent []byte) (string, error) {
	return f(ctx, canonicalIntent)
}

// ES256IntentSigner signs governance intents with an ES256 Signer (P256Signer,
// or your own HSM/KMS-backed implementation): base64 ASN.1 DER over
// SHA-256(canonical intent). The signer's public key must be a member of the
// group's governance.
//
// Each signature is verified locally against the signer's PublicKeyBase64
// before it is returned, so a custody backend that returns another encoding
// (raw r‖s instead of ASN.1 DER, say) fails here rather than at the platform,
// where a refused change uses up its idempotency key and every signature
// collected for it.
func ES256IntentSigner(s Signer) IntentSigner {
	return IntentSignerFunc(func(_ context.Context, canonicalIntent []byte) (string, error) {
		if s == nil {
			return "", errors.New("nil ES256 signer")
		}
		if p, ok := s.(*P256Signer); ok && (p == nil || p.priv == nil) {
			return "", errors.New("nil ES256 signer")
		}
		sig, err := s.Sign(canonicalIntent)
		if err != nil {
			return "", err
		}
		if err := VerifyMandateSignature(s.PublicKeyBase64(), canonicalIntent, sig); err != nil {
			return "", fmt.Errorf("ES256 signer produced a signature that does not verify against its own public key (want base64 ASN.1 DER over SHA-256 of the intent): %w", err)
		}
		return sig, nil
	})
}

// WebAuthnAssertFunc runs a WebAuthn assertion (navigator.credentials.get) with
// challenge as the challenge bytes, and returns the resulting
// PublicKeyCredential as JSON — the shape PublicKeyCredential.toJSON() gives,
// see WebAuthnAssertion. Pass challenge to the authenticator unchanged: do not
// hash or re-encode it (WebAuthn libraries base64url-encode it into
// clientDataJSON themselves; WebAuthnChallenge returns that encoding when your
// front end needs the string).
type WebAuthnAssertFunc func(ctx context.Context, challenge []byte) (assertionJSON []byte, err error)

// WebAuthnSigner signs governance intents with a passkey. assert is called with
// the canonical intent as the challenge; the returned assertion is checked to
// answer exactly that challenge (see EncodeWebAuthnAssertion) and packaged as a
// signatures[] entry. The passkey's credential must be registered as a WEBAUTHN
// signer that is a member of the group's governance, and its RP ID must be one
// the platform accepts for your client.
func WebAuthnSigner(assert WebAuthnAssertFunc) IntentSigner {
	return IntentSignerFunc(func(ctx context.Context, canonicalIntent []byte) (string, error) {
		if assert == nil {
			return "", errors.New("nil WebAuthn assert function")
		}
		// Hand the authenticator its own copy, so a library that mutates the
		// slice cannot change the bytes checked below.
		assertionJSON, err := assert(ctx, bytes.Clone(canonicalIntent))
		if err != nil {
			return "", fmt.Errorf("webauthn assertion: %w", err)
		}
		return EncodeWebAuthnAssertion(canonicalIntent, assertionJSON)
	})
}

// WebAuthnChallenge is the challenge string a passkey assertion over
// canonicalIntent carries in its clientDataJSON: base64url, no padding, of the
// raw canonical bytes. Give the browser the bytes themselves
// (publicKey.challenge); this string is for comparing or for front ends whose
// WebAuthn library takes the encoded form.
func WebAuthnChallenge(canonicalIntent []byte) string {
	return base64.RawURLEncoding.EncodeToString(canonicalIntent)
}

// WebAuthnAssertion is a WebAuthn assertion credential in the JSON shape of
// PublicKeyCredential.toJSON(): binary fields base64url-encoded.
type WebAuthnAssertion struct {
	ID                      string                    `json:"id"`
	RawID                   string                    `json:"rawId"`
	Type                    string                    `json:"type"`
	AuthenticatorAttachment string                    `json:"authenticatorAttachment,omitempty"`
	Response                WebAuthnAssertionResponse `json:"response"`
}

// WebAuthnAssertionResponse is an AuthenticatorAssertionResponse, base64url
// fields as in PublicKeyCredential.toJSON().
type WebAuthnAssertionResponse struct {
	ClientDataJSON    string `json:"clientDataJSON"`
	AuthenticatorData string `json:"authenticatorData"`
	Signature         string `json:"signature"`
	UserHandle        string `json:"userHandle,omitempty"`
}

// EncodeWebAuthnAssertion turns a passkey assertion over canonicalIntent into
// a signatures[] entry: base64url (no padding) of the assertion JSON. Use it
// when assertions are collected outside WebAuthnSigner, e.g. from several
// people's browsers for a quorum.
//
// It refuses, before the platform does, an assertion the platform's WebAuthn
// parser would reject or that answers another challenge:
//   - id is present and unpadded base64url, and type is "public-key";
//   - rawId, clientDataJSON, authenticatorData, signature and userHandle are
//     base64url (standard base64 with + or / is refused);
//   - authenticatorData is at least 37 bytes (RP ID hash, flags, counter);
//   - clientDataJSON is of type webauthn.get, and its challenge decodes to
//     exactly canonicalIntent (catching a hashed challenge, or one signed over
//     another intent or an old group version).
//
// It cannot verify the signature itself: the SDK does not know the
// credential's public key. A refused change uses up its idempotency key and
// every signature collected for it, which is why the shape is checked here.
func EncodeWebAuthnAssertion(canonicalIntent, assertionJSON []byte) (string, error) {
	var a WebAuthnAssertion
	if err := json.Unmarshal(assertionJSON, &a); err != nil {
		return "", fmt.Errorf("webauthn assertion is not JSON: %w", err)
	}
	if a.ID == "" {
		return "", errors.New("webauthn assertion has no id")
	}
	if _, err := base64.RawURLEncoding.DecodeString(a.ID); err != nil {
		return "", errors.New("webauthn assertion id is not unpadded base64url")
	}
	if a.Type != "public-key" {
		return "", fmt.Errorf("webauthn assertion type is %q, want public-key", a.Type)
	}
	fields := []struct {
		name, value string
		required    bool
	}{
		{"rawId", a.RawID, false},
		{"clientDataJSON", a.Response.ClientDataJSON, true},
		{"authenticatorData", a.Response.AuthenticatorData, true},
		{"signature", a.Response.Signature, true},
		{"userHandle", a.Response.UserHandle, false},
	}
	decoded := map[string][]byte{}
	for _, f := range fields {
		if f.value == "" {
			if f.required {
				return "", fmt.Errorf("webauthn assertion has no %s", f.name)
			}
			continue
		}
		b, err := decodeBase64URL(f.value)
		if err != nil {
			return "", fmt.Errorf("webauthn assertion %s is not base64url: %w", f.name, err)
		}
		decoded[f.name] = b
	}
	if n := len(decoded["authenticatorData"]); n < 37 {
		return "", fmt.Errorf("webauthn authenticatorData is %d bytes, want at least 37", n)
	}
	var cd struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(decoded["clientDataJSON"], &cd); err != nil {
		return "", fmt.Errorf("webauthn clientDataJSON is not JSON: %w", err)
	}
	if cd.Type != "webauthn.get" {
		return "", fmt.Errorf("webauthn clientDataJSON type is %q, want webauthn.get (an assertion, not a registration)", cd.Type)
	}
	challenge, err := decodeBase64URL(cd.Challenge)
	if err != nil {
		return "", fmt.Errorf("webauthn challenge is not base64url: %w", err)
	}
	if !bytes.Equal(challenge, canonicalIntent) {
		return "", errors.New("webauthn challenge is not the canonical intent: pass the intent bytes themselves as the challenge (not their hash), and re-sign after any change to the intent or the group's version")
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, assertionJSON); err != nil {
		return "", fmt.Errorf("compact webauthn assertion: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(compact.Bytes()), nil
}

// decodeBase64URL decodes base64url with or without padding, as the
// platform's WebAuthn parser does. Standard base64 (+ or /) is refused.
func decodeBase64URL(s string) ([]byte, error) {
	return base64.RawURLEncoding.DecodeString(strings.TrimRight(s, "="))
}
