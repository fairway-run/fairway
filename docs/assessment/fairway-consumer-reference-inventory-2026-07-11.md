# Fairway consumer-specific reference inventory

Date: 2026-07-11  
Task: FW-332  
Snapshot: pre-change tracked-file census

## Scope

This inventory classifies every tracked occurrence of the legacy consumer names covered by FW-332 before cleanup. Generated portal output, dependencies, local Fairway DB state, and scratch files are excluded. The exact matched source remains in Git history; this artifact records file/line ownership without republishing raw consumer evidence into the docs portal.

## Summary

- archive candidate: 20 occurrence(s)
- archive material: 100 occurrence(s)
- assessment evidence: 50 occurrence(s)
- backlog history: 39 occurrence(s)
- case study: 6 occurrence(s)
- compatibility fixture: 16 occurrence(s)
- core-product defect: 31 occurrence(s)
- current design defect: 57 occurrence(s)
- release history: 8 occurrence(s)
- reusable example defect: 11 occurrence(s)
- total: 338 occurrence(s) across 70 file(s)

## Decision Rules

- Core code, defaults, help, current reference docs, agent roles, generic examples, and portal language are neutralized.
- Consumer compatibility fixtures remain explicit and gain generic normal-path replacements.
- Dated assessments, release notes, backlog history, and archives retain exact identity for provenance.
- A named consumer appears in canonical content only as a labeled validated practice or case study, never as the standalone default.
- Consumer hostname planning moves to archive; current dashboard-sharing guidance becomes deployment-neutral.

## Complete Occurrence Classification

| Location | Class | Disposition |
|---|---|---|
| `CHANGELOG.md:269` | release history | retain |
| `CHANGELOG.md:283` | release history | retain |
| `README.md:131` | case study | retain and label |
| `agents/arch.md:51` | core-product defect | neutralize |
| `cmd/fairway/main.go:2992` | core-product defect | neutralize |
| `cmd/fairway/main.go:2998` | core-product defect | neutralize |
| `cmd/fairway/main.go:11445` | core-product defect | neutralize |
| `cmd/fairway/main.go:11448` | core-product defect | neutralize |
| `cmd/fairway/main.go:11449` | core-product defect | neutralize |
| `cmd/fairway/main.go:11451` | core-product defect | neutralize |
| `cmd/fairway/main_test.go:4551` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6500` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6505` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6506` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6550` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6564` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6568` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6587` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6588` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6589` | compatibility fixture | retain and label |
| `cmd/fairway/main_test.go:6590` | compatibility fixture | retain and label |
| `docs/archive/README.md:20` | archive material | retain |
| `docs/archive/README.md:21` | archive material | retain |
| `docs/archive/README.md:22` | archive material | retain |
| `docs/archive/README.md:23` | archive material | retain |
| `docs/archive/dashboard-redesign-backlog.yaml:769` | archive material | retain |
| `docs/archive/dashboard-redesign-backlog.yaml:787` | archive material | retain |
| `docs/archive/dashboard-redesign-backlog.yaml:885` | archive material | retain |
| `docs/archive/dashboard-redesign-backlog.yaml:887` | archive material | retain |
| `docs/archive/dashboard-redesign.md:51` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:1` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:3` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:24` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:31` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:78` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:80` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:85` | archive material | retain |
| `docs/archive/gpuaas-arc-adoption.md:222` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:1` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:4` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:7` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:11` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:15` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:20` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:22` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:26` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:31` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:34` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:35` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:36` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:77` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:79` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:81` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:96` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:148` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:149` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:170` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:171` | archive material | retain |
| `docs/archive/gpuaas-extraction.md:172` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:1` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:5` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:10` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:14` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:38` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:39` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:48` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:54` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:58` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:106` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:108` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:112` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:115` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:117` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:118` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:119` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:120` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:123` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:125` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:126` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:127` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:128` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:135` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:136` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:137` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:138` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:139` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:140` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:154` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:155` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:156` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:160` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:166` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:173` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:174` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:180` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:181` | archive material | retain |
| `docs/archive/gpuaas-parity-and-gap-assessment-2026-05-29.md:189` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:1` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:3` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:4` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:9` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:11` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:13` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:15` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:25` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:27` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:36` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:42` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:49` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:52` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:54` | archive material | retain |
| `docs/archive/gpuaas-parity-runbook.md:73` | archive material | retain |
| `docs/archive/open-questions.md:3` | archive material | retain |
| `docs/archive/open-questions.md:15` | archive material | retain |
| `docs/archive/open-questions.md:53` | archive material | retain |
| `docs/archive/open-questions.md:81` | archive material | retain |
| `docs/archive/open-questions.md:83` | archive material | retain |
| `docs/archive/open-questions.md:84` | archive material | retain |
| `docs/archive/open-questions.md:86` | archive material | retain |
| `docs/archive/open-questions.md:95` | archive material | retain |
| `docs/archive/open-questions.md:104` | archive material | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:1` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:7` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:9` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:10` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:14` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:15` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:21` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:23` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:48` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:58` | assessment evidence | retain |
| `docs/assessment/ai-cloud-consumer-gap-audit-2026-07-11.md:68` | assessment evidence | retain |
| `docs/assessment/coordination-notification-backlog-audit-2026-06-17.md:51` | assessment evidence | retain |
| `docs/assessment/coordination-notification-backlog-audit-2026-06-17.md:66` | assessment evidence | retain |
| `docs/assessment/coordination-notification-backlog-audit-2026-06-17.md:80` | assessment evidence | retain |
| `docs/assessment/dashboard-redesign-closeout.md:130` | assessment evidence | retain |
| `docs/assessment/dashboard-redesign-closeout.md:134` | assessment evidence | retain |
| `docs/assessment/dashboard-redesign-closeout.md:150` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:10` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:26` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:27` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:32` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:159` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:165` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:171` | assessment evidence | retain |
| `docs/assessment/dashboard-v2-walkthrough.md:177` | assessment evidence | retain |
| `docs/assessment/detailed-design-backlog-backfill-2026-06-18.md:73` | assessment evidence | retain |
| `docs/assessment/detailed-design-backlog-backfill-2026-06-18.md:91` | assessment evidence | retain |
| `docs/assessment/fairway-ai-cloud-data-hygiene-packet-2026-07-11.md:1` | assessment evidence | retain |
| `docs/assessment/fairway-dashboard-contention-2026-07-11.md:12` | assessment evidence | retain |
| `docs/assessment/fairway-dashboard-contention-2026-07-11.md:21` | assessment evidence | retain |
| `docs/assessment/fairway-dashboard-contention-2026-07-11.md:22` | assessment evidence | retain |
| `docs/assessment/fairway-documentation-inventory-2026-07-11.md:42` | assessment evidence | retain |
| `docs/assessment/fairway-documentation-inventory-2026-07-11.md:43` | assessment evidence | retain |
| `docs/assessment/fairway-documentation-inventory-2026-07-11.md:44` | assessment evidence | retain |
| `docs/assessment/fairway-documentation-inventory-2026-07-11.md:45` | assessment evidence | retain |
| `docs/assessment/fairway-pending-state-cleanup-2026-06-08.md:7` | assessment evidence | retain |
| `docs/assessment/fairway-pending-state-cleanup-2026-06-08.md:9` | assessment evidence | retain |
| `docs/assessment/fairway-pending-state-cleanup-2026-06-08.md:24` | assessment evidence | retain |
| `docs/assessment/fairway-release-prep-2026-06-18.md:19` | assessment evidence | retain |
| `docs/assessment/fairway-release-prep-2026-06-18.md:40` | assessment evidence | retain |
| `docs/assessment/fairway-release-prep-2026-06-18.md:66` | assessment evidence | retain |
| `docs/assessment/fairway-release-prep-2026-06-18.md:67` | assessment evidence | retain |
| `docs/assessment/fairway-review-debt-execution-2026-06-10.md:126` | assessment evidence | retain |
| `docs/assessment/fairway-v0.1.12-ai-cloud-dashboard-performance-2026-07-11.md:1` | assessment evidence | retain |
| `docs/assessment/fairway-v0.1.12-ai-cloud-dashboard-performance-2026-07-11.md:20` | assessment evidence | retain |
| `docs/assessment/fairway-v0.1.12-ai-cloud-dashboard-performance-2026-07-11.md:64` | assessment evidence | retain |
| `docs/assessment/fairway-v0.1.12-release-prep-2026-07-11.md:18` | assessment evidence | retain |
| `docs/assessment/fairway-v0.1.9-release-prep-2026-06-30.md:78` | assessment evidence | retain |
| `docs/assessment/memory-only-completion-reconciliation-2026-06-18.md:79` | assessment evidence | retain |
| `docs/assessment/memory-only-completion-reconciliation-2026-06-18.md:87` | assessment evidence | retain |
| `docs/config-reference.md:204` | core-product defect | neutralize |
| `docs/config-reference.md:706` | core-product defect | neutralize |
| `docs/config-reference.md:707` | core-product defect | neutralize |
| `docs/config-reference.md:794` | core-product defect | neutralize |
| `docs/design/backlog-sources.md:73` | current design defect | neutralize or label |
| `docs/design/checkpoints.md:3` | current design defect | neutralize or label |
| `docs/design/cli.md:691` | current design defect | neutralize or label |
| `docs/design/context-packets.md:4` | current design defect | neutralize or label |
| `docs/design/coordination-intelligence.md:9` | current design defect | neutralize or label |
| `docs/design/coordinator-loop.md:4` | current design defect | neutralize or label |
| `docs/design/coordinator-loop.md:273` | current design defect | neutralize or label |
| `docs/design/dashboard-share-hostname-release.md:8` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:9` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:16` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:20` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:24` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:30` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:34` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:38` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:39` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:44` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:48` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:56` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:59` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:65` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:104` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:105` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:106` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:107` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:108` | archive candidate | move to archive |
| `docs/design/dashboard-share-hostname-release.md:109` | archive candidate | move to archive |
| `docs/design/dashboard-sharing.md:20` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:25` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:28` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:29` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:30` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:32` | current design defect | neutralize or label |
| `docs/design/dashboard-sharing.md:33` | current design defect | neutralize or label |
| `docs/design/dashboard.md:54` | current design defect | neutralize or label |
| `docs/design/dashboard.md:119` | current design defect | neutralize or label |
| `docs/design/dashboard.md:200` | current design defect | neutralize or label |
| `docs/design/dashboard.md:292` | current design defect | neutralize or label |
| `docs/design/environment-deploy-preflight.md:16` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:33` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:100` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:135` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:138` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:231` | current design defect | neutralize or label |
| `docs/design/implementation-roadmap.md:236` | current design defect | neutralize or label |
| `docs/design/issue-tracker-integrations.md:169` | current design defect | neutralize or label |
| `docs/design/mockups/fairway-wall-mockup.html:325` | reusable example defect | neutralize |
| `docs/design/mockups/fairway-wall-mockup.html:393` | reusable example defect | neutralize |
| `docs/design/mockups/fairway-wall-mockup.html:813` | reusable example defect | neutralize |
| `docs/design/multi-project.md:23` | current design defect | neutralize or label |
| `docs/design/multi-project.md:24` | current design defect | neutralize or label |
| `docs/design/multi-project.md:47` | current design defect | neutralize or label |
| `docs/design/multi-project.md:58` | current design defect | neutralize or label |
| `docs/design/multi-project.md:59` | current design defect | neutralize or label |
| `docs/design/multi-project.md:60` | current design defect | neutralize or label |
| `docs/design/postgres-adapter.md:4` | current design defect | neutralize or label |
| `docs/design/postgres-adapter.md:16` | current design defect | neutralize or label |
| `docs/design/provider-surface-capability-readiness.md:104` | current design defect | neutralize or label |
| `docs/design/regression-packets.md:3` | current design defect | neutralize or label |
| `docs/design/review-lanes.md:3` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:23` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:100` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:104` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:209` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:210` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:257` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:258` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:259` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:260` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:280` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:452` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:475` | current design defect | neutralize or label |
| `docs/design/rule-packs.md:490` | current design defect | neutralize or label |
| `docs/design/schema.md:451` | current design defect | neutralize or label |
| `docs/design/session-launch.md:178` | current design defect | neutralize or label |
| `docs/design/small-team-autonomy-operating-model.md:6` | case study | retain and label |
| `docs/design/small-team-autonomy-operating-model.md:45` | case study | retain and label |
| `docs/design/small-team-autonomy-operating-model.md:60` | case study | retain and label |
| `docs/design/small-team-autonomy-operating-model.md:218` | case study | retain and label |
| `docs/design/state-machine.md:56` | current design defect | neutralize or label |
| `docs/design/watchers.md:4` | current design defect | neutralize or label |
| `docs/design/work-batch-model.md:23` | current design defect | neutralize or label |
| `docs/design/workstream-profiles.md:235` | current design defect | neutralize or label |
| `docs/design/worktrees.md:3` | current design defect | neutralize or label |
| `docs/docs-portal.md:8` | core-product defect | neutralize |
| `docs/docs-portal.md:124` | core-product defect | neutralize |
| `docs/governance/release.md:145` | core-product defect | neutralize |
| `docs/governance/release.md:410` | core-product defect | neutralize |
| `docs/governance/release.md:437` | core-product defect | neutralize |
| `docs/governance/release.md:467` | core-product defect | neutralize |
| `docs/governance/release.md:468` | core-product defect | neutralize |
| `docs/governance/release.md:469` | core-product defect | neutralize |
| `docs/governance/release.md:470` | core-product defect | neutralize |
| `docs/product.md:51` | case study | retain and label |
| `docs/release-notes.md:217` | release history | retain |
| `docs/release-notes.md:253` | release history | retain |
| `docs/release-notes.md:289` | release history | retain |
| `docs/release-notes.md:359` | release history | retain |
| `docs/release-notes.md:360` | release history | retain |
| `docs/release-notes.md:411` | release history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:45` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:197` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:232` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:1184` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:1209` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:1481` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:2118` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:2516` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:2890` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3122` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3138` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3173` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3174` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3263` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3314` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3315` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3316` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3351` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3384` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3447` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3484` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:3588` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5044` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5056` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5371` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5384` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5402` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5420` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5438` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5456` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5474` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5496` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5659` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5673` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5697` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5708` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5711` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5712` | backlog history | retain |
| `docs/roadmap/fairway-product-backlog.yaml:5713` | backlog history | retain |
| `docs/workstream-profile-guide.md:139` | core-product defect | neutralize |
| `examples/fairway-adoption-improvements.yaml:28` | reusable example defect | neutralize |
| `examples/fairway-adoption-improvements.yaml:180` | reusable example defect | neutralize |
| `examples/fairway-adoption-improvements.yaml:215` | reusable example defect | neutralize |
| `examples/gpuaas-a-b-c-d-e-config.toml:1` | compatibility fixture | retain and label |
| `examples/gpuaas-a-b-c-d-e-config.toml:2` | compatibility fixture | retain and label |
| `examples/gpuaas-a-b-c-d-e-config.toml:6` | compatibility fixture | retain and label |
| `examples/gpuaas-config.toml:1` | compatibility fixture | retain and label |
| `examples/gpuaas-config.toml:5` | compatibility fixture | retain and label |
| `examples/session-adapters/README.md:73` | reusable example defect | neutralize |
| `examples/session-adapters/README.md:79` | reusable example defect | neutralize |
| `internal/audit/audit.go:220` | core-product defect | neutralize |
| `internal/audit/audit.go:259` | core-product defect | neutralize |
| `internal/audit/audit.go:276` | core-product defect | neutralize |
| `internal/audit/audit.go:334` | core-product defect | neutralize |
| `internal/audit/audit.go:335` | core-product defect | neutralize |
| `internal/config/config_test.go:59` | core-product defect | neutralize |
| `internal/reconcile/active_test.go:328` | core-product defect | neutralize |
| `internal/reconcile/active_test.go:357` | core-product defect | neutralize |
| `internal/reconcile/active_test.go:359` | core-product defect | neutralize |
| `internal/reconcile/active_test.go:412` | core-product defect | neutralize |
| `website/static/mockups/fairway-wall-mockup.html:325` | reusable example defect | neutralize |
| `website/static/mockups/fairway-wall-mockup.html:393` | reusable example defect | neutralize |
| `website/static/mockups/fairway-wall-mockup.html:813` | reusable example defect | neutralize |

The inventory file itself necessarily describes the search terms as assessment evidence. Post-change checks exclude `docs/assessment/`, `docs/archive/`, release history, versioned backlog history, and explicitly labeled compatibility fixtures when testing the standalone product surface.

## Post-Change Readback

The second tracked-file census found **211 residual occurrences across 31 files**, reduced from 338 occurrences across 70 files. Every residual occurrence is now assessment evidence, archive material, release history, backlog history, a labeled compatibility fixture/regression, or the deprecated compatibility JSON alias.

The fail-closed check reported no consumer-specific references in current core defaults, help text, agent-role guidance, governance/reference docs, portal source, generic examples, or non-compatibility runtime behavior.

## Replacement And Compatibility Decisions

- `examples/fairway-config.toml` is the generic config starting point.
- `examples/platform-regression-packs.yaml` is the generic regression-pack starting point.
- Existing consumer-named example files remain labeled compatibility fixtures; they no longer supply hidden defaults.
- Adoption route samples now come only from configured workstream profiles. The hidden project-name/role fallback was removed.
- `consumer_lessons` replaces consumer-specific docs-audit terminology. The old JSON key remains deprecated compatibility output; human output is neutral.
- The consumer hostname plan and dated implementation roadmap moved to `docs/archive/` with provenance headers and were removed from current portal navigation.
