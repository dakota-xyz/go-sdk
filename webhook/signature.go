package webhook

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/hex"

	"github.com/dakota-xyz/go-sdk/errors"
)

const (
	// SignatureHeader is the HTTP header containing the base64-encoded
	// Ed25519 signature.
	SignatureHeader = "X-Webhook-Signature"

	// TimestampHeader is the HTTP header containing the unix timestamp
	// (seconds) as a string.
	TimestampHeader = "X-Webhook-Timestamp"
)

// VerifySignature verifies an Ed25519 webhook signature.
//
// The signed message is the concatenation of the timestamp string bytes and
// the payload bytes, with no delimiter. This matches Dakota Platform's signing
// implementation.
func VerifySignature(
	payload []byte,
	signatureB64 string,
	timestampStr string,
	publicKeyHex string,
) error {
	pubKey, err := ParsePublicKey(publicKeyHex)
	if err != nil {
		return err
	}

	return verifySignatureWithPublicKey(
		payload,
		signatureB64,
		timestampStr,
		pubKey,
	)
}

func verifySignatureWithPublicKey(
	payload []byte,
	signatureB64 string,
	timestampStr string,
	publicKey ed25519.PublicKey,
) error {
	sig, err := base64.StdEncoding.DecodeString(signatureB64)
	if err != nil {
		return errors.Wrap(
			errors.CodeInvalidSignature,
			"failed to decode signature",
			err,
		)
	}

	if len(sig) != ed25519.SignatureSize {
		return errors.New(
			errors.CodeInvalidSignature,
			"invalid signature length",
		)
	}

	message := append([]byte(timestampStr), payload...)
	if !ed25519.Verify(publicKey, message, sig) {
		return errors.New(
			errors.CodeInvalidSignature,
			"signature verification failed",
		)
	}

	return nil
}

// ComputeSignature creates a base64-encoded Ed25519 signature for the given
// timestamp and payload. This is useful for testing or for producers.
func ComputeSignature(
	timestamp string,
	payload []byte,
	privateKey ed25519.PrivateKey,
) string {
	message := append([]byte(timestamp), payload...)
	sig := ed25519.Sign(privateKey, message)
	return base64.StdEncoding.EncodeToString(sig)
}

// ParsePublicKey decodes a hex-encoded Ed25519 public key (32 bytes).
func ParsePublicKey(hexKey string) (ed25519.PublicKey, error) {
	keyBytes, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, errors.Wrap(
			errors.CodeInvalidConfig,
			"failed to decode public key hex",
			err,
		)
	}

	if len(keyBytes) != ed25519.PublicKeySize {
		return nil, errors.New(
			errors.CodeInvalidConfig,
			"public key must be 32 bytes",
		)
	}

	return ed25519.PublicKey(keyBytes), nil
}
