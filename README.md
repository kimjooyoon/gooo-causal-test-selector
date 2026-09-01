# gooo-causal-test-selector

`gooo-causal-test-selector` is an independent, fail-closed selector for
self-improvement candidates. It selects tests from the causal closure of
changed semantic IDs in a `.gooo` semantic/provenance graph, then compares the
selected execution with the full-suite oracle from the same CI run.

The authority chain is intentionally short:

```text
.gooo metadata → semantic IR → causal closure → selected execution
                                      ↘ full-suite oracle
```

The `.gooo` file is the authority for semantic nodes, dependency/provenance
edges, test witnesses, policy, and the fixed six-case denominator. Go only
parses, lowers, executes the declared deterministic runner, and verifies the
evidence. A path filter, cache hit, or generated artifact cannot close a
semantic claim.

## Fixed v1 contract

The six fixed cases are:

1. `leaf-change` — a leaf semantic change closes with its causal witnesses.
2. `shared-dependency` — a shared dependency closes with every descendant
   witness.
3. `comment-only` — no changed semantic ID selects zero tests and closes only
   after the full-suite oracle also observes no effect.
4. `missing-edge` — an incomplete graph becomes `UNKNOWN` and falls back to
   the full suite.
5. `stale-edge` — stale provenance is `REFUTED` and falls back to the full
   suite.
6. `replay` — the selected result is replayed and must be byte-equivalent in
   semantic result, terminal reason, and effect trace.

Resolution precedence is fixed: `REFUTED > UNKNOWN > CLOSED`. Every UNKNOWN
claim preserves `stage`, `step`, `reason`, `unknown_class`, `next_operation`,
and `blocked_by`. Missing or stale graph evidence never earns a speed claim.

Each case records an exact before/after pair with identical
`runner_digest`, `run_id`, and `scenario_digest`. The report includes exact
integer values for Go/Gooo inventory, physical lines, descendant directories,
regular files, generated files/bytes, wall time, peak RSS, and test totals,
selected, executed, reused, failed, and unknown. The root `README.md` is
excluded from inventory by contract.

## Commands

Compile the authoritative source to a caller-owned IR file:

```sh
go run ./cmd/gooo-causal-test-selector compile \
  --root . \
  --source examples/causal-test-selector/main.gooo \
  --contract contracts/causal-test-selector-denominator-v1.json \
  --output-ir /tmp/gooo-causal-test-selector-ir.json
```

Run the fixed conformance suite. The output directory must be empty and
outside the input repository:

```sh
go run ./cmd/gooo-causal-test-selector conformance \
  --root . \
  --source examples/causal-test-selector/main.gooo \
  --contract contracts/causal-test-selector-denominator-v1.json \
  --output-dir /tmp/gooo-causal-test-selector-conformance \
  --run-id ci-run-identity
```

The output contains `report.json`, `semantic-ir.json`, `metrics.json`,
`execution-receipt.json`, and `ci-summary.md`. Generated output is caller
owned; the input repository is never patched. `repository_writes` is fixed at
zero and CI checks the Git worktree before and after execution.

## CI and release boundary

GitHub Actions is the validation authority. It runs Go 1.27 build, test, vet,
format, compile, and conformance checks. Local test/build/vet/conformance is
not required for the product workflow. The release workflow refuses to
replace an existing release, requires an annotated tag, and publishes an
immutable evidence archive with its SHA-256 digest.

## Repository identity

The public repository is intended to be
`github.com/kimjooyoon/gooo-causal-test-selector`.
