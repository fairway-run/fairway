# Review Policy Profiles

Fairway supports configurable, risk-scaled review profiles so consumer repos do
not have to apply the same review matrix to every slice. The goal is to spend
review effort where it changes risk and cycle time: epic, live-window, deploy,
production-readiness, compliance, enforcement, credential, and public exposure
boundaries get full review; small docs, harness, setup, readback, classifier,
or provider-shape fixes inside approved non-live boundaries can use lightweight
or grouped review.

Process is useful only when it improves speed, quality, or safety. New review
or gate policies should start as bounded pilots in `advisory` mode with a
stated hypothesis before becoming blocking defaults.

Profiles can come from Fairway's built-in defaults or from
`.fairway/config.toml` `[[review_profiles]]` entries. They are deterministic
policy inputs for `merge-ready`, `task-detail`, `review-waits`, and coordinator
plan output. They are not approval authority. They do not authorize live
execution, merge, deploy, production mutation, credential action, safety-gate
weakening, or public exposure.

Config entries are evaluated before built-in defaults. A configured profile
with the same name as a built-in profile replaces that built-in profile, so
consumer repos can tune local policy without carrying a second hidden rule.

## Model

A review profile can match on task facts:

- kind;
- risk level;
- tags;
- authoring role/domain;
- owning domain;
- source or target paths;
- parent/group relationship.

The selected profile can then:

- add required review domains;
- mark domains as waived for a micro-slice;
- defer domains to an epic, release, launch, or production-readiness review;
- inherit domains from an approved parent/group packet;
- block inheritance when the child expands authority, mutates environments,
  handles credentials, weakens safety gates, or authorizes live, deploy, or
  public exposure action;
- mark the slice as a safe iteration zone;
- explain the expected defect class or risk-control value for extra reviewers.
- name a process hypothesis and outcome metrics for pilot review.

Fairway reports each domain with a policy status:

```text
required
inherited
waived
deferred
```

Only `required` domains block `merge-ready` when missing. Inherited, waived, and
deferred domains remain visible in review-wait/task detail output so operators
can see why a full matrix was not requested for that slice.

If a profile runs in `advisory` mode, missing required domains are reported as
warnings rather than blockers. This lets teams measure process value before
turning the policy into a blocking gate.

## Built-In Defaults

Fairway ships four default profiles for the common risk boundaries. They are
active when no configured profile has the same name:

| Profile | Matches | Default effect |
|---|---|---|
| `reversible` | `risk_level = "reversible"` or tags `reversible`, `risk:reversible`, `review:reversible` | Advisory safe-iteration profile. Normal architecture, backend, governance, ops, and security review domains are waived for the slice and remain visible as waived policy rows. Evidence and self-check are expected. |
| `irreversible` | `risk_level = "irreversible"` or tags `irreversible`, `risk:irreversible`, `boundary:irreversible`, `credentials`, `security`, or `prod` | Blocking architecture, governance, ops, and security review. Inheritance is disabled for high/irreversible risk and authority-expanding tags. |
| `live-boundary` | `risk_level = "live-boundary"`, kind `live-window`, or tags `live`, `live-window`, `boundary:live`, `environment:production` | Blocking backend, governance, ops, and security review before live execution. |
| `release-boundary` | `risk_level = "release-boundary"`, kind `release-risk`, release/deploy/public-exposure tags, or release paths such as `docs/release`, `.goreleaser`, `dist/`, or `scripts/release` | Blocking governance, ops, and security review before public distribution or release changes. |

The defaults encode the small-team operating principle: reversible work should
move quickly with evidence, while irreversible, live, and release boundaries
keep explicit controls. They do not weaken explicit gates configured elsewhere.

## Safe Iteration Zones

A safe iteration zone is an approved non-live or disposable boundary where a
consumer repo can iterate on setup, readback, harness, classifier, fixture, or
provider-shape defects without re-running full review matrices for every child
slice.

The profile must name:

- `safe_iteration_defect_class`, such as `harness`, `classifier`,
  `provider_shape`, `readback`, or `setup`;
- `safe_iteration_control`, such as `non-live disposable boundary`,
  `offline fixture`, or `approved preflight surface`;
- `extra_reviewer_rationale`, explaining why extra reviewers do or do not
  improve quality or cycle time.

Boundary exit requests should use a stricter profile. Examples include live
window execution, deploy, production-readiness, compliance, enforcement,
credential handling, safety-gate changes, public exposure, and authority
expansion.

## Coordinator Behavior

Coordinator plan may recommend grouped review for related ready tasks that
match a safe iteration profile. The recommendation should say that Fairway is
continuing safe-boundary iteration and reserving the full matrix for boundary
exit. This reduces provider token burn and review churn without hiding review
policy from the control surface.

`fairway review-policy report` compares review/gate overhead with outcomes
recorded in Fairway facts. The first slice reports per-profile task count,
required/approved/missing/inherited/waived/deferred review domains, evidence
failures as defect-caught signals, partial evidence as rework-reduction signals,
blocked evidence as avoided-unsafe-action signals, blocked task count, and
completed task count. Advisory profiles with overhead and no useful outcomes
should be removed or narrowed instead of promoted to blocking defaults.

The same report also detects looping review/retry patterns from existing
evidence and review facts. A loop is advisory-detected when repeated meaningful
failures happen after near-ready claims, when the same layer keeps failing, or
when approvals do not lead to end-to-end flow progress. The report should
recommend a causal reset that names the failure chain, real unknowns, required
proof before another retry, and a lighter safe-boundary review plan. This is a
step-back signal: reviewers should challenge another full-matrix retry when the
recorded facts show that the causal model is still unknown.

## Trust Boundary

Risk-scaled review is advisory policy and merge gating. It is not self-review
and not hidden approval. A child can inherit a domain only from an actual
approved parent/group review recorded in Fairway, and only when no
no-inheritance trigger applies.
