#!/usr/bin/env bash
set -euo pipefail

if test "$#" -ne 5; then
  echo "usage: conformance.sh ROOT SOURCE CONTRACT OUTPUT_DIR RUN_ID" >&2
  exit 64
fi

root=$1
source_path=$2
contract_path=$3
output_dir=$4
run_id=$5

go run ./cmd/gooo-causal-test-selector conformance \
  --root "$root" \
  --source "$source_path" \
  --contract "$contract_path" \
  --output-dir "$output_dir" \
  --run-id "$run_id"

jq -e '
  .fixed_conformance_denominator == 6 and
  .repository_writes == 0 and
  .summary.cases_total == 6 and
  .summary.observed_false_negatives == 0 and
  .summary.replay_comparisons == 1 and
  .summary.replay_mismatches == 0 and
  (([.cases[] | select((.decision == "CLOSED") and ((.pair.false_negative // 0) != 0))] | length) == 0)
' "$output_dir/report.json" >/dev/null
