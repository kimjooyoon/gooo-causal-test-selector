# Causal selector protocol v1

## Authority

`examples/causal-test-selector/main.gooo` is the semantic authority. It
declares a graph of semantic nodes and directed edges from a dependency to its
dependent. The selector computes the transitive forward closure of every
changed semantic ID. A witness is selected when at least one of its target
nodes is in that closure.

`contracts/causal-test-selector-denominator-v1.json` is a fixed external
denominator check. The parser rejects a source/contract identity mismatch,
case reorder, case count change, non-zero authority, or a policy that permits
cache-only closure.

## Selection and fallback

The declared policy is:

```text
missing edge → UNKNOWN → full-suite fallback
stale edge   → REFUTED → full-suite fallback
cache hit    → never a semantic close
```

The candidate list is recorded before fallback. The selected list is the list
actually executed; for UNKNOWN and REFUTED cases it is the full suite. This
keeps the safety comparison explicit and prevents a fallback from being
reported as a fast selection.

## Oracle comparison

For each fixed case the deterministic runner emits a result for every
executed witness. The full suite is the oracle. The selected run must have the
same runner digest, run identity, scenario digest, failed-test count, terminal
reason, and effect trace. An observed false negative is exactly one when the
full oracle fails and the selected run passes. The closed safety proof requires
the sum to be zero.

Wall time and peak RSS are measured separately for the two runs and retained
as exact integers. They are not converted to a score or percentage.

## State claims

The precedence relation is total and fixed:

```text
REFUTED > UNKNOWN > CLOSED
```

UNKNOWN has a six-field tuple. `blocked_by` is required to be present even
when empty; a missing value is not treated as an empty list. An improvement
claim closes only when every case is CLOSED, the exact identity pair is
present, false negatives are zero, and the selected execution count is lower
than the full-suite count. Otherwise improvement remains UNKNOWN.
