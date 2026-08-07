# Dashboard Overview

The single-project dashboard opens with a product Overview for engineers,
reviewers, leaders, and evaluators who do not already know Fairway's operating
vocabulary. The Overview is an orientation and evidence surface, not an
operator control room or a marketing landing page.

## User Journey

The page follows four questions in order:

1. **Understand:** What problem does Fairway solve and what does it preserve?
2. **Inspect:** Can I follow one real work item from intent through outcome?
3. **Trust:** Which claims are supported by current project records, and which
   authority remains elsewhere?
4. **Adopt:** Where do I go next for active work, lifecycle evidence, delivery
   outcomes, control analysis, or diagnostics?

The operational views retain narrower responsibilities:

| View | Responsibility |
|---|---|
| Overview | Explain the product using current project facts and cited records. |
| Wall | Show live provider lanes, handoffs, gates, and activity. |
| Board | Navigate and manage bounded work items. |
| Quality | Compare lifecycle evidence states across work items. |
| Reports | Explain what changed during a delivery window. |
| Controls | Measure whether recorded controls discriminate outcomes. |
| Diagnostics | Inspect coordination and runtime machinery. |

## Information Architecture

```text
Product promise                       Current project proof
  Intent -> evidence -> judgment        tracked / evidence / review / done
          -> promotion -> outcome                       |
                       |                                |
                       +--- Cited work-item trace ------+
                                      |
                         Authority and system boundaries
                                      |
                    Work / Quality / Reports / Controls
```

The first viewport establishes the product promise and shows the Quality Record
as a connected lifecycle. It must leave the current-project proof visible
without requiring the user to understand task statuses or provider sessions.

The proof section uses only current Fairway records:

- total bounded work items;
- work items with recorded evidence;
- work items with recorded review;
- completed work items; and
- one recent, evidence-rich task projected through the canonical nine-stage
  Quality Record.

These are coverage facts, not a quality score or a causal claim.

## Authority Boundary

The Overview must make system ownership explicit:

- Fairway owns the accountable work record and readiness projection;
- Git owns source history;
- CI/CD and deployment systems own execution results;
- reviewers and accountable operators retain judgment, promotion, and risk
  authority.

The dashboard remains read-only. It cannot approve, waive, merge, deploy,
release, accept risk, or turn missing information into generated narrative.

## Routing

In single-project mode:

- `/` renders Overview;
- `/wall` renders the operational Wall; and
- the brand links to Overview.

Multi-project mode keeps `/` as the project coordination wall because its
primary job is aggregation. It does not claim a cross-project Quality Record
authority.

## Browser Acceptance

The implementation is accepted when a first-time user can identify, from the
rendered page alone:

- Fairway's product promise;
- the lifecycle being preserved;
- current evidence coverage and its limitations;
- one cited real work item;
- the external authority boundary; and
- the correct next view for work, evidence, delivery history, or control
  analysis.

Desktop validation must confirm that the first viewport is legible at 1440 by
900, navigation does not overlap, lifecycle connections are unambiguous, and
all drill-down links resolve. A compact-width check must confirm that content
wraps without page-level horizontal overflow; wide evidence structures may
scroll only inside their own bounded container.
