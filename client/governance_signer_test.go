package client

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

const testIntent = `{"idempotency_key":"047caa77-8cec-4e96-994c-a12b655231de","new_member_keys":["pk"],"signer_group_id":"sg","signer_group_version":0,"type":"add_signers_to_group"}`

// syntheticPasskey is a software WebAuthn authenticator: the recipe that
// passed policy-engine in sandbox on 2026-11-17 (ENG-6464). authenticatorData
// is SHA-256(rpId) ‖ flags 0x05 (user present + verified) ‖ signCount, and the
// signature is ECDSA P-256 over authenticatorData ‖ SHA-256(clientDataJSON).
type syntheticPasskey struct {
	priv  *ecdsa.PrivateKey
	rpID  string
	count uint32
	// challengeOverride, when set, replaces the challenge the authenticator
	// is given (to simulate a caller that hashed or altered it).
	challengeOverride func([]byte) []byte
	clientDataType    string
}

func newSyntheticPasskey(t *testing.T) *syntheticPasskey {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)
	return &syntheticPasskey{priv: priv, rpID: "passkey-test.dakota.xyz", clientDataType: "webauthn.get"}
}

func (p *syntheticPasskey) assert(_ context.Context, challenge []byte) ([]byte, error) {
	if p.challengeOverride != nil {
		challenge = p.challengeOverride(challenge)
	}
	p.count++
	rpHash := sha256.Sum256([]byte(p.rpID))
	authData := append(rpHash[:], 0x05, byte(p.count>>24), byte(p.count>>16), byte(p.count>>8), byte(p.count))
	clientData, err := json.Marshal(map[string]any{
		"type":        p.clientDataType,
		"challenge":   base64.RawURLEncoding.EncodeToString(challenge),
		"origin":      "https://" + p.rpID,
		"crossOrigin": false,
	})
	if err != nil {
		return nil, err
	}
	cdHash := sha256.Sum256(clientData)
	digest := sha256.Sum256(append(append([]byte{}, authData...), cdHash[:]...))
	sig, err := ecdsa.SignASN1(rand.Reader, p.priv, digest[:])
	if err != nil {
		return nil, err
	}
	b64 := base64.RawURLEncoding.EncodeToString
	return json.Marshal(WebAuthnAssertion{
		ID:                      b64([]byte("cred-1")),
		RawID:                   b64([]byte("cred-1")),
		Type:                    "public-key",
		AuthenticatorAttachment: "platform",
		Response: WebAuthnAssertionResponse{
			ClientDataJSON:    b64(clientData),
			AuthenticatorData: b64(authData),
			Signature:         b64(sig),
		},
	})
}

func TestES256IntentSigner(t *testing.T) {
	t.Parallel()
	key, err := NewP256Signer()
	require.NoError(t, err)

	sig, err := ES256IntentSigner(key).SignIntent(context.Background(), []byte(testIntent))
	require.NoError(t, err)
	// The platform's ES256 format: base64 ASN.1 DER over SHA-256(intent).
	require.NoError(t, VerifyMandateSignature(key.PublicKeyBase64(), []byte(testIntent), sig))
}

func TestWebAuthnSigner(t *testing.T) {
	t.Parallel()

	t.Run("challenge is the raw intent and the entry is base64url of the assertion", func(t *testing.T) {
		pk := newSyntheticPasskey(t)
		var seen []byte
		signer := WebAuthnSigner(func(ctx context.Context, challenge []byte) ([]byte, error) {
			seen = challenge
			return pk.assert(ctx, challenge)
		})

		entry, err := signer.SignIntent(context.Background(), []byte(testIntent))
		require.NoError(t, err)
		require.Equal(t, testIntent, string(seen), "the challenge must be the canonical bytes, not a hash")

		raw, err := base64.RawURLEncoding.DecodeString(entry)
		require.NoError(t, err, "the signatures[] entry is unpadded base64url")
		var a WebAuthnAssertion
		require.NoError(t, json.Unmarshal(raw, &a))

		// Verify the assertion the way the server does, independently of the SDK.
		cd, err := base64.RawURLEncoding.DecodeString(a.Response.ClientDataJSON)
		require.NoError(t, err)
		var clientData struct{ Type, Challenge string }
		require.NoError(t, json.Unmarshal(cd, &clientData))
		require.Equal(t, "webauthn.get", clientData.Type)
		require.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(testIntent)), clientData.Challenge)
		authData, err := base64.RawURLEncoding.DecodeString(a.Response.AuthenticatorData)
		require.NoError(t, err)
		sig, err := base64.RawURLEncoding.DecodeString(a.Response.Signature)
		require.NoError(t, err)
		cdHash := sha256.Sum256(cd)
		digest := sha256.Sum256(append(authData, cdHash[:]...))
		require.True(t, ecdsa.VerifyASN1(&pk.priv.PublicKey, digest[:], sig))
	})

	t.Run("WebAuthnChallenge is the clientDataJSON challenge", func(t *testing.T) {
		require.Equal(t, base64.RawURLEncoding.EncodeToString([]byte(testIntent)), WebAuthnChallenge([]byte(testIntent)))
	})

	refusals := map[string]func(p *syntheticPasskey){
		// The classic mistake: signing SHA-256(intent) as the challenge.
		"hashed challenge": func(p *syntheticPasskey) {
			p.challengeOverride = func(c []byte) []byte { h := sha256.Sum256(c); return h[:] }
		},
		"other intent": func(p *syntheticPasskey) {
			p.challengeOverride = func([]byte) []byte { return []byte(`{"type":"other"}`) }
		},
		"registration, not assertion": func(p *syntheticPasskey) { p.clientDataType = "webauthn.create" },
	}
	for name, spoil := range refusals {
		t.Run("refuses "+name, func(t *testing.T) {
			pk := newSyntheticPasskey(t)
			spoil(pk)
			entry, err := WebAuthnSigner(pk.assert).SignIntent(context.Background(), []byte(testIntent))
			require.Error(t, err)
			require.Empty(t, entry)
		})
	}

	t.Run("refuses an assertion without a signature", func(t *testing.T) {
		pk := newSyntheticPasskey(t)
		raw, err := pk.assert(context.Background(), []byte(testIntent))
		require.NoError(t, err)
		var a WebAuthnAssertion
		require.NoError(t, json.Unmarshal(raw, &a))
		a.Response.Signature = ""
		stripped, err := json.Marshal(a)
		require.NoError(t, err)
		_, err = EncodeWebAuthnAssertion([]byte(testIntent), stripped)
		require.Error(t, err)
	})

	t.Run("accepts an assertion collected elsewhere (browser JSON with whitespace)", func(t *testing.T) {
		pk := newSyntheticPasskey(t)
		raw, err := pk.assert(context.Background(), []byte(testIntent))
		require.NoError(t, err)
		var pretty map[string]any
		require.NoError(t, json.Unmarshal(raw, &pretty))
		indented, err := json.MarshalIndent(pretty, "", "  ")
		require.NoError(t, err)
		entry, err := EncodeWebAuthnAssertion([]byte(testIntent), indented)
		require.NoError(t, err)
		decoded, err := base64.RawURLEncoding.DecodeString(entry)
		require.NoError(t, err)
		require.JSONEq(t, string(raw), string(decoded))
	})
}
