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
func ES256IntentSigner(s Signer) IntentSigner {
	return IntentSignerFunc(func(_ context.Context, canonicalIntent []byte) (string, error) {
		if s == nil {
			return "", errors.New("nil ES256 signer")
		}
		return s.Sign(canonicalIntent)
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
// It refuses an assertion that could not verify, before the platform does:
// clientDataJSON must be of type webauthn.get and its challenge must decode to
// exactly canonicalIntent (catching a hashed challenge or one signed over a
// different intent or version), and the credential id, authenticatorData and
// signature must be present. It does not verify the signature itself: the SDK
// does not know the credential's public key.
func EncodeWebAuthnAssertion(canonicalIntent, assertionJSON []byte) (string, error) {
	var a WebAuthnAssertion
	if err := json.Unmarshal(assertionJSON, &a); err != nil {
		return "", fmt.Errorf("webauthn assertion is not JSON: %w", err)
	}
	switch {
	case a.ID == "" && a.RawID == "":
		return "", errors.New("webauthn assertion has no credential id")
	case a.Response.AuthenticatorData == "":
		return "", errors.New("webauthn assertion has no authenticatorData")
	case a.Response.Signature == "":
		return "", errors.New("webauthn assertion has no signature")
	case a.Response.ClientDataJSON == "":
		return "", errors.New("webauthn assertion has no clientDataJSON")
	}
	cdRaw, err := decodeBase64Any(a.Response.ClientDataJSON)
	if err != nil {
		return "", fmt.Errorf("webauthn clientDataJSON: %w", err)
	}
	var cd struct {
		Type      string `json:"type"`
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(cdRaw, &cd); err != nil {
		return "", fmt.Errorf("webauthn clientDataJSON is not JSON: %w", err)
	}
	if cd.Type != "webauthn.get" {
		return "", fmt.Errorf("webauthn clientDataJSON type is %q, want webauthn.get (an assertion, not a registration)", cd.Type)
	}
	challenge, err := decodeBase64Any(cd.Challenge)
	if err != nil {
		return "", fmt.Errorf("webauthn challenge: %w", err)
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

// decodeBase64Any accepts the base64 spellings WebAuthn libraries emit:
// base64url with or without padding, and standard base64.
func decodeBase64Any(s string) ([]byte, error) {
	for _, enc := range []*base64.Encoding{base64.RawURLEncoding, base64.URLEncoding, base64.RawStdEncoding, base64.StdEncoding} {
		if b, err := enc.DecodeString(s); err == nil {
			return b, nil
		}
	}
	return nil, errors.New("not base64 or base64url")
}
