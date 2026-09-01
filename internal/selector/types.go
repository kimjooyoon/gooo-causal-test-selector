package selector

import "sort"

const (
	SourceSchema    = "gooo/causal-test-selector/source/v1"
	GraphSchema     = "gooo/causal-test-selector/graph/v1"
	IRSchema        = "gooo/causal-test-selector/semantic-ir/v1"
	ReportSchema    = "gooo/causal-test-selector/report/v1"
	ReceiptSchema   = "gooo/causal-test-selector/execution-receipt/v1"
	ContractSchema  = "gooo/causal-test-selector/denominator/v1"
	FixedCases      = 6
	DecisionClosed  = "CLOSED"
	DecisionUnknown = "UNKNOWN"
	DecisionRefuted = "REFUTED"
)

var Precedence = []string{DecisionRefuted, DecisionUnknown, DecisionClosed}

type Authority struct {
	RepositoryWrites      int `json:"repository_writes"`
	LocalTestExecutions   int `json:"local_test_executions"`
	ExternalRequiredGates int `json:"external_required_gates"`
}

type Policy struct {
	MissingEdge    string `json:"missing_edge"`
	StaleEdge      string `json:"stale_edge"`
	FullFallback   bool   `json:"full_fallback"`
	CacheClose     bool   `json:"cache_close"`
	SemanticSource string `json:"semantic_source"`
}

type Node struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type Edge struct {
	ID         string `json:"id"`
	From       string `json:"from"`
	To         string `json:"to"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Provenance string `json:"provenance"`
}

type Witness struct {
	ID         string   `json:"id"`
	TestID     string   `json:"test_id"`
	Targets    []string `json:"targets"`
	Command    string   `json:"command"`
	Provenance string   `json:"provenance"`
}

type Case struct {
	Ordinal        int      `json:"ordinal"`
	ID             string   `json:"id"`
	Kind           string   `json:"kind"`
	Changed        []string `json:"changed"`
	Expected       string   `json:"expected"`
	FaultTest      string   `json:"fault_test"`
	TerminalReason string   `json:"terminal_reason"`
	Effect         string   `json:"effect"`
	EdgeFault      string   `json:"edge_fault"`
	FaultEdge      string   `json:"fault_edge"`
	Replay         bool     `json:"replay"`
}

type Source struct {
	Schema        string    `json:"schema"`
	Version       string    `json:"version"`
	DenominatorID string    `json:"denominator_id"`
	CaseCount     int       `json:"case_count"`
	Authority     Authority `json:"authority"`
	Precedence    []string  `json:"precedence"`
	UnknownFields []string  `json:"unknown_fields"`
	Policy        Policy    `json:"policy"`
	Nodes         []Node    `json:"nodes"`
	Edges         []Edge    `json:"edges"`
	Witnesses     []Witness `json:"witnesses"`
	Cases         []Case    `json:"cases"`
	SourceDigest  string    `json:"source_digest"`
}

type ContractCase struct {
	Ordinal  int    `json:"ordinal"`
	ID       string `json:"id"`
	Kind     string `json:"kind"`
	Expected string `json:"expected"`
}

type Contract struct {
	Schema          string         `json:"schema"`
	Version         string         `json:"version"`
	ID              string         `json:"id"`
	Fixed           bool           `json:"fixed"`
	CaseCount       int            `json:"case_count"`
	Cases           []ContractCase `json:"cases"`
	RequiredMetrics []string       `json:"required_metrics"`
}

type Graph struct {
	Schema    string    `json:"schema"`
	Nodes     []Node    `json:"nodes"`
	Edges     []Edge    `json:"edges"`
	Witnesses []Witness `json:"witnesses"`
}

type IR struct {
	Schema         string    `json:"schema"`
	Version        string    `json:"version"`
	SourceDigest   string    `json:"source_digest"`
	ContractDigest string    `json:"contract_digest"`
	DenominatorID  string    `json:"denominator_id"`
	GraphDigest    string    `json:"graph_digest"`
	Graph          Graph     `json:"graph"`
	Cases          []Case    `json:"cases"`
	Policy         Policy    `json:"policy"`
	Authority      Authority `json:"authority"`
	IRDigest       string    `json:"ir_digest,omitempty"`
}

type Claim struct {
	State         string   `json:"state"`
	Stage         string   `json:"stage"`
	Step          string   `json:"step"`
	Reason        string   `json:"reason"`
	UnknownClass  string   `json:"unknown_class"`
	NextOperation string   `json:"next_operation"`
	BlockedBy     []string `json:"blocked_by"`
}

func (c Claim) HasCompleteUnknownTuple() bool {
	return c.State == DecisionUnknown && c.Stage != "" && c.Step != "" && c.Reason != "" &&
		c.UnknownClass != "" && c.NextOperation != "" && c.BlockedBy != nil
}

type TestResult struct {
	TestID         string `json:"test_id"`
	Outcome        string `json:"outcome"`
	TerminalReason string `json:"terminal_reason"`
	Effect         string `json:"effect"`
	TraceDigest    string `json:"trace_digest"`
}

type RunObservation struct {
	Mode            string       `json:"mode"`
	RunnerDigest    string       `json:"runner_digest"`
	ToolchainDigest string       `json:"toolchain_digest"`
	RunID           string       `json:"run_id"`
	ScenarioDigest  string       `json:"scenario_digest"`
	Tests           []TestResult `json:"tests"`
	Executed        int          `json:"executed"`
	Failed          int          `json:"failed"`
	Unknown         int          `json:"unknown"`
	TerminalReason  string       `json:"terminal_reason"`
	EffectTrace     string       `json:"effect_trace"`
	WallMS          int          `json:"wall_ms"`
	PeakRSSKiB      int          `json:"peak_rss_kib"`
	RunDigest       string       `json:"run_digest"`
}

type PairMeasurement struct {
	RunnerDigest       string         `json:"runner_digest"`
	ToolchainDigest    string         `json:"toolchain_digest"`
	RunID              string         `json:"run_id"`
	ScenarioDigest     string         `json:"scenario_digest"`
	SourceDigest       string         `json:"source_digest"`
	ContractDigest     string         `json:"contract_digest"`
	ExactIdentity      bool           `json:"exact_identity"`
	BeforeFull         RunObservation `json:"before_full"`
	AfterSelected      RunObservation `json:"after_selected"`
	TerminalTraceEqual bool           `json:"terminal_trace_equal"`
	OutcomeEqual       bool           `json:"outcome_equal"`
	FalseNegative      int            `json:"false_negative"`
}

type CaseReport struct {
	Ordinal            int             `json:"ordinal"`
	CaseID             string          `json:"case_id"`
	Kind               string          `json:"kind"`
	Expected           string          `json:"expected"`
	ChangedSemanticIDs []string        `json:"changed_semantic_ids"`
	CausalClosure      []string        `json:"causal_closure"`
	CandidateTests     []string        `json:"candidate_tests"`
	SelectedTests      []string        `json:"selected_tests"`
	Fallback           bool            `json:"full_suite_fallback"`
	SelectionClaim     Claim           `json:"selection_claim"`
	FullOracle         RunObservation  `json:"full_suite_oracle"`
	SelectedRun        RunObservation  `json:"selected_run"`
	Pair               PairMeasurement `json:"exact_pair"`
	ReplayEqual        bool            `json:"replay_equal"`
	ReplayMismatches   int             `json:"replay_mismatches"`
	Decision           string          `json:"decision"`
	Claim              Claim           `json:"claim"`
}

type Inventory struct {
	GoFiles            int   `json:"go_files"`
	GoooFiles          int   `json:"gooo_files"`
	GoPhysicalLines    int   `json:"go_physical_lines"`
	GoooPhysicalLines  int   `json:"gooo_physical_lines"`
	DescendantDirs     int   `json:"descendant_dirs"`
	RegularFiles       int   `json:"regular_files"`
	GeneratedFiles     int   `json:"generated_files"`
	GeneratedBytes     int64 `json:"generated_bytes"`
	RootReadmeExcluded bool  `json:"root_readme_excluded"`
}

type Metrics struct {
	GoFiles           int   `json:"go_files"`
	GoooFiles         int   `json:"gooo_files"`
	GoPhysicalLines   int   `json:"go_physical_lines"`
	GoooPhysicalLines int   `json:"gooo_physical_lines"`
	DescendantDirs    int   `json:"descendant_dirs"`
	RegularFiles      int   `json:"regular_files"`
	GeneratedFiles    int   `json:"generated_files"`
	GeneratedBytes    int64 `json:"generated_bytes"`
	WallMS            int   `json:"wall_ms"`
	PeakRSSKiB        int   `json:"peak_rss_kib"`
	TestsTotal        int   `json:"tests_total"`
	TestsSelected     int   `json:"tests_selected"`
	TestsExecuted     int   `json:"tests_executed"`
	TestsReused       int   `json:"tests_reused"`
	TestsFailed       int   `json:"tests_failed"`
	TestsUnknown      int   `json:"tests_unknown"`
	RepositoryWrites  int   `json:"repository_writes"`
}

type Summary struct {
	CasesTotal             int `json:"cases_total"`
	Closed                 int `json:"closed"`
	Unknown                int `json:"unknown"`
	Refuted                int `json:"refuted"`
	ObservedFalseNegatives int `json:"observed_false_negatives"`
	ReplayComparisons      int `json:"replay_comparisons"`
	ReplayMismatches       int `json:"replay_mismatches"`
	FallbackCases          int `json:"fallback_cases"`
	FullSuiteTests         int `json:"full_suite_tests"`
	SelectedTests          int `json:"selected_tests"`
	SelectedExecuted       int `json:"selected_executed"`
	SelectedReused         int `json:"selected_reused"`
	SelectedFailed         int `json:"selected_failed"`
	SelectedUnknown        int `json:"selected_unknown"`
}

type Report struct {
	Schema                      string       `json:"schema"`
	Decision                    string       `json:"decision"`
	SourceDigest                string       `json:"source_digest"`
	ContractDigest              string       `json:"contract_digest"`
	IRDigest                    string       `json:"ir_digest"`
	DenominatorID               string       `json:"denominator_id"`
	FixedConformanceDenominator int          `json:"fixed_conformance_denominator"`
	Precedence                  []string     `json:"precedence"`
	UnknownFields               []string     `json:"unknown_fields"`
	Authority                   Authority    `json:"authority"`
	Policy                      Policy       `json:"policy"`
	Inventory                   Inventory    `json:"inventory"`
	Summary                     Summary      `json:"summary"`
	Metrics                     Metrics      `json:"metrics"`
	Cases                       []CaseReport `json:"cases"`
	Improvement                 Claim        `json:"improvement"`
	RepositoryWrites            int          `json:"repository_writes"`
}

type ExecutionReceipt struct {
	Schema              string `json:"schema"`
	SourceToIR          string `json:"source_to_ir"`
	IRToSelection       string `json:"ir_to_selection"`
	SelectionToRun      string `json:"selection_to_run"`
	Cases               int    `json:"cases"`
	CallerOwnedOutput   bool   `json:"caller_owned_output"`
	RepositoryWrites    int    `json:"repository_writes"`
	LocalTestExecutions int    `json:"local_test_executions"`
	CacheReused         int    `json:"cache_reused"`
}

func sortedCopy(values []string) []string {
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result
}
