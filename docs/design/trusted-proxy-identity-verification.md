# Trusted Proxy Identity Verification Model

Fairway can run a shared read-only dashboard behind an identity-aware proxy, but
the current `trusted_proxy` value is deployment metadata only. Fairway must not
trust identity headers for authorization or audit until it can verify that the
headers came from the configured proxy and were bound to a verified identity.

This model defines the implementation boundary for a future dashboard-security
slice. It does not enable dashboard mutation, provider wake, approval, merge,
deploy, release, or live-operation authority.

## Current Boundary

- `read_only = true` disables dashboard mutation handlers.
- `trusted_proxy = "cloudflare_access"` documents the deployment pattern.
- Operators must bind the Fairway origin to loopback and expose it only through
  a tunnel or trusted reverse proxy.
- Cloudflare Access or another identity-aware proxy owns user authentication,
  allowlists, identity provider policy, and access logs.
- Fairway treats identity headers as advisory text unless a verifier is
  configured and passes.

## Verification Goals

A trusted proxy verifier should make these facts explicit before Fairway displays
or records viewer identity:

- proxy type, issuer, audience, and key source;
- which headers carry identity and signed proof;
- whether verification is enabled, disabled, or report-only;
- whether missing or invalid proof fails closed;
- which identity fields are safe to show in read-only dashboard diagnostics;
- which audit rows are recorded for accepted, missing, or failed verification.

The verifier should support Cloudflare Access first because that is the current
shared-dashboard reference deployment. A generic identity-aware-proxy verifier
can follow once the Cloudflare path is proven.

## Candidate Config Shape

```toml
[dashboard]
read_only = true
trusted_proxy = "cloudflare_access"

[dashboard.trusted_identity]
mode = "report_only"                  # disabled | report_only | enforce
provider = "cloudflare_access"        # cloudflare_access | oidc_proxy
issuer = "https://<team>.cloudflareaccess.com"
audience = ["<access-app-aud>"]
jwks_url = "https://<team>.cloudflareaccess.com/cdn-cgi/access/certs"
jwt_header = "Cf-Access-Jwt-Assertion"
email_header = "Cf-Access-Authenticated-User-Email"
name_header = "Cf-Access-Authenticated-User-Name"
allowed_email_domains = ["example.com"]
fail_closed = true
```

The config must not contain private keys, client secrets, service tokens,
cookies, or bearer token values.

## Runtime Behavior

- `disabled`: current behavior; dashboard remains read-only when configured, and
  identity headers are not trusted.
- `report_only`: verify when proof is present, expose verification status in
  diagnostics, and record audit findings for missing or invalid proof without
  blocking the dashboard response.
- `enforce`: reject dashboard requests that lack valid proof, do not match the
  configured issuer/audience, or fail allowed-domain checks.

All modes must keep read-only dashboard semantics. Verification affects viewer
identity confidence only; it does not add dashboard write authority.

## Audit And Diagnostics

The dashboard status and diagnostics surfaces should show:

- trusted identity mode and provider;
- whether the current request was verified, missing proof, invalid, or outside
  allowed policy;
- sanitized identity fields, such as email domain or redacted email, only when
  verified;
- the verification failure class without dumping raw JWTs, headers, cookies, or
  tokens.

Audit rows should record bounded metadata: provider, mode, result, reason code,
and redacted identity. They must not store raw signed assertions.

## Implementation Acceptance

A future implementation slice should:

- extend config parsing and validation for `[dashboard.trusted_identity]`;
- implement Cloudflare Access JWT verification using issuer, audience, and JWKS;
- fail closed in `enforce` mode and stay non-blocking in `report_only` mode;
- add request tests for valid proof, missing proof, bad audience, bad issuer,
  expired token, malformed headers, and read-only mutation blocking;
- add dashboard status/diagnostics readback without raw token/header storage;
- update dashboard-sharing, config reference, and operator docs;
- preserve dashboard read-only boundaries and avoid provider send authority.

