# Concepts

These terms are used across the design docs and should stay stable once code
starts.

## Role

A role is a configured responsibility lane, such as `backend`, `ui`, `ops`,
`arch`, or `governance`. Tasks are assigned to roles; review routing and
worktree setup are role-aware.

## Lane

A lane is one active execution slot for a role. In the common case there is one
lane per role, backed by one long-lived worktree and branch. When multiple lanes
share a role, `lane` is an optional identifier that distinguishes concurrent
sessions or watcher/sidecar work under that role.

Examples:

- `backend` role, default lane: normal backend implementation work.
- `ops` role, `watch` lane: an ops-owned deploy watcher.
- `ui` role, `review` lane: a UI review session on another role's branch.

Lane is not a new permission model and not a replacement for role. It is a
display and coordination label used by sessions, packets, watcher work, and the
dashboard.

## Actor

An actor is the identity recorded on state transitions and audit-like rows. Use
the active `agent_sessions.id` when a command can infer one; otherwise use
`<os_user>@<host>`. Actor is never NULL.

## Task ID

Task IDs are user-supplied and stable. Fairway does not auto-generate IDs in
v1. The default validation regex is `^[A-Z]+-[0-9]+$`, configurable later only
if a project already has a different durable convention.
