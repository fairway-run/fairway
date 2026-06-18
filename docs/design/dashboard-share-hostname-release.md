# Dashboard Share Hostname Release Plan

Fairway's shared dashboard feature is product-neutral. A consumer may expose the
read-only dashboard through Cloudflare Access, Tailscale, Pomerium, or another
identity-aware proxy, but the hostname and proxy policy are deployment-owned.

This plan covers the Fairway-side naming and release work for the current
Core42 AI Cloud public dashboard/share surface. Product-facing language should
say "AI Cloud"; use "GPUaaS" only for repository, module, history, internal, or
compatibility references.

## Hostname Decision

Recommended target:

- `fairway.aicloud.core42.dev`

Reason:

- It reads as a Fairway dashboard under the AI Cloud area rather than as a
  legacy consumer-specific surface.
- It keeps the Fairway product capability separate from any one consumer
  program.
- It leaves room for additional AI Cloud Fairway surfaces under the same
  subdomain if the organization chooses to manage them there.

Observed blocker:

- DNS lookup from the Fairway implementation lane found no public records for
  `fairway.aicloud.core42.dev`.

Fallback target:

- `aicloud-fairway.core42.dev`

Fallback reason:

- It stays product-neutral while fitting the existing `core42.dev` zone shape
  if `aicloud.core42.dev` is not delegated or not manageable by the current
  Cloudflare operator.

Existing compatibility hostname:

- `fairway-gpuaas.core42.dev`

Current observation:

- `fairway-gpuaas.core42.dev` resolves through Cloudflare A/AAAA records.

## Compatibility And Deprecation

Use a two-name compatibility window:

1. Bring up the neutral hostname behind the same read-only dashboard origin and
   Access policy, or behind a reviewed equivalent policy.
2. Keep `fairway-gpuaas.core42.dev` available as an alias while active users and
   bookmarks migrate.
3. Update Fairway docs, release notes, and consumer handoff packets to prefer
   the AI Cloud-aligned neutral hostname.
4. Record a teardown checkpoint before retiring the older compatibility
   hostname.
5. Remove the compatibility hostname only after the replacement hostname has
   passing reachability and Access-policy evidence.

Do not redirect or remove `fairway-gpuaas.core42.dev` without a deployment-owner
task and viewer communication. Fairway can provide the plan and evidence shape;
the Cloudflare account, DNS zone, tunnel, Access application, and allowlists are
deployment-owned.

## Fairway Release Impact

The hostname change is a deployment/docs update, not a binary packaging change.

No GoReleaser or Homebrew cask change is required unless a future release starts
embedding a default public dashboard URL in the binary or packaging metadata.
The current Fairway dashboard command accepts `--listen`, `--read-only`, and
`--no-open`; it does not encode a public hostname.

Release notes should mention the new neutral reference hostname when the
deployment owner is ready to advertise it. Until then, docs should describe the
target and compatibility plan without claiming the DNS record or Access app is
already active.

## Trust Boundary

The shared dashboard remains read-only. This plan does not add:

- dashboard mutation authority;
- provider prompt or wake send authority from the dashboard;
- review approval authority;
- merge, deploy, release, or live-operation authority;
- Cloudflare API integration in Fairway core;
- storage of Cloudflare tokens or Access credentials.

Fairway records task/evidence state and serves read-only projections. Operators
must make DNS, Cloudflare Tunnel, and Cloudflare Access changes in the
deployment-owned environment.

## Verification Commands

Read-only DNS observations:

```bash
dig +short fairway.aicloud.core42.dev A
dig +short fairway.aicloud.core42.dev AAAA
dig +short aicloud-fairway.core42.dev A
dig +short aicloud-fairway.core42.dev AAAA
dig +short fairway-gpuaas.core42.dev A
dig +short fairway-gpuaas.core42.dev AAAA
```

Fairway local smoke:

```bash
fairway dashboard --listen 127.0.0.1:7878 --read-only --no-open
fairway workflow check
```
