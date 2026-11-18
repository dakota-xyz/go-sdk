// Command signer-group-governance walks the signed signer-group flow end to
// end against the Dakota SANDBOX: a passkey (WebAuthn) and an ES256 key endorse
// member adds and removals, and a stale signature gets the typed error.
//
// It creates throwaway signers and one throwaway signer group (names carry a
// timestamp), and no API keys. Run it with an API key of an ordinary sandbox
// client, never an internal superadmin client:
//
//	DAKOTA_API_KEY=... go run ./cmd/signer-group-governance
//
// The passkey here is a software authenticator, so the example runs without a
// browser. NEVER use a software authenticator outside a test: it claims "user
// present and verified" with no user. In a real integration the passkey lives
// in the user's browser or device, and WebAuthnSigner's assert function calls
// navigator.credentials.get with the challenge it is given (see the README's
// "Signer-group governance" section).
//
// The example also holds the governing keys in the same process as the API
// key, and uses a quorum of 1, to stay short. In production, keep governing
// keys away from the service that holds the API key (governance exists to
// limit what an API key alone can do) and prefer a quorum of 2 or more.
package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"

	"github.com/dakota-xyz/go-sdk/client"
	"github.com/dakota-xyz/go-sdk/client/gen"
)

func main() {
	if err := run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, "FAIL:", err)
		os.Exit(1)
	}
	fmt.Println("PASS")
}

func run(ctx context.Context) error {
	apiKey := os.Getenv("DAKOTA_API_KEY")
	if apiKey == "" {
		return errors.New("set DAKOTA_API_KEY to an API key of an ordinary sandbox client")
	}
	c, err := client.New(client.WithAPIKey(apiKey), client.WithEnvironment(client.EnvironmentSandbox))
	if err != nil {
		return err
	}
	suffix := time.Now().UTC().Format("150405")

	// Three ES256 keys and one passkey, registered as signers.
	esGov, esNew1, esNew2 := mustP256(), mustP256(), mustP256()
	passkey, err := newSoftwarePasskey("passkey-test.dakota.xyz")
	if err != nil {
		return err
	}
	for _, s := range []struct {
		name, key string
		typ       gen.SignerCreateRequestKeyType
	}{
		{"gov-passkey", passkey.coseKey(), gen.SignerCreateRequestKeyTypeWEBAUTHN},
		{"gov-es256", esGov.PublicKeyBase64(), gen.SignerCreateRequestKeyTypeES256},
		{"new-1", esNew1.PublicKeyBase64(), gen.SignerCreateRequestKeyTypeES256},
		{"new-2", esNew2.PublicKeyBase64(), gen.SignerCreateRequestKeyTypeES256},
	} {
		if _, err := client.CheckResponse(c.Raw().CreateSignerWithResponse(ctx,
			&gen.CreateSignerParams{XIdempotencyKey: uuid.New()},
			gen.SignerCreateRequest{Name: "sdk-governance-" + s.name + "-" + suffix, PublicKey: s.key, KeyType: s.typ})); err != nil {
			return fmt.Errorf("register signer %s: %w", s.name, err)
		}
	}

	// A group governed by its two members, either of whom may endorse
	// (governance_threshold 1). Governance settings make the platform refuse
	// unsigned membership changes even without client-wide enforcement.
	one := int32(1)
	created, err := client.CheckResponse(c.Raw().CreateSignerGroupWithResponse(ctx,
		&gen.CreateSignerGroupParams{XIdempotencyKey: uuid.New()},
		gen.SignerGroupCreateRequest{
			Name:                "sdk-governance-" + suffix,
			MemberKeys:          []string{passkey.coseKey(), esGov.PublicKeyBase64()},
			GovernanceThreshold: &one,
		}))
	if err != nil {
		return fmt.Errorf("create signer group: %w", err)
	}
	if created.JSON201 == nil {
		return errors.New("create signer group: empty response body")
	}
	groupID := created.JSON201.Id
	fmt.Printf("group %s created, version %d\n", groupID, deref(created.JSON201.Version))

	viaPasskey := client.WebAuthnSigner(passkey.assert)
	viaES256 := client.ES256IntentSigner(esGov)

	// Best effort, also on failure: take the throwaway members back out, so
	// the group is left with its two governing members.
	defer func() {
		cur, err := client.CheckResponse(c.Raw().GetSignerGroupWithResponse(ctx, groupID, nil))
		if err != nil || cur.JSON200 == nil {
			return
		}
		var extra []string
		for _, m := range cur.JSON200.Members {
			if m.PublicKey == esNew1.PublicKeyBase64() || m.PublicKey == esNew2.PublicKeyBase64() {
				extra = append(extra, m.PublicKey)
			}
		}
		if len(extra) > 0 {
			if _, err := c.RemoveSignersFromGroupSigned(ctx, groupID, extra, viaES256); err != nil {
				fmt.Fprintln(os.Stderr, "cleanup: could not remove throwaway members:", err)
			}
		}
	}()

	// 1. Unsigned add: refused.
	_, err = client.CheckResponse(c.Raw().CreateSignerGroupSignerWithResponse(ctx, groupID,
		&gen.CreateSignerGroupSignerParams{XIdempotencyKey: uuid.New()},
		gen.CreateSignerGroupSignerRequest{MemberKey: esNew1.PublicKeyBase64()}))
	if !errors.Is(err, client.ErrEndorsementRequired) {
		return fmt.Errorf("unsigned add: want ErrEndorsementRequired, got %v", err)
	}
	fmt.Println("1. unsigned add refused: endorsement_required")

	// 2. Add, signed with the passkey.
	g, err := c.AddSignerToGroupSigned(ctx, groupID, esNew1.PublicKeyBase64(), viaPasskey)
	if err != nil {
		return fmt.Errorf("passkey-signed add: %w", err)
	}
	fmt.Printf("2. passkey-signed add: version %d, %d members\n", deref(g.Version), len(g.Members))

	// 3. Remove, signed with the ES256 key.
	g, err = c.RemoveSignersFromGroupSigned(ctx, groupID, []string{esNew1.PublicKeyBase64()}, viaES256)
	if err != nil {
		return fmt.Errorf("ES256-signed removal: %w", err)
	}
	fmt.Printf("3. ES256-signed removal: version %d, %d members\n", deref(g.Version), len(g.Members))

	// 4. Add, signed with the ES256 key; 5. remove, signed with the passkey.
	if g, err = c.AddSignerToGroupSigned(ctx, groupID, esNew2.PublicKeyBase64(), viaES256); err != nil {
		return fmt.Errorf("ES256-signed add: %w", err)
	}
	fmt.Printf("4. ES256-signed add: version %d, %d members\n", deref(g.Version), len(g.Members))
	if g, err = c.RemoveSignersFromGroupSigned(ctx, groupID, []string{esNew2.PublicKeyBase64()}, viaPasskey); err != nil {
		return fmt.Errorf("passkey-signed removal: %w", err)
	}
	fmt.Printf("5. passkey-signed removal: version %d, %d members\n", deref(g.Version), len(g.Members))

	// 6. Stale: sign an add, let another change land first, then submit.
	stale, err := c.PrepareAddSignerToGroup(ctx, groupID, esNew1.PublicKeyBase64())
	if err != nil {
		return err
	}
	staleSigs, err := stale.Sign(ctx, viaPasskey)
	if err != nil {
		return err
	}
	if _, err := c.AddSignerToGroupSigned(ctx, groupID, esNew2.PublicKeyBase64(), viaES256); err != nil {
		return fmt.Errorf("intervening add: %w", err)
	}
	_, err = c.SubmitGovernanceIntent(ctx, stale, staleSigs)
	if !errors.Is(err, client.ErrStaleSignerGroupVersion) {
		return fmt.Errorf("stale submit: want ErrStaleSignerGroupVersion, got %v", err)
	}
	fmt.Printf("6. signatures over version %d refused after a change: stale_signer_group_version\n", stale.SignerGroupVersion)
	return nil
}

func mustP256() *client.P256Signer {
	s, err := client.NewP256Signer()
	if err != nil {
		panic(err)
	}
	return s
}

func deref(v *int64) int64 {
	if v == nil {
		return -1
	}
	return *v
}

// softwarePasskey is a WebAuthn authenticator in software, for this example
// only: authenticatorData = SHA-256(rpID) ‖ flags (user present + verified) ‖
// signCount, signature = ECDSA P-256 over authenticatorData ‖
// SHA-256(clientDataJSON).
type softwarePasskey struct {
	priv  *ecdsa.PrivateKey
	rpID  string
	count uint32
}

func newSoftwarePasskey(rpID string) (*softwarePasskey, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &softwarePasskey{priv: priv, rpID: rpID}, nil
}

// coseKey is the credential public key as registered for a WEBAUTHN signer:
// base64 of the COSE_Key map {1: 2 (EC2), 3: -7 (ES256), -1: 1 (P-256), -2: x, -3: y}.
func (p *softwarePasskey) coseKey() string {
	b := []byte{0xa5, 0x01, 0x02, 0x03, 0x26, 0x20, 0x01, 0x21, 0x58, 0x20}
	b = append(b, p.priv.X.FillBytes(make([]byte, 32))...)
	b = append(b, 0x22, 0x58, 0x20)
	b = append(b, p.priv.Y.FillBytes(make([]byte, 32))...)
	return base64.StdEncoding.EncodeToString(b)
}

// assert plays navigator.credentials.get({publicKey: {challenge, rpId}}).
func (p *softwarePasskey) assert(_ context.Context, challenge []byte) ([]byte, error) {
	p.count++
	rpHash := sha256.Sum256([]byte(p.rpID))
	authData := append(rpHash[:], 0x05)
	authData = binary.BigEndian.AppendUint32(authData, p.count)
	clientData, err := json.Marshal(map[string]any{
		"type":        "webauthn.get",
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
	return json.Marshal(client.WebAuthnAssertion{
		ID:                      b64([]byte("sdk-example-credential")),
		RawID:                   b64([]byte("sdk-example-credential")),
		Type:                    "public-key",
		AuthenticatorAttachment: "platform",
		Response: client.WebAuthnAssertionResponse{
			ClientDataJSON:    b64(clientData),
			AuthenticatorData: b64(authData),
			Signature:         b64(sig),
		},
	})
}
