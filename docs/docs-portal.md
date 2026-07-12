# Public Docs Portal

Fairway's public docs portal presents the product as the engineering control and
accountability layer for agent-driven delivery. A reader should understand the
accountability chain and complete one bounded work item before encountering
lane, worktree, watcher, or shared-team mechanics.

The portal must stand on public repository evidence. A named consumer may
appear in a clearly labeled case study, but private deployment context is not
required to understand or adopt Fairway.

## Audience Paths

Use these paths when organizing the Docusaurus portal:

| Audience | First docs |
|---|---|
| New user | README, quickstart, product, concepts |
| Evaluator | product, product boundaries, architecture, evidence-backed case study |
| Agent/operator | agent guide, dashboard, workflow guards, checkpoints, context packets |
| Maintainer | architecture, coding standards, testing, release |
| Integrator | integrations, configuration, CLI, provider capability readiness |
| Provenance reader | supply-chain provenance, assessments, archive index |

## Navigation Model

The public sidebar is organized by reader intent rather than package names or
internal feature vocabulary:

| Journey | Reader question | Canonical first page |
|---|---|---|
| Evaluate | What problem does Fairway solve, and where does its authority stop? | [Product](product.md) |
| Get started | Can I get one bounded result without learning the whole model? | [Quickstart](quickstart.md) |
| Use Fairway | How do I run ordinary task, evidence, review, and handoff work? | [Agent guide](agent-guide.md) |
| Operate | How do I run dashboards, reports, environments, and releases safely? | [Dashboard](design/dashboard.md) |
| Integrate | How does Fairway compose with providers, trackers, profiles, and stores? | [Workstream profile guide](workstream-profile-guide.md) |
| Understand | Why are the concepts and authority boundaries designed this way? | [Concepts](design/concepts.md) |
| Reference | What is the exact command, config, schema, or policy contract? | [Configuration reference](config-reference.md) |

The sidebar is intentionally curated. Focused design pages can remain reachable
through links from their canonical owner without becoming top-level navigation.
The dated
[documentation inventory](assessment/fairway-documentation-inventory-2026-07-11.md)
tracks every source artifact, including pages intentionally excluded from the
common path.

## Canonical Ownership

- Product definition and claim status: `docs/product.md`.
- First-value path: `docs/quickstart.md`.
- Operational workflow: `docs/agent-guide.md`.
- Concept definitions: `docs/design/concepts.md`.
- Authority and trust boundaries: `docs/design/product-boundaries.md`.
- Architecture: `docs/architecture.md`.
- CLI, configuration, schema, and state lifecycle: their named reference pages.
- Portal IA and publication: this page.

Supporting pages must link to these owners rather than restating definitions.
Dated assessments may support a claim, but do not replace its canonical owner.

## Public Content Rules

- Keep Fairway's generic product path first.
- Lead with intent, decisions, evidence, independent judgment, and promotion;
  introduce coordination mechanics only when the journey needs them.
- Label substantive claims as implemented, validated practice, experimental,
  planned, or non-goal.
- Treat external reviews and generated narrative as input, not authority or
  provenance.
- Avoid unsupported compliance, certification, adoption, or market claims.
- Keep repo-specific adoption material under archive/provenance navigation, not
  the main getting-started path.
- Do not publish secrets, local env values, customer names, private deployment
  hostnames, or private operational details.
- Document secret names and required permissions, never values.
- Prefer examples under `examples/` over copied private task data.

## Portal Structure

The Docusaurus portal should expose:

- home page: accountable product promise, audience, first action, authority
  boundary, and evidence-backed proof
- docs: quickstart, product, concepts, product boundaries, agent guide,
  integrations, case study, workstream profiles, dashboard, release notes
- reference: config reference, CLI surface, schema, state machine
- governance: release, testing, coding standards, review guards
- release notes: current release candidate scope, known limits, and release
  checklist
- archive: historical decision logs and adoption notes only

Assessments are not a primary navigation category. Canonical pages and case
studies may cite safe assessment artifacts, while raw dated evidence remains a
repository-level provenance surface.

## Consolidation Rule

Before adding a page:

1. identify its reader question and canonical subject owner;
2. extend the canonical page when the question is already owned;
3. add a supporting page only when the material has a distinct operational or
   design contract;
4. move superseded material to `docs/archive/` with provenance rather than
   leaving two current definitions;
5. update the documentation inventory and portal navigation when ownership
   changes.

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
- Credentials from consumer projects or unrelated domains must not be reused.
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
