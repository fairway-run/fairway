# Plane Tracker Adapter Evaluation

This directory contains provider-specific evaluation fixtures for Plane. These
files are examples and dry-run inputs only; Fairway core should stay
provider-neutral.

Use `docs/operations/plane-local-evaluation.md` to run a local Plane instance,
then create a `fairway-eval` workspace/project using
`evaluation-workspace.yaml` as the reference data set.

Do not commit:

- Plane `.env` files,
- admin credentials,
- API tokens,
- browser cookies,
- database dumps,
- generated compose files downloaded by Plane's installer.

Future adapter work should consume this fixture to prove field mapping before
adding write operations.

Dry-run the current spike:

```bash
export PLANE_BASE_URL=http://localhost:8088
export PLANE_WORKSPACE=fairway-eval
export PLANE_PROJECT=FWPLANE

fairway tracker plane import --fixture examples/tracker-adapters/plane/evaluation-workspace.yaml
fairway tracker plane export --task-id FW-122
fairway tracker plane comment --task-id FW-122 --external-id FWPLANE-122
```

These commands do not call Plane and do not mutate Fairway execution state.
