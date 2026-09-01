# Fixed conformance matrix

| Ordinal | Case | Graph condition | Expected | Fallback |
|---:|---|---|---|---|
| 1 | leaf-change | `cart.total` closure | CLOSED | no |
| 2 | shared-dependency | `tax.rate` reaches cart and invoice | CLOSED | no |
| 3 | comment-only | no changed semantic IDs | CLOSED | no |
| 4 | missing-edge | declared dependency edge unavailable | UNKNOWN | full suite |
| 5 | stale-edge | declared provenance edge stale | REFUTED | full suite |
| 6 | replay | exact selected replay | CLOSED | no |

The fixed expected summary is four CLOSED cases, one UNKNOWN case, and one
REFUTED case. The exact safety count is `observed_false_negatives=0`. The
runner executes three witnesses in the full suite for each of six cases:
`tests_total=18`. The selected/fallback executions are `13`, with
`tests_reused=0`; CI also records the measured wall time and peak RSS for each
before/after pair.
