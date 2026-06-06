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
