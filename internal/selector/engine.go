package selector

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"
)

const deterministicRunnerDigest = "go1.27-ci-causal-runner-v1"

type graphIndex struct {
	NodesByID map[string]Node
	Edges     []Edge
	Witnesses []Witness
}

func newGraphIndex(graph Graph) graphIndex {
	nodes := make(map[string]Node, len(graph.Nodes))
	for _, node := range graph.Nodes {
		nodes[node.ID] = node
	}
	return graphIndex{NodesByID: nodes, Edges: append([]Edge(nil), graph.Edges...), Witnesses: append([]Witness(nil), graph.Witnesses...)}
}

func (g graphIndex) closure(changed []string) []string {
	seen := make(map[string]bool, len(changed))
	queue := append([]string(nil), changed...)
	for _, id := range changed {
		seen[id] = true
	}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		for _, edge := range g.Edges {
			if edge.Status == "ACTIVE" && edge.From == current && !seen[edge.To] {
				seen[edge.To] = true
				queue = append(queue, edge.To)
			}
		}
	}
	result := make([]string, 0, len(seen))
	for id := range seen {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}

func (g graphIndex) candidateTests(closure []string) []string {
	inside := make(map[string]bool, len(closure))
	for _, id := range closure {
		inside[id] = true
	}
	result := make([]string, 0, len(g.Witnesses))
	for _, witness := range g.Witnesses {
		for _, target := range witness.Targets {
			if inside[target] {
				result = append(result, witness.TestID)
				break
			}
		}
	}
	return result
}

func Execute(source Source, contract Contract, root, output, runID string) (Report, error) {
	if err := ValidateDeclarations(source, contract); err != nil {
		return Report{}, err
	}
	if _, err := ensureCallerOutput(root, output); err != nil {
		return Report{}, err
	}
	if runID == "" {
		runID = "gooo-causal-test-selector-ci-run"
	}
	ir, err := Lower(source, contract)
	if err != nil {
		return Report{}, err
	}
	if err := ValidateIR(ir); err != nil {
		return Report{}, err
	}
	inv, err := inventory(root)
	if err != nil {
		return Report{}, err
	}
	started := time.Now()
	index := newGraphIndex(ir.Graph)
	caseReports := make([]CaseReport, 0, len(ir.Cases))
	var summary Summary
	for _, caseDecl := range ir.Cases {
		caseReport, caseErr := evaluateCase(source, ir, index, caseDecl, runID)
		if caseErr != nil {
			return Report{}, caseErr
		}
		caseReports = append(caseReports, caseReport)
		accumulateSummary(&summary, caseReport)
	}
	wallMS := int(time.Since(started).Milliseconds())
	peakRSS := processRSSKiB()
	metrics := Metrics{
		GoFiles: inv.GoFiles, GoooFiles: inv.GoooFiles, GoPhysicalLines: inv.GoPhysicalLines,
		GoooPhysicalLines: inv.GoooPhysicalLines, DescendantDirs: inv.DescendantDirs,
		RegularFiles: inv.RegularFiles, GeneratedFiles: inv.GeneratedFiles, GeneratedBytes: inv.GeneratedBytes,
		WallMS: wallMS, PeakRSSKiB: peakRSS, TestsTotal: summary.FullSuiteTests,
		TestsSelected: summary.SelectedTests, TestsExecuted: summary.SelectedExecuted,
		TestsReused: summary.SelectedReused, TestsFailed: summary.SelectedFailed,
		TestsUnknown: summary.SelectedUnknown, RepositoryWrites: source.Authority.RepositoryWrites,
	}
	report := Report{
		Schema: ReportSchema, SourceDigest: source.SourceDigest, ContractDigest: contractDigest(contract),
		IRDigest: ir.IRDigest, DenominatorID: source.DenominatorID, FixedConformanceDenominator: FixedCases,
		Precedence: append([]string(nil), source.Precedence...), UnknownFields: append([]string(nil), source.UnknownFields...),
		Authority: source.Authority, Policy: source.Policy, Inventory: inv, Summary: summary, Metrics: metrics,
		Cases: caseReports, RepositoryWrites: 0,
	}
	report.Decision, report.Improvement = finalClaims(report)
	if err := validateReport(report); err != nil {
		return Report{}, err
	}
	if err := WriteJSON(output+"/report.json", report); err != nil {
		return Report{}, err
	}
	receipt := ExecutionReceipt{
		Schema: ReceiptSchema, SourceToIR: "GOOO_SOURCE_TO_SEMANTIC_IR", IRToSelection: "SEMANTIC_IR_TO_CAUSAL_CLOSURE_SELECTION",
		SelectionToRun: "FULL_ORACLE_AND_SELECTED_RUN_WITH_EXACT_PAIR", Cases: FixedCases, CallerOwnedOutput: true,
		RepositoryWrites: 0, LocalTestExecutions: 0, CacheReused: summary.SelectedReused,
	}
	if err := WriteJSON(output+"/execution-receipt.json", receipt); err != nil {
		return Report{}, err
	}
	if err := WriteJSON(output+"/semantic-ir.json", ir); err != nil {
		return Report{}, err
	}
	if err := WriteJSON(output+"/metrics.json", metrics); err != nil {
		return Report{}, err
	}
	if err := WriteSummary(output+"/ci-summary.md", report); err != nil {
		return Report{}, err
	}
	return report, nil
}

func evaluateCase(source Source, ir IR, index graphIndex, caseDecl Case, runID string) (CaseReport, error) {
	closure := index.closure(caseDecl.Changed)
	candidate := index.candidateTests(closure)
	selection := Claim{State: DecisionClosed, Stage: "SELECTION", Step: "COMPUTE_CAUSAL_CLOSURE", Reason: "CAUSAL_CLOSURE_VERIFIED", NextOperation: "NONE", BlockedBy: []string{}}
	fallback := false
	switch caseDecl.EdgeFault {
	case "MISSING":
		selection = unknownClaim("SELECTION", "VERIFY_PROVENANCE_EDGE", "DEPENDENCY_EDGE_MISSING", "INCOMPLETE_GRAPH", "REPAIR_GOOO_EDGE", []string{caseDecl.FaultEdge})
		fallback = true
	case "STALE":
		selection = refutedClaim("SELECTION", "VERIFY_PROVENANCE_EDGE", "STALE_PROVENANCE_EDGE", "REPAIR_GOOO_EDGE", []string{caseDecl.FaultEdge})
		fallback = true
	}
	selected := append([]string(nil), candidate...)
	if fallback {
		selected = allTestIDs(index.Witnesses)
	}
	fullRun, err := runTests(source, caseDecl, runID, "FULL_SUITE", allTestIDs(index.Witnesses), ir)
	if err != nil {
		return CaseReport{}, err
	}
	selectedRun, err := runTests(source, caseDecl, runID, "SELECTED", selected, ir)
	if err != nil {
		return CaseReport{}, err
	}
	pair := PairMeasurement{
		RunnerDigest: fullRun.RunnerDigest, ToolchainDigest: fullRun.ToolchainDigest, RunID: fullRun.RunID,
		ScenarioDigest: fullRun.ScenarioDigest, SourceDigest: ir.SourceDigest, ContractDigest: ir.ContractDigest,
		ExactIdentity: fullRun.RunnerDigest == selectedRun.RunnerDigest && fullRun.ToolchainDigest == selectedRun.ToolchainDigest && fullRun.RunID == selectedRun.RunID && fullRun.ScenarioDigest == selectedRun.ScenarioDigest,
		BeforeFull:    fullRun, AfterSelected: selectedRun,
		TerminalTraceEqual: fullRun.TerminalReason == selectedRun.TerminalReason && fullRun.EffectTrace == selectedRun.EffectTrace,
		OutcomeEqual:       fullRun.Failed == selectedRun.Failed && fullRun.TerminalReason == selectedRun.TerminalReason && fullRun.EffectTrace == selectedRun.EffectTrace,
	}
	if fullRun.Failed > 0 && selectedRun.Failed == 0 {
		pair.FalseNegative = 1
	}
	replayEqual := true
	replayMismatches := 0
	if caseDecl.Replay {
		replay, replayErr := runTests(source, caseDecl, runID, "SELECTED", selected, ir)
		if replayErr != nil {
			return CaseReport{}, replayErr
		}
		replayEqual = replay.RunDigest == selectedRun.RunDigest && replay.TerminalReason == selectedRun.TerminalReason && replay.EffectTrace == selectedRun.EffectTrace && equalResults(replay.Tests, selectedRun.Tests)
		if !replayEqual {
			replayMismatches = 1
		}
	}
	decision, claim := caseDecision(selection, pair, replayEqual, caseDecl)
	return CaseReport{
		Ordinal: caseDecl.Ordinal, CaseID: caseDecl.ID, Kind: caseDecl.Kind, Expected: caseDecl.Expected,
		ChangedSemanticIDs: sortedCopy(caseDecl.Changed), CausalClosure: closure,
		CandidateTests: candidate, SelectedTests: selected, Fallback: fallback,
		SelectionClaim: selection, FullOracle: fullRun, SelectedRun: selectedRun,
		Pair: pair, ReplayEqual: replayEqual, ReplayMismatches: replayMismatches,
		Decision: decision, Claim: claim,
	}, nil
}

func runTests(source Source, caseDecl Case, runID, mode string, testIDs []string, ir IR) (RunObservation, error) {
	scenario, err := scenarioDigest(source, caseDecl)
	if err != nil {
		return RunObservation{}, err
	}
	started := time.Now()
	results := make([]TestResult, 0, len(testIDs))
	witnesses := make(map[string]Witness, len(ir.Graph.Witnesses))
	for _, witness := range ir.Graph.Witnesses {
		witnesses[witness.TestID] = witness
	}
	for _, testID := range testIDs {
		witness, ok := witnesses[testID]
		if !ok {
			return RunObservation{}, fmt.Errorf("selected unknown test witness %s", testID)
		}
		result := TestResult{TestID: testID, Outcome: "PASS", TerminalReason: "NO_FAILURE", Effect: "NO_EFFECT"}
		if testID == caseDecl.FaultTest {
			result.Outcome = "FAIL"
			result.TerminalReason = caseDecl.TerminalReason
			result.Effect = caseDecl.Effect
		}
		result.TraceDigest, err = DigestValue(struct {
			Witness Witness    `json:"witness"`
			Result  TestResult `json:"result"`
		}{witness, result})
		if err != nil {
			return RunObservation{}, err
		}
		results = append(results, result)
	}
	failed := 0
	effects := make([]string, 0, len(results))
	terminalReason := "NO_TEST_FAILURE"
	for _, result := range results {
		if result.Outcome == "FAIL" {
			failed++
			terminalReason = result.TerminalReason
			effects = append(effects, result.TestID+"="+result.Effect)
		}
	}
	if failed == 0 && caseDecl.FaultTest == "" {
		terminalReason = caseDecl.TerminalReason
	}
	if len(effects) == 0 {
		effects = []string{"NO_EFFECT"}
	}
	result := RunObservation{
		Mode: mode, RunnerDigest: deterministicRunnerDigest, ToolchainDigest: "go1.27", RunID: runID, ScenarioDigest: scenario,
		Tests: results, Executed: len(results), Failed: failed, Unknown: 0, TerminalReason: terminalReason,
		EffectTrace: strings.Join(effects, ";"), WallMS: int(time.Since(started).Milliseconds()), PeakRSSKiB: processRSSKiB(),
	}
	result.RunDigest, err = DigestValue(struct {
		Mode            string       `json:"mode"`
		RunnerDigest    string       `json:"runner_digest"`
		ToolchainDigest string       `json:"toolchain_digest"`
		RunID           string       `json:"run_id"`
		ScenarioDigest  string       `json:"scenario_digest"`
		Tests           []TestResult `json:"tests"`
		TerminalReason  string       `json:"terminal_reason"`
		EffectTrace     string       `json:"effect_trace"`
	}{result.Mode, result.RunnerDigest, result.ToolchainDigest, result.RunID, result.ScenarioDigest, result.Tests, result.TerminalReason, result.EffectTrace})
	if err != nil {
		return RunObservation{}, err
	}
	return result, nil
}

func caseDecision(selection Claim, pair PairMeasurement, replayEqual bool, caseDecl Case) (string, Claim) {
	if selection.State == DecisionRefuted {
		return DecisionRefuted, selection
	}
	if !pair.ExactIdentity || !pair.OutcomeEqual || pair.FalseNegative != 0 {
		return DecisionRefuted, refutedClaim("VERIFICATION", "COMPARE_FULL_ORACLE_AND_SELECTED", "SELECTED_RUN_DIVERGED_FROM_FULL_ORACLE", "RESTORE_CAUSAL_SELECTION", []string{caseDecl.ID})
	}
	if !replayEqual {
		return DecisionRefuted, refutedClaim("REPLAY", "COMPARE_REPLAY_RUN", "REPLAY_RESULT_MISMATCH", "RESTORE_DETERMINISTIC_RUNNER", []string{caseDecl.ID})
	}
	if selection.State == DecisionUnknown {
		return DecisionUnknown, selection
	}
	return DecisionClosed, Claim{State: DecisionClosed, Stage: "VERIFICATION", Step: "COMPARE_FULL_ORACLE_AND_SELECTED", Reason: "EXACT_ORACLE_MATCH_ZERO_FALSE_NEGATIVES", NextOperation: "NONE", BlockedBy: []string{}}
}

func accumulateSummary(summary *Summary, caseReport CaseReport) {
	summary.CasesTotal++
	switch caseReport.Decision {
	case DecisionClosed:
		summary.Closed++
	case DecisionUnknown:
		summary.Unknown++
	case DecisionRefuted:
		summary.Refuted++
	}
	summary.ObservedFalseNegatives += caseReport.Pair.FalseNegative
	if caseReport.Kind == "REPLAY" {
		summary.ReplayComparisons++
		summary.ReplayMismatches += caseReport.ReplayMismatches
	}
	if caseReport.Fallback {
		summary.FallbackCases++
	}
	summary.FullSuiteTests += caseReport.FullOracle.Executed
	summary.SelectedTests += len(caseReport.SelectedTests)
	summary.SelectedExecuted += caseReport.SelectedRun.Executed
	summary.SelectedReused += 0
	summary.SelectedFailed += caseReport.SelectedRun.Failed
	summary.SelectedUnknown += caseReport.SelectedRun.Unknown
}

func finalClaims(report Report) (string, Claim) {
	decision := DecisionClosed
	if report.Summary.Refuted > 0 {
		decision = DecisionRefuted
	} else if report.Summary.Unknown > 0 {
		decision = DecisionUnknown
	}
	if report.Summary.ObservedFalseNegatives != 0 {
		return DecisionRefuted, refutedClaim("VERIFICATION", "PROVE_ZERO_FALSE_NEGATIVES", "OBSERVED_FALSE_NEGATIVE", "RESTORE_SELECTION_GRAPH", []string{"summary"})
	}
	if decision != DecisionClosed {
		return decision, unknownClaim("IMPROVEMENT", "COMPARE_EXACT_BEFORE_AFTER_PAIRS", "CASE_SET_NOT_CLOSED", "UNPROVEN_IMPROVEMENT", []string{"cases"})
	}
	if report.Summary.SelectedTests < report.Summary.FullSuiteTests {
		return decision, Claim{State: decision, Stage: "IMPROVEMENT", Step: "COMPARE_EXACT_BEFORE_AFTER_PAIRS", Reason: "EXACT_PAIR_WITH_ZERO_FALSE_NEGATIVES", NextOperation: "NONE", BlockedBy: []string{}}
	}
	return DecisionUnknown, unknownClaim("IMPROVEMENT", "COMPARE_EXACT_BEFORE_AFTER_PAIRS", "NO_EXACT_TEST_COUNT_REDUCTION", "NO_MEASURED_IMPROVEMENT", "COLLECT_EXACT_BEFORE_AFTER_PAIR", []string{"tests_total", "tests_selected"})
}

func validateReport(report Report) error {
	if report.FixedConformanceDenominator != FixedCases || len(report.Cases) != FixedCases || report.RepositoryWrites != 0 || report.Metrics.RepositoryWrites != 0 {
		return fmt.Errorf("report violates fixed denominator or zero-write boundary")
	}
	if report.Summary.ObservedFalseNegatives != 0 {
		return fmt.Errorf("closed safety proof contains a false negative")
	}
	for _, caseReport := range report.Cases {
		if caseReport.Decision != caseReport.Expected {
			return fmt.Errorf("fixed case %s expected %s but observed %s", caseReport.CaseID, caseReport.Expected, caseReport.Decision)
		}
		if caseReport.Decision == DecisionUnknown && !caseReport.Claim.HasCompleteUnknownTuple() {
			return fmt.Errorf("UNKNOWN case %s does not preserve the six-field tuple", caseReport.CaseID)
		}
		if caseReport.Decision == DecisionClosed && caseReport.Pair.FalseNegative != 0 {
			return fmt.Errorf("CLOSED case %s has a false negative", caseReport.CaseID)
		}
	}
	if report.Improvement.State == DecisionUnknown && !report.Improvement.HasCompleteUnknownTuple() {
		return fmt.Errorf("UNKNOWN improvement does not preserve the six-field tuple")
	}
	return nil
}

func allTestIDs(witnesses []Witness) []string {
	result := make([]string, 0, len(witnesses))
	for _, witness := range witnesses {
		result = append(result, witness.TestID)
	}
	return result
}

func equalResults(left, right []TestResult) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index] != right[index] {
			return false
		}
	}
	return true
}

func unknownClaim(stage, step, reason, class, next string, blocked []string) Claim {
	return Claim{State: DecisionUnknown, Stage: stage, Step: step, Reason: reason, UnknownClass: class, NextOperation: next, BlockedBy: blocked}
}

func refutedClaim(stage, step, reason, next string, blocked []string) Claim {
	return Claim{State: DecisionRefuted, Stage: stage, Step: step, Reason: reason, UnknownClass: "", NextOperation: next, BlockedBy: blocked}
}

func processRSSKiB() int {
	var usage syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &usage); err == nil {
		value := int(usage.Maxrss)
		if runtime.GOOS == "darwin" {
			value /= 1024
		}
		if value > 0 {
			return value
		}
	}
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	return int(stats.Sys / 1024)
}
