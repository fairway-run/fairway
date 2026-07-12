# Fairway documentation inventory and disposition

Date: 2026-07-11  
Task: FW-324  
Scope: every tracked documentation and portal-source artifact under `README.md`, `docs/`, and `website/`; generated portal output and dependencies are excluded.

## Decision

Fairway documentation will lead with accountable engineering intent, material decisions, evidence, independent judgment, and explicit promotion. Coordination, lanes, worktrees, watchers, and provider adapters remain capabilities and implementation references rather than the category definition. Existing pages are consolidated before new canonical pages are added.

## Disposition classes

- **Canonical**: one authoritative page for a reader question; other pages link to it instead of restating it.
- **Supporting**: focused design, operations, or implementation reference linked through progressive disclosure.
- **Assessment**: dated evidence or decision input; never presented as current product authority.
- **Release**: versioned release communication and upgrade history.
- **Archive**: historical provenance excluded from normal adoption paths.
- **Portal source**: navigation, presentation, build, or media owned by the public docs portal.

## Claim labels

Public and canonical pages must label substantive capability claims as **implemented**, **validated practice**, **experimental**, **planned**, or **non-goal**. External reviews and generated narrative are inputs, not authority or provenance.

## Consolidation priorities

1. Merge overlapping product/category language across `README.md`, `docs/product.md`, `docs/governed-agentic-engineering.md`, `docs/design/agent-native-product-interface.md`, and `docs/design/scope.md`.
2. Make `docs/design/concepts.md` the concepts map; keep narrow design pages as supporting references rather than repeating definitions.
3. Make `docs/quickstart.md` the tested first-value path and keep deep operation in `docs/agent-guide.md` and focused references.
4. Keep dated assessments and release packets as evidence, outside the primary adopter navigation.
5. Treat `website/` as portal source and `docs/` as content source; do not create a parallel Docusaurus content tree.

## Complete inventory

| Path | Disposition | Canonical subject | Action |
|---|---|---|---|
| `README.md` | Canonical | Product entry point | Rewrite around accountability and the five-minute path. |
| `docs/agent-guide.md` | Canonical | Operator and agent workflow | Retain as the canonical operational guide; remove duplicated product narrative. |
| `docs/architecture.md` | Canonical | System architecture | Retain as the canonical component and data-flow overview. |
| `docs/archive/README.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/dashboard-redesign-backlog.yaml` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/dashboard-redesign.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/gpuaas-arc-adoption.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/gpuaas-extraction.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/gpuaas-parity-runbook.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/archive/open-questions.md` | Archive | Historical provenance | Retain under archive; exclude from normal portal navigation and current capability claims. |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/coordination-notification-backlog-audit-2026-06-17.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/dashboard-performance-budget.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/dashboard-performance-reconciliation-2026-06-07.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/dashboard-redesign-closeout.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/dashboard-v2-walkthrough.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/detailed-design-backlog-backfill-2026-06-18.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-ai-cloud-data-hygiene-packet-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-common-path-pilot-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-common-path-pilot-2026-07-11.sql` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-dashboard-contention-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-documentation-inventory-2026-07-11.md` | Assessment | Documentation program evidence | Retain as the dated FW-324 disposition baseline; update through the documentation program rather than presenting it as evergreen product authority. |
| `docs/assessment/fairway-pending-state-cleanup-2026-06-08.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-product-backlog-reconciliation.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-release-prep-2026-06-18.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-review-debt-execution-2026-06-10.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-review-debt-sweep-2026-06-07.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-small-team-repeat-pilot-2026-07-10.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-small-team-repeat-pilot-packet-2026-07-10.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-small-team-shared-pilot-2026-07-06.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.10-release-prep-2026-07-06.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.12-ai-cloud-dashboard-performance-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.12-release-prep-2026-07-11.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.4-release-run.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.5-release-run.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/fairway-v0.1.9-release-prep-2026-06-30.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/assessment/memory-only-completion-reconciliation-2026-06-18.md` | Assessment | Dated evidence | Retain as evidence; cite selectively from canonical docs and case studies without promoting it to authority. |
| `docs/config-reference.md` | Canonical | Configuration reference | Retain as the canonical exhaustive configuration reference. |
| `docs/design/agent-native-product-interface.md` | Supporting | Product and operating context | Consolidate product-interface claims into product/concepts; retain implementation-specific design detail. |
| `docs/design/automation-candidate-detection.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/backlog-sources.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/checkpoints.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/cli.md` | Canonical | CLI reference | Retain as the canonical command reference; keep adoption prose out. |
| `docs/design/common-path-automation.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/concepts.md` | Canonical | Concept model | Rewrite as the canonical concepts map and route details to focused references. |
| `docs/design/consumer-critical-flow-governance.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/context-packets.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/coordination-intelligence.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/coordinator-loop.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/dashboard-share-hostname-release.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/dashboard-sharing.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/dashboard.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/delivery-resources.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/delivery-velocity-and-overhead.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/environment-deploy-preflight.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/hierarchy.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/implementation-roadmap.md` | Supporting | Product and operating context | Consolidate current planning into the backlog; archive historical roadmap detail with provenance. |
| `docs/design/issue-tracker-integrations.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/live-operation-control-room.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/mockups/fairway-board-mockup.html` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/mockups/fairway-wall-mockup.html` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/multi-project.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/postgres-adapter.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/product-boundaries.md` | Canonical | Authority and trust boundaries | Retain as the canonical authority boundary; reconcile overlapping scope text. |
| `docs/design/provider-notifications.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/provider-surface-capability-readiness.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/provider-usage-accounting.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/regression-packets.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/release-cuts.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/reports.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/review-lanes.md` | Supporting | Product and operating context | Keep as a focused coordination reference; do not lead the adopter journey with lane mechanics. |
| `docs/design/review-policy-profiles.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/review-wait-notification-model.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/rule-packs.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/schema.md` | Canonical | Data model reference | Retain as the canonical schema/read-model reference. |
| `docs/design/scope.md` | Supporting | Product and operating context | Consolidate authority and anti-goals into product-boundaries; keep a compatibility pointer if links require it. |
| `docs/design/session-launch.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/shared-team-concurrency-and-sync.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/shared-team-deployment-operations.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/shared-team-operating-model.md` | Supporting | Product and operating context | Keep as a focused operating model; remove product-positioning duplication and link from concepts. |
| `docs/design/shared-team-server-api.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/sketches/logo/01-channel-mark-dots.svg` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/sketches/logo/01b-channel-mark-chevrons.svg` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/sketches/logo/02-wake-v.svg` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/sketches/logo/03-lane-stack.svg` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/sketches/logo/preview.html` | Supporting | Design asset | Retain as labeled design exploration; do not present as current UI evidence. |
| `docs/design/small-team-autonomy-operating-model.md` | Supporting | Product and operating context | Keep as a focused operating model; remove product-positioning duplication and link from concepts. |
| `docs/design/state-machine.md` | Canonical | Task lifecycle | Retain as the canonical task-state lifecycle reference. |
| `docs/design/supply-chain-provenance.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/task-decision-memory.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/trusted-proxy-identity-verification.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/watchers.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/work-batch-model.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/workstream-profiles.md` | Supporting | Focused design reference | Retain as a focused reference; deduplicate definitions and link from concepts, architecture, or product boundaries. |
| `docs/design/worktrees.md` | Supporting | Product and operating context | Keep as an optional execution-topology reference; do not present worktrees as the product category. |
| `docs/docs-portal.md` | Canonical | Portal ownership and publication | Update as the canonical portal IA, build, publication, and readback guide. |
| `docs/embed.go` | Supporting | Embedded documentation implementation | Retain as implementation glue; not a reader-facing navigation entry. |
| `docs/governance/README.md` | Canonical | Governance index | Retain as the canonical governance entry point and remove duplicate summaries. |
| `docs/governance/coding-standards.md` | Canonical | Coding standards | Retain as canonical governance policy. |
| `docs/governance/commits.md` | Canonical | Commit policy | Retain as canonical governance policy. |
| `docs/governance/release.md` | Canonical | Release policy | Retain as canonical governance policy. |
| `docs/governance/review-guards.md` | Canonical | Review guard policy | Retain as canonical governance policy. |
| `docs/governance/testing.md` | Canonical | Testing policy | Retain as canonical governance policy. |
| `docs/governed-agentic-engineering.md` | Supporting | Product and operating context | Consolidate core thesis into product and concepts; retain only distinct explanatory material. |
| `docs/operations/dashboard-contention-benchmark.md` | Supporting | Operations runbook | Retain as an operator reference; link from the relevant journey and keep environment-specific facts bounded. |
| `docs/operations/plane-local-evaluation.md` | Supporting | Operations runbook | Retain as an operator reference; link from the relevant journey and keep environment-specific facts bounded. |
| `docs/operations/small-team-lab-deployment.md` | Supporting | Operations runbook | Retain as an operator reference; link from the relevant journey and keep environment-specific facts bounded. |
| `docs/product.md` | Canonical | Product definition | Rewrite as the canonical product promise, principles, capability status, and non-goals. |
| `docs/quickstart.md` | Canonical | First-value journey | Rehearse and reduce to a clean five-minute path; move depth to references. |
| `docs/release-highlights.md` | Release | Release communication | Retain in the release journey; remove duplicated evergreen product explanation. |
| `docs/release-notes.md` | Release | Release communication | Retain in the release journey; remove duplicated evergreen product explanation. |
| `docs/roadmap/fairway-product-backlog.yaml` | Canonical | Product backlog | Retain as the canonical versioned backlog mirror; Fairway DB remains runtime truth. |
| `docs/workstream-profile-guide.md` | Canonical | Workstream profile adoption | Retain as the canonical profile adoption guide. |
| `website/docusaurus.config.js` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/package-lock.json` | Portal source | Public docs portal | Retain as portal build dependency metadata. |
| `website/package.json` | Portal source | Public docs portal | Retain as portal build dependency metadata. |
| `website/sidebars.js` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/src/css/custom.css` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/src/pages/index.jsx` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/src/pages/index.module.css` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/static/_headers` | Portal source | Public docs portal | Update in FW-326/FW-331 to implement the canonical navigation and narrative; keep portal behavior read-only. |
| `website/static/img/dashboard/fairway-dashboard-board.png` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/dashboard/fairway-dashboard-reports.png` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/dashboard/fairway-dashboard-task-detail.png` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/dashboard/fairway-dashboard-wall.png` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/logo-lockup.svg` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/logo.svg` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/img/social-card.png` | Portal source | Public docs portal | Retain as portal media; verify provenance, accessibility text, and current UI before publication. |
| `website/static/mockups/fairway-board-mockup.html` | Portal source | Public docs portal | Retain as supporting prototype asset; label as mockup rather than current product evidence. |
| `website/static/mockups/fairway-wall-mockup.html` | Portal source | Public docs portal | Retain as supporting prototype asset; label as mockup rather than current product evidence. |

Inventory count: **131 tracked or newly added artifacts**. Generated `website/build/`, `website/.docusaurus/`, `website/node_modules/`, and local scratch files are intentionally excluded.
