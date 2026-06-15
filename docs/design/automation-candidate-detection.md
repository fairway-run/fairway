# Automation Candidate Detection

Fairway should help operators notice repeated deterministic coordination work.
The default rule is:

```text
first time manual
second time capture the checklist or command
third time automate or create an automation task
```

The first implementation is read-only:

```bash
fairway automation candidates --since 168h [--threshold 3] [--format text|json]
```

The report groups repeated signals from existing Fairway rows:

- evidence command text, with task ids normalized for repeated command shapes;
- evidence artifact/result classes such as merge-ready checks, review-wait
  checks, preflight packet rendering, redaction, commit-boundary handling, and
  CI/deploy handbacks;
- notification domain/state loops such as repeated `thread_steered` wakes.

Each candidate includes frequency, recent task ids, representative commands or
artifacts, likely owner, estimated coordination cost, suggested surface, and a
recommended action. Suggested surfaces include script, Fairway CLI, dashboard
panel, watcher, and packet template.

The command does not auto-create tasks, approve reviews, mutate workflow,
merge, deploy, or authorize live work. A future configured policy may add an
explicit apply path, but the default remains advisory.
