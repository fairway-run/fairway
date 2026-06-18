# Dashboard Sharing

Fairway can serve a shared read-only dashboard for small-team visibility. The
product feature is generic: Fairway can disable dashboard mutations and document
safe operation behind an identity-aware proxy. The user or project owns the
domain, proxy provider, identity policy, tunnel connector, and allowlists.

## Product Boundary

Fairway owns:

- shared/read-only dashboard mode;
- blocking dashboard mutation endpoints while shared mode is enabled;
- safe defaults for local origins;
- documentation for trusted proxy and identity header boundaries;
- audit/logging expectations for dashboard actions.

The user or project owns:

- the public hostname, such as `fairway.core42.dev`;
- Cloudflare account, zone, Access application, and Tunnel connector;
- allowed users, email domains, and identity provider choice;
- operational teardown when sharing is no longer needed.

`fairway.core42.dev` is only an example user/project hostname. It is not a
Fairway product default.

For the current Core42 AI Cloud dashboard-share naming work, prefer an
AI Cloud-aligned Fairway hostname such as `fairway.aicloud.core42.dev` when the
`aicloud.core42.dev` subdomain can be managed by the deployment owner. If that
subdomain is not available, use a concrete neutral fallback such as
`aicloud-fairway.core42.dev` in the existing zone. Keep any historical
consumer-specific hostname, such as `fairway-gpuaas.core42.dev`, as a temporary
compatibility alias until the new hostname has passing DNS, tunnel, and
Access-policy evidence. See
[Dashboard Share Hostname Release Plan](dashboard-share-hostname-release.md).

## Reference Config

Bind the Fairway origin to localhost and enable read-only mode:

```toml
[dashboard]
listen = "127.0.0.1:7878"
auto_open = false
read_only = true
trusted_proxy = "cloudflare_access"
```

`trusted_proxy = "cloudflare_access"` is deployment metadata for operators and
docs. This slice does not verify Cloudflare Access JWTs in core Fairway. Treat
identity headers as advisory unless the origin is reachable exclusively through
the trusted tunnel and JWT verification has been added or performed upstream.
The planned Fairway-side verifier model is defined in
[Trusted Proxy Identity Verification](trusted-proxy-identity-verification.md).

## Cloudflare Access Reference Pattern

1. Start Fairway locally:

   ```bash
   fairway dashboard start --listen 127.0.0.1:7878 --read-only --no-open
   fairway dashboard status --listen 127.0.0.1:7878
   ```

2. Create a Cloudflare Tunnel that forwards the chosen hostname to
   `http://127.0.0.1:7878`.
3. Create a Cloudflare Access application for the hostname.
4. Use One-Time PIN or the chosen IdP as the login method.
5. Add explicit named-email allowlist entries, for example:

   ```text
   alice@example.com
   bob@example.com
   ```

6. Add email-domain rules only when appropriate for the project boundary:

   ```text
   example.com
   ```

7. Confirm the dashboard is reachable only through the tunnel. Do not expose the
   Fairway dashboard directly to the public internet.

For release restarts, record the `fairway dashboard status` version and binary
path before and after `fairway dashboard restart`. The shared dashboard is still
read-only after restart; version readback is an operator confidence check, not a
grant of send, approval, merge, deploy, or execution authority.

## Trust Boundary

Cloudflare Access headers, or any identity-aware proxy headers, are trustworthy
only if the Fairway origin cannot be reached except through that proxy. A local
origin bound to `127.0.0.1` and reached through Cloudflare Tunnel is the
reference pattern.

Before trusting identity headers for authorization beyond local/dev sharing,
verify Cloudflare Access JWTs or perform authorization in a trusted upstream
proxy. Fairway shared mode currently blocks writes rather than authorizing them.
Future write access must be an explicit opt-in with authorization and audit
requirements. Until the verifier model is implemented, dashboard identity
display remains advisory and read-only.

## Audit And Logging

Read-only mode blocks dashboard mutation handlers before task state changes or
dashboard audit writes. Operators should still keep Cloudflare Access logs for:

- authenticated viewer identity;
- login method;
- hostname and path;
- denied attempts;
- tunnel connector health.

Use Fairway CLI audit and evidence records for actual task state changes made
from trusted local worktrees.

## Teardown

When sharing is no longer needed:

1. Stop the Fairway dashboard process.
2. Stop or delete the Cloudflare Tunnel connector.
3. Disable or delete the Cloudflare Access application.
4. Remove temporary named-email/domain allowlist entries.
5. Rotate any exposed local operational notes if they included sensitive
   deployment details.

When retiring a compatibility hostname, also record the replacement hostname,
viewer communication, DNS/Access evidence, and rollback plan in Fairway before
removing the old route.
