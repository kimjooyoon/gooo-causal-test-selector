package selector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFixedCasesAndExactSafetyProof(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(filepath.Join(root, "examples", "causal-test-selector", "main.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := LoadContract(filepath.Join(root, "contracts", "causal-test-selector-denominator-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	report, err := Execute(source, contract, root, out, "test-run-fixed")
	if err != nil {
		t.Fatal(err)
	}
	if report.Summary.CasesTotal != 6 || report.Summary.Closed != 4 || report.Summary.Unknown != 1 || report.Summary.Refuted != 1 {
		t.Fatalf("unexpected fixed-case summary: %#v", report.Summary)
	}
	if report.Summary.ObservedFalseNegatives != 0 {
		t.Fatalf("false-negative proof is not zero: %d", report.Summary.ObservedFalseNegatives)
	}
	if report.Summary.ReplayComparisons != 1 || report.Summary.ReplayMismatches != 0 {
		t.Fatalf("replay proof did not close: %#v", report.Summary)
	}
	if report.Summary.FullSuiteTests != 18 || report.Summary.SelectedTests != 13 || report.Summary.SelectedExecuted != 13 || report.Summary.SelectedReused != 0 {
		t.Fatalf("unexpected exact test counts: %#v", report.Summary)
	}
	if report.Improvement.State != DecisionUnknown || !report.Improvement.HasCompleteUnknownTuple() {
		t.Fatalf("improvement must remain UNKNOWN when fixed set is not all CLOSED: %#v", report.Improvement)
	}
	if report.Cases[0].Decision != DecisionClosed || len(report.Cases[0].CandidateTests) != 2 || len(report.Cases[0].SelectedTests) != 2 {
		t.Fatalf("leaf closure did not select its causal witnesses: %#v", report.Cases[0])
	}
	if report.Cases[2].Decision != DecisionClosed || len(report.Cases[2].CandidateTests) != 0 || len(report.Cases[2].SelectedTests) != 0 {
		t.Fatalf("comment-only case did not produce zero selection: %#v", report.Cases[2])
	}
	if report.Cases[3].Decision != DecisionUnknown || !report.Cases[3].Fallback || !report.Cases[3].Claim.HasCompleteUnknownTuple() {
		t.Fatalf("missing-edge case did not fail closed: %#v", report.Cases[3])
	}
	if report.Cases[4].Decision != DecisionRefuted || !report.Cases[4].Fallback {
		t.Fatalf("stale-edge case did not refute and fall back: %#v", report.Cases[4])
	}
	if report.Cases[5].Decision != DecisionClosed || !report.Cases[5].ReplayEqual {
		t.Fatalf("replay case did not close: %#v", report.Cases[5])
	}
	if _, err := os.Stat(filepath.Join(out, "report.json")); err != nil {
		t.Fatal(err)
	}
}

func TestOutputMustBeOutsideInputRepository(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(filepath.Join(root, "examples", "causal-test-selector", "main.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := LoadContract(filepath.Join(root, "contracts", "causal-test-selector-denominator-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Execute(source, contract, root, filepath.Join(root, "forbidden-output"), "test-run-boundary"); err == nil {
		t.Fatal("expected repository boundary rejection")
	}
}

func TestUnknownTupleRequiresAllSixFields(t *testing.T) {
	claim := unknownClaim("SELECTION", "VERIFY_PROVENANCE_EDGE", "DEPENDENCY_EDGE_MISSING", "INCOMPLETE_GRAPH", "REPAIR_GOOO_EDGE", []string{"edge-id"})
	if !claim.HasCompleteUnknownTuple() {
		t.Fatalf("complete UNKNOWN tuple rejected: %#v", claim)
	}
	claim.BlockedBy = nil
	if claim.HasCompleteUnknownTuple() {
		t.Fatal("nil blocked_by must not close UNKNOWN")
	}
}

func TestIRCannotRelaxPolicy(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	source, err := LoadSource(filepath.Join(root, "examples", "causal-test-selector", "main.gooo"))
	if err != nil {
		t.Fatal(err)
	}
	contract, err := LoadContract(filepath.Join(root, "contracts", "causal-test-selector-denominator-v1.json"))
	if err != nil {
		t.Fatal(err)
	}
	ir, err := Lower(source, contract)
	if err != nil {
		t.Fatal(err)
	}
	ir.Policy.CacheClose = true
	ir.IRDigest, err = unsignedIRDigest(ir)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateIR(ir); err == nil {
		t.Fatal("relaxed cache policy must not validate")
	}
}
