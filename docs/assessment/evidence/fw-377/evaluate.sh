#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
tmp_matches="$(mktemp)"
trap 'rm -f "$tmp_matches"' EXIT

secret_pattern='begin (rsa|ec|openssh|private) key|access[_-]?token[[:space:]]*[:=][[:space:]]*[^[:space:]"]+|refresh[_-]?token[[:space:]]*[:=][[:space:]]*[^[:space:]"]+|password[[:space:]]*[:=][[:space:]]*[^[:space:]"]+|authorization:[[:space:]]*bearer[[:space:]]+[a-z0-9._-]+'
if rg -l -i "$secret_pattern" "$ROOT_DIR"/*-cold-start.json >"$tmp_matches"; then
  printf 'retained packet secret scan failed; flagged files=%s\n' "$(wc -l <"$tmp_matches" | tr -d ' ')" >&2
  exit 1
fi

for spec in \
  'repair|architecture/repair-quarantine-recovery.md' \
  'domains|architecture/failure-upgrade-domains.md' \
  'identity|architecture/logical-workload-identity-resolution.md'; do
  name="${spec%%|*}"
  expected="${spec#*|}"
  packet="$ROOT_DIR/$name-cold-start.json"
  jq -e --arg expected "$expected" '
    .bounded == true and .read_only == true and
    .packet.memory_disposition == "active" and
    .packet.track_task_status == "todo" and
    .packet.checkpoint_chronology == "newest_first_historical" and
    ((.packet.blockers // []) | length) == ((.packet.blockers // []) | unique | length) and
    ((.packet.next_actions // []) | length) == ((.packet.next_actions // []) | unique | length) and
    ([.packet.next_actions[]? | startswith("inspect ")] | any | not) and
    ([.packet.checkpoints[]? | contains("historical=true")] | all) and
    .git.commit == "ec2b93f67" and .git.dirty == false and
    .knowledge.repository_revision == "ec2b93f67" and
    .knowledge.bytes <= .knowledge.budget_bytes and
    .knowledge.pages[0].path == $expected and
    ([.knowledge.pages[].source_freshness] |
      all(. == "current_content_at_recorded_revision" or . == "current_repository_revision")) and
    ([.knowledge.pages[].stale] | any | not)
  ' "$packet" >/dev/null
done

jq -e '.checks | to_entries | all(.value == true)' "$ROOT_DIR/evaluation-summary.json" >/dev/null
printf 'FW-377 retained packet evaluation: pass\n'
