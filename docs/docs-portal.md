# Public Docs Portal

Fairway's public docs portal should make the product understandable without any
GPUaaS, Core42, or private-track context.

## Audience Paths

Use these paths when organizing the Docusaurus portal:

| Audience | First docs |
|---|---|
| New user | README, quickstart, product, concepts |
| Agent/operator | agent guide, dashboard, checkpoints, context packets, review lanes |
| Maintainer | architecture, coding standards, testing, release |
| Evaluator | scope, product, workstream profile guide, adoption artifact flow |
| Case-study reader | GPUaaS adoption, extraction, parity runbook |

## Public Content Rules

- Keep Fairway's generic product path first.
- Keep GPUaaS material under adoption/case-study navigation, not the main
  getting-started path.
- Do not publish secrets, local env values, customer names, private deployment
  hostnames, or GPUaaS-private operational details.
- Document secret names and required permissions, never values.
- Prefer examples under `examples/` over copied private task data.

## Portal Structure

The Docusaurus portal should expose:

- home page: what Fairway is, why it exists, install path, dashboard screenshots
- docs: quickstart, concepts, agent guide, workstream profiles, dashboard
- reference: config reference, CLI surface, schema, state machine
- governance: release, testing, coding standards, review guards
- adoption: GPUaaS as a public case study and parity process

## Cloudflare Requirements

The portal is intended for Cloudflare Pages on `fairway.run`.

Current production site:

- `https://fairway.run`
- `https://www.fairway.run`
- `https://docs.fairway.run`
- Cloudflare Pages project: `fairway-docs`

Required credential separation:

- Fairway Cloudflare token is stored locally in `.env.cloudflare.fairway-run`
  and in GitHub Actions secrets for CI.
- Core42/Core42.dev credentials must not be reused.
- Account-level Pages permission is separate from zone-level DNS permission.
- Routine GitHub Actions deploys use `FAIRWAY_CLOUDFLARE_API_TOKEN`,
  `FAIRWAY_CLOUDFLARE_ACCOUNT_ID`, and `FAIRWAY_PAGES_PROJECT`.
- DNS/custom-domain setup also needs `FAIRWAY_CLOUDFLARE_ZONE_ID` and a token
  with DNS write scoped only to `fairway.run`.

Required edge checks:

- custom domains route correctly for `fairway.run`, `www.fairway.run`, and
  `docs.fairway.run`
- Pages preview hosts are not indexed when public previews are enabled
- security headers are present
- bot protection posture is explicitly configured or documented
- normal docs browsing, legitimate search crawlers, GitHub release links, and
  Homebrew install paths are not broken by WAF/bot rules

Current bot-management posture for `fairway.run`:

- Bot Fight Mode enabled
- AI Scrapers and Crawlers blocked
- JavaScript detections enabled
- Cloudflare managed robots.txt enabled

## Local Development

```bash
cd website
npm install
npm run dev
```

Build the static portal:

```bash
cd website
npm run build
```

The portal reads documentation from `../docs`, so changes to the repo docs are
reflected in the Docusaurus build without copying Markdown into `website/`.

## Cloudflare Deployment

The deploy workflow lives at `.github/workflows/docs-portal.yml`.

Expected GitHub secrets:

| Secret | Used by |
|---|---|
| `FAIRWAY_CLOUDFLARE_API_TOKEN` | Wrangler Pages deploy. |
| `FAIRWAY_CLOUDFLARE_ACCOUNT_ID` | Wrangler account selection. |
| `FAIRWAY_PAGES_PROJECT` | Cloudflare Pages project name. |
| `FAIRWAY_CLOUDFLARE_ZONE_ID` | Setup/admin workflows and custom-domain verification. |

The workflow builds on docs/site changes, uploads the static build as an
artifact, and deploys `website/build` to Cloudflare Pages from `main`.

## Security Headers

Cloudflare Pages receives headers from `website/static/_headers`.

Current policy includes:

- frame denial
- content-type sniffing prevention
- strict referrer policy
- permissions policy disabling unused browser capabilities
- CSP suitable for the generated static Docusaurus assets
- `X-Robots-Tag: noindex` for Pages preview hosts

## Dependency Review

Run this after dependency changes and before release/deploy work:

```bash
cd website
npm audit --omit=dev --audit-level=high
npm run build
```

The portal uses npm `overrides` for vulnerable transitive packages when the
upstream Docusaurus dependency chain has not yet moved to the patched version.
Keep overrides narrow and remove them when upstream packages catch up.
