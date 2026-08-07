# Quality Workspace

The dashboard Quality workspace is the portfolio-level entry point for
Fairway's cited Quality Record. It answers a different question from the other
dashboard views:

| View | Primary question |
|---|---|
| Wall | What is active or blocked now? |
| Board | Which tasks are moving through the configured workflow? |
| Quality | What lifecycle evidence exists for each bounded work item? |
| Reports | What changed during a delivery window? |
| Controls | Do specific recorded controls discriminate observable outcomes? |
| Diagnostics | Is the coordination/runtime machinery healthy? |

## Portfolio Projection

`/quality` shows one row per visible task and one column per Quality Record
stage:

1. intent;
2. material decisions;
3. production context;
4. collected evidence;
5. automatic verification;
6. human judgment;
7. promotion;
8. operational outcomes; and
9. lessons and controlled improvement.

The workspace preserves the canonical states `present`, `missing`,
`unavailable`, `conflicting`, and `externally_owned`. It does not collapse them
into a score. Every stage cell links to the task-level Quality Record, where the
underlying fact and source references remain inspectable.

The default view is bounded and paginated. Search, task status, role, profile,
and risk filters are applied before the visible page is projected. Global
session and checkpoint context is loaded once per page projection, and the
result participates in the dashboard's short-lived read-model cache.

## Interpretation

- `missing` means an expected Fairway record is absent.
- `unavailable` means Fairway lacks an authoritative fact; it is not evidence
  that no event or outcome occurred.
- `conflicting` retains contradictory evidence for accountable interpretation.
- `externally_owned` identifies authority retained by Git, CI/CD, deployment
  systems, reviewers, operators, or another named system.

The workspace is read-only. It cannot approve, waive, merge, deploy, release,
accept risk, or mutate workflow. Control-effectiveness analysis remains on the
Controls page so lifecycle completeness and statistical discrimination are not
presented as the same claim.
