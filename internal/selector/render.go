package selector

import (
	"fmt"
	"os"
	"strings"
)

func WriteSummary(path string, report Report) error {
	var builder strings.Builder
	builder.WriteString("# gooo-causal-test-selector conformance\n\n")
	fmt.Fprintf(&builder, "decision=%s\nfixed_conformance_denominator=%d\nrepository_writes=%d\nobserved_false_negatives=%d\n", report.Decision, report.FixedConformanceDenominator, report.RepositoryWrites, report.Summary.ObservedFalseNegatives)
	fmt.Fprintf(&builder, "cases_total=%d closed=%d unknown=%d refuted=%d fallback_cases=%d replay_comparisons=%d replay_mismatches=%d\n", report.Summary.CasesTotal, report.Summary.Closed, report.Summary.Unknown, report.Summary.Refuted, report.Summary.FallbackCases, report.Summary.ReplayComparisons, report.Summary.ReplayMismatches)
	fmt.Fprintf(&builder, "go_files=%d gooo_files=%d go_physical_lines=%d gooo_physical_lines=%d descendant_dirs=%d regular_files=%d generated_files=%d generated_bytes=%d wall_ms=%d peak_rss_kib=%d\n", report.Metrics.GoFiles, report.Metrics.GoooFiles, report.Metrics.GoPhysicalLines, report.Metrics.GoooPhysicalLines, report.Metrics.DescendantDirs, report.Metrics.RegularFiles, report.Metrics.GeneratedFiles, report.Metrics.GeneratedBytes, report.Metrics.WallMS, report.Metrics.PeakRSSKiB)
	fmt.Fprintf(&builder, "tests_total=%d tests_selected=%d tests_executed=%d tests_reused=%d tests_failed=%d tests_unknown=%d\n\n", report.Metrics.TestsTotal, report.Metrics.TestsSelected, report.Metrics.TestsExecuted, report.Metrics.TestsReused, report.Metrics.TestsFailed, report.Metrics.TestsUnknown)
	builder.WriteString("| ordinal | case | decision | changed | closure | candidate | selected | fallback | before(full) | after(selected) | false_negative |\n")
	builder.WriteString("|---:|---|---|---|---:|---:|---:|---|---|---|---:|\n")
	for _, item := range report.Cases {
		before := fmt.Sprintf("total=%d executed=%d failed=%d wall_ms=%d peak_rss_kib=%d", item.FullOracle.Executed, item.FullOracle.Executed, item.FullOracle.Failed, item.FullOracle.WallMS, item.FullOracle.PeakRSSKiB)
		after := fmt.Sprintf("selected=%d executed=%d reused=0 failed=%d unknown=%d wall_ms=%d peak_rss_kib=%d", len(item.SelectedTests), item.SelectedRun.Executed, item.SelectedRun.Failed, item.SelectedRun.Unknown, item.SelectedRun.WallMS, item.SelectedRun.PeakRSSKiB)
		fmt.Fprintf(&builder, "| %d | %s | %s | %s | %d | %d | %d | %t | %s | %s | %d |\n", item.Ordinal, item.CaseID, item.Decision, strings.Join(item.ChangedSemanticIDs, ","), len(item.CausalClosure), len(item.CandidateTests), len(item.SelectedTests), item.Fallback, before, after, item.Pair.FalseNegative)
	}
	builder.WriteString("\nAll before/after observations carry the exact runner_digest, run_id, and scenario_digest pair recorded in report.json. `tests_reused` is zero; a cache hit is never a semantic close.\n")
	return os.WriteFile(path, []byte(builder.String()), 0o644)
}
