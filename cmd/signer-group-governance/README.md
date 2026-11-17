# Signer-group governance example (sandbox)

Runs the signed signer-group flow end to end against the Dakota **sandbox**:

1. an unsigned member add is refused (`endorsement_required`);
2. a passkey-signed add and an ES256-signed removal succeed;
3. an ES256-signed add and a passkey-signed removal succeed;
4. a signature over an old group version is refused (`stale_signer_group_version`).

```bash
DAKOTA_API_KEY=<sandbox API key> go run ./cmd/signer-group-governance
```

It prints each step and ends with `PASS` or `FAIL`.

**Use an API key of an ordinary sandbox client.** Never run it with a key of an
internal superadmin client: there, every key acts as a superadmin.

What it creates in that client: four throwaway signers (one WEBAUTHN, three
ES256) and one signer group with `governance_threshold: 1`, all named
`sdk-governance-…-<hhmmss>`. It creates no API keys and touches nothing that
existed before. The environment is hard-wired to sandbox.

The passkey is a software authenticator so the example runs without a browser.
In a real integration, the passkey lives on the user's device: the function you
give `client.WebAuthnSigner` passes the challenge it receives to
`navigator.credentials.get` unchanged and returns the credential's JSON
(`credential.toJSON()`). The platform may restrict which RP IDs it accepts for
your client.
