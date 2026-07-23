- Pre-tag production rehearsals now prove the exact pushed source through
  tests, signing, notarization, binary smoke, and release assurance before a
  final version tag exists.
- Annotated tags promote one immutable, run-bound candidate packet instead of
  rebuilding release assets after tagging.
- Promotion verifies exact source, workflow, policy, inventory, checksums, and
  signed assurance without receiving build-signing or Homebrew credentials.
- Release validation now uses bounded behavioral synchronization instead of
  scheduler-sensitive SSE timing assumptions on the macOS signing runner.
- Release assurance records the version of the exact pinned GoReleaser
  executable that built the signed candidate archives.
- Release-assurance checksums are portable and reference only the published
  asset basename, never a runner-local build path.
