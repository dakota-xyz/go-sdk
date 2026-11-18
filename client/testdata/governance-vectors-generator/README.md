# Governance intent golden vectors

`../governance_intents.json` holds the canonical bytes policy-engine signs over
for each signer-group governance intent, produced by policy-engine's own
`intent.CanonicalIntentBytes`. `TestGovernanceIntentGoldenVectors` checks that
the SDK's builders produce the same bytes, so a signature made with the SDK
verifies server-side.

policy-engine is a private module on a newer Go, so the SDK does not depend on
it. This generator lives here, outside the Go build (`testdata` is ignored), for
anyone with policy-engine access to re-check the vectors after a policy-engine
change:

```bash
mkdir -p /tmp/vecgen && cp main.go /tmp/vecgen/ && cd /tmp/vecgen
cat > go.mod <<'MOD'
module vecgen

go 1.26.10

require github.com/dakota-xyz/policy-engine v0.0.0

replace github.com/dakota-xyz/policy-engine => /path/to/policy-engine
MOD
cp /path/to/policy-engine/go.sum . && go mod tidy
go run . | jq '.source = "policy-engine pkg/intent.CanonicalIntentBytes at <sha>, generated <date>"' \
  > /path/to/go-sdk/client/testdata/governance_intents.json
```

A changed `want` means policy-engine changed what it signs over: the SDK
builders must change with it before a release.
