package webhook

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/dakota-xyz/go-sdk/errors"
)

// SignaturesHeader is the HTTP header carrying one signature per Dakota
// signing key, as comma-separated "<key id>:<base64 signature>" entries:
//
//	X-Dakota-Webhook-Signatures: 65b797d6:<base64>, 9c41e0aa:<base64>
//
// It usually carries one entry, and two while Dakota rotates its signing key.
// Every entry signs the same message as [SignatureHeader]: the timestamp
// followed by the body. Prefer it over [SignatureHeader], which carries only
// the current key and so switches keys at cutover with no overlap.
const SignaturesHeader = "X-Dakota-Webhook-Signatures"

// keyIDLength is the number of hex characters of a public key that name it
// in [SignaturesHeader].
const keyIDLength = 8

// KeyID returns the id that names publicKey in [SignaturesHeader]: the first
// 8 hex characters of the key, in lowercase. Key ids are safe to log. For a
// key shorter than 4 bytes (never a valid Ed25519 key) it returns the whole
// key in hex.
func KeyID(publicKey ed25519.PublicKey) string {
	h := hex.EncodeToString(publicKey)
	if len(h) < keyIDLength {
		return h
	}
	return h[:keyIDLength]
}

// ParsePublicKeys decodes hex-encoded Ed25519 public keys. Each argument may
// itself be a comma-separated list, such as the value of a
// DAKOTA_WEBHOOK_PUBLIC_KEYS environment variable. Spaces around a key and
// empty list items are ignored, and a key given twice is kept once.
//
// It returns an error with code [errors.CodeInvalidConfig] if a key is not 64
// hex characters, or if no key is given. Errors name the key's position, never
// its value, so a private key pasted by mistake does not reach a log.
func ParsePublicKeys(lists ...string) ([]ed25519.PublicKey, error) {
	keys, err := parsePublicKeyList(lists...)
	if err != nil {
		return nil, err
	}
	if len(keys) == 0 {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"at least one webhook public key is required",
		)
	}
	return keys, nil
}

// parsePublicKeyList is [ParsePublicKeys] without the at-least-one check.
func parsePublicKeyList(lists ...string) ([]ed25519.PublicKey, error) {
	var keys []ed25519.PublicKey
	seen := make(map[string]bool)
	position := 0
	for _, list := range lists {
		for _, value := range strings.Split(list, ",") {
			value = strings.TrimSpace(value)
			if value == "" {
				continue
			}
			position++
			raw, err := hex.DecodeString(value)
			if err != nil || len(raw) != ed25519.PublicKeySize {
				return nil, errors.New(
					errors.CodeInvalidConfig,
					fmt.Sprintf(
						"webhook public key %d is not 64 hex characters (a hex-encoded Ed25519 public key)",
						position,
					),
				)
			}
			if seen[string(raw)] {
				continue
			}
			seen[string(raw)] = true
			keys = append(keys, ed25519.PublicKey(raw))
		}
	}
	return keys, nil
}

// Verifier verifies Dakota webhook signatures against a list of trusted
// public keys, so an integration keeps working while Dakota rotates its
// signing key: trust the current key and, once Dakota publishes it, the next
// one; remove the old key when Dakota retires it.
//
// A Verifier never changes after [NewVerifier] returns it, so it is safe for
// concurrent use.
type Verifier struct {
	// keys holds the trusted keys in the order given.
	keys []ed25519.PublicKey
	// byID maps a key id to every trusted key with that id. Two keys share
	// an id only by a 1-in-2^32 chance; both are then checked.
	byID map[string][]ed25519.PublicKey
}

// NewVerifier returns a Verifier that trusts the given hex-encoded Ed25519
// public keys. Each argument may be one key or a comma-separated list, so both
// of these work:
//
//	v, err := webhook.NewVerifier(os.Getenv("DAKOTA_WEBHOOK_PUBLIC_KEYS"))
//	v, err := webhook.NewVerifier(currentKeyHex, nextKeyHex)
//
// See [ParsePublicKeys] for the accepted format and errors.
func NewVerifier(publicKeysHex ...string) (*Verifier, error) {
	keys, err := ParsePublicKeys(publicKeysHex...)
	if err != nil {
		return nil, err
	}
	return newVerifierFromKeys(keys), nil
}

func newVerifierFromKeys(keys []ed25519.PublicKey) *Verifier {
	v := &Verifier{
		keys: keys,
		byID: make(map[string][]ed25519.PublicKey, len(keys)),
	}
	for _, key := range keys {
		id := KeyID(key)
		v.byID[id] = append(v.byID[id], key)
	}
	return v
}

// KeyIDs returns the ids of the trusted keys, in the order given. They are
// safe to log, for example at startup to confirm which keys are configured.
func (v *Verifier) KeyIDs() []string {
	ids := make([]string, len(v.keys))
	for i, key := range v.keys {
		ids[i] = KeyID(key)
	}
	return ids
}

// Verify checks that payload was signed by a trusted key. signatures is the
// value of [SignaturesHeader] and legacySignature the value of
// [SignatureHeader]; timestamp is the value of [TimestampHeader]. Verify does
// not check the timestamp's age: use [ValidateTimestamp], or
// [Verifier.ConstructEvent], which does both.
//
// When signatures is not blank, Verify splits it on ",", trims spaces around
// each entry, and splits each entry at its first ":" into a key id and a
// base64 signature (spaces around either are ignored). Entries whose key id (compared in lowercase) is not
// trusted are skipped. Each trusted key is checked at most once: the first
// entry naming it is the one checked, and later entries naming it are
// skipped, so a request costs at most one signature check per trusted key
// however many entries it carries. The payload is accepted if any check
// passes. Entry order carries no meaning.
//
// Only when signatures is blank does Verify fall back to legacySignature,
// checking it against each trusted key. A signatures header that is present
// but fails is rejected without consulting legacySignature.
//
// It returns an error with code [errors.CodeMissingHeader] if both signature
// values are blank, and [errors.CodeInvalidSignature] if no trusted key
// verifies. The message says whether the header named no trusted key (for
// example, Dakota signs with a key you have not added yet) or a trusted key's
// signature failed; it names only trusted key ids, which are safe to log.
//
// Prefer [Verifier.VerifyHeaders] or [Verifier.ConstructEvent], which read the
// headers themselves: the three string arguments here are easy to swap.
func (v *Verifier) Verify(
	payload []byte,
	timestamp string,
	signatures string,
	legacySignature string,
) error {
	message := append([]byte(timestamp), payload...)

	if strings.TrimSpace(signatures) != "" {
		checked := make(map[string]bool, len(v.byID))
		for entry := range strings.SplitSeq(signatures, ",") {
			id, sig, ok := strings.Cut(strings.TrimSpace(entry), ":")
			if !ok {
				continue
			}
			id = strings.ToLower(strings.TrimSpace(id))
			keys := v.byID[id]
			if len(keys) == 0 || checked[id] {
				continue
			}
			checked[id] = true
			if raw, ok := decodeSignature(sig); ok {
				for _, key := range keys {
					if ed25519.Verify(key, message, raw) {
						return nil
					}
				}
			}
			if len(checked) == len(v.byID) {
				break // every trusted key has had its one check
			}
		}
		if len(checked) == 0 {
			return errors.New(
				errors.CodeInvalidSignature,
				SignaturesHeader+" names none of the trusted keys "+v.trustedIDs(),
			)
		}
		ids := make([]string, 0, len(checked))
		for id := range checked {
			ids = append(ids, id)
		}
		slices.Sort(ids)
		return errors.New(
			errors.CodeInvalidSignature,
			"signature by trusted key "+strings.Join(ids, ", ")+" did not verify",
		)
	}

	legacySignature = strings.TrimSpace(legacySignature)
	if legacySignature == "" {
		return errors.New(
			errors.CodeMissingHeader,
			"missing "+SignaturesHeader+" and "+SignatureHeader+" headers",
		)
	}
	if raw, ok := decodeSignature(legacySignature); ok {
		for _, key := range v.keys {
			if ed25519.Verify(key, message, raw) {
				return nil
			}
		}
	}
	return errors.New(
		errors.CodeInvalidSignature,
		SignatureHeader+" did not verify with any trusted key "+v.trustedIDs(),
	)
}

// trustedIDs names the trusted keys for an error message. It never names a
// key id taken from the request, which an attacker controls.
func (v *Verifier) trustedIDs() string {
	return "(trusted key ids: " + strings.Join(v.KeyIDs(), ", ") + ")"
}

// VerifyHeaders is [Verifier.Verify] with the values read from header. Several
// [SignaturesHeader] lines (a proxy may split the list) are joined with ",",
// and empty lines are ignored, so a header whose lines are all empty counts as
// missing. Like Verify, it does not check the timestamp's age; use
// [Verifier.ConstructEvent] for that. It returns an error with code
// [errors.CodeMissingHeader] if [TimestampHeader] is missing.
func (v *Verifier) VerifyHeaders(payload []byte, header http.Header) error {
	timestamp := header.Get(TimestampHeader)
	if timestamp == "" {
		return errors.New(
			errors.CodeMissingHeader,
			"missing "+TimestampHeader+" header",
		)
	}
	return v.Verify(
		payload,
		timestamp,
		signaturesHeaderValue(header),
		header.Get(SignatureHeader),
	)
}

// ConstructEvent verifies the request's signature headers against the trusted
// keys, validates the timestamp, and parses the event. It is the multi-key
// counterpart of the package-level [ConstructEvent], and accepts the same
// options.
func (v *Verifier) ConstructEvent(
	payload []byte,
	header http.Header,
	opts ...ConstructOption,
) (Event, error) {
	cfg := constructConfig{
		tolerance: DefaultTimestampTolerance,
	}
	for _, opt := range opts {
		opt(&cfg)
	}

	if err := v.VerifyHeaders(payload, header); err != nil {
		return Event{}, err
	}

	if !cfg.ignoreTolerance {
		if err := ValidateTimestamp(
			header.Get(TimestampHeader),
			cfg.tolerance,
		); err != nil {
			return Event{}, err
		}
	}

	return ParseEvent(payload)
}

// signaturesHeaderValue returns the [SignaturesHeader] lines of header joined
// with ",", leaving out empty lines.
func signaturesHeaderValue(header http.Header) string {
	var lines []string
	for _, line := range header.Values(SignaturesHeader) {
		if strings.TrimSpace(line) != "" {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, ",")
}

// decodeSignature decodes a base64 Ed25519 signature, rejecting any value
// that is not exactly one signature long before decoding it.
func decodeSignature(signatureB64 string) ([]byte, bool) {
	signatureB64 = strings.TrimSpace(signatureB64)
	if base64.StdEncoding.DecodedLen(len(signatureB64)) > ed25519.SignatureSize+2 {
		return nil, false
	}
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return nil, false
	}
	return sig, true
}
