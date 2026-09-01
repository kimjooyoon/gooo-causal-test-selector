package selector

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func ParseSource(raw []byte) (Source, error) {
	source := Source{SourceDigest: DigestBytes(raw)}
	scanner := bufio.NewScanner(bytes.NewReader(raw))
	lineNumber := 0
	headerSeen := false
	for scanner.Scan() {
		lineNumber++
		line := stripComment(scanner.Text())
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		values, err := keyValues(fields[1:])
		if err != nil && fields[0] != "gooo" && fields[0] != "precedence" && fields[0] != "unknown_fields" {
			return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
		}
		switch fields[0] {
		case "gooo":
			if headerSeen || len(fields) != 3 || fields[1] != "causal_test_selector" || fields[2] != "v1" {
				return Source{}, fmt.Errorf("line %d: invalid gooo header", lineNumber)
			}
			headerSeen = true
			source.Schema = SourceSchema
			source.Version = fields[2]
		case "denominator":
			source.DenominatorID = values["id"]
			source.CaseCount, err = parseInt(values, "case_count")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
		case "authority":
			source.Authority.RepositoryWrites, err = parseInt(values, "repository_writes")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			source.Authority.LocalTestExecutions, err = parseInt(values, "local_test_executions")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			source.Authority.ExternalRequiredGates, err = parseInt(values, "external_required_gates")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
		case "precedence":
			if len(fields) != 2 {
				return Source{}, fmt.Errorf("line %d: invalid precedence", lineNumber)
			}
			source.Precedence = strings.Split(fields[1], ">")
		case "unknown_fields":
			if len(fields) != 2 {
				return Source{}, fmt.Errorf("line %d: invalid unknown_fields", lineNumber)
			}
			source.UnknownFields = strings.Split(fields[1], ",")
		case "policy":
			source.Policy = Policy{
				MissingEdge: values["missing_edge"], StaleEdge: values["stale_edge"],
				SemanticSource: values["semantic_source"],
			}
			source.Policy.FullFallback, err = parseBool(values, "full_fallback")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
			source.Policy.CacheClose, err = parseBool(values, "cache_close")
			if err != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, err)
			}
		case "node":
			source.Nodes = append(source.Nodes, Node{ID: values["id"], Kind: values["kind"]})
		case "edge":
			source.Edges = append(source.Edges, Edge{ID: values["id"], From: values["from"], To: values["to"], Kind: values["kind"], Status: values["status"], Provenance: values["provenance"]})
		case "witness":
			source.Witnesses = append(source.Witnesses, Witness{ID: values["id"], TestID: values["test_id"], Targets: parseList(values["targets"]), Command: values["command"], Provenance: values["provenance"]})
		case "case":
			ordinal, parseErr := parseInt(values, "ordinal")
			if parseErr != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, parseErr)
			}
			replay, parseErr := parseBool(values, "replay")
			if parseErr != nil {
				return Source{}, fmt.Errorf("line %d: %w", lineNumber, parseErr)
			}
			source.Cases = append(source.Cases, Case{
				Ordinal: ordinal, ID: values["id"], Kind: values["kind"], Changed: parseList(values["changed"]),
				Expected: values["expected"], FaultTest: emptyMarker(values["fault_test"]),
				TerminalReason: values["terminal_reason"], Effect: values["effect"],
				EdgeFault: values["edge_fault"], FaultEdge: emptyMarker(values["fault_edge"]), Replay: replay,
			})
		default:
			return Source{}, fmt.Errorf("line %d: unsupported declaration %q", lineNumber, fields[0])
		}
	}
	if err := scanner.Err(); err != nil {
		return Source{}, err
	}
	if !headerSeen {
		return Source{}, fmt.Errorf("source has no gooo header")
	}
	if err := ValidateSource(source); err != nil {
		return Source{}, err
	}
	return source, nil
}

func LoadSource(path string) (Source, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Source{}, err
	}
	return ParseSource(raw)
}

func LoadContract(path string) (Contract, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Contract{}, err
	}
	var contract Contract
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("decode contract: %w", err)
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func Lower(source Source, contract Contract) (IR, error) {
	if err := ValidateDeclarations(source, contract); err != nil {
		return IR{}, err
	}
	graph := Graph{Schema: GraphSchema, Nodes: append([]Node(nil), source.Nodes...), Edges: append([]Edge(nil), source.Edges...), Witnesses: append([]Witness(nil), source.Witnesses...)}
	graphDigestValue, err := graphDigest(graph)
	if err != nil {
		return IR{}, err
	}
	ir := IR{
		Schema: IRSchema, Version: "v1", SourceDigest: source.SourceDigest,
		ContractDigest: contractDigest(contract), DenominatorID: source.DenominatorID,
		GraphDigest: graphDigestValue, Graph: graph, Cases: append([]Case(nil), source.Cases...),
		Policy: source.Policy, Authority: source.Authority,
	}
	ir.IRDigest, err = unsignedIRDigest(ir)
	if err != nil {
		return IR{}, err
	}
	return ir, nil
}

func ValidateIR(ir IR) error {
	if ir.Schema != IRSchema || ir.Version != "v1" || ir.DenominatorID == "" || ir.IRDigest == "" || ir.GraphDigest == "" || len(ir.Cases) != FixedCases {
		return fmt.Errorf("invalid semantic IR envelope")
	}
	if ir.Graph.Schema != GraphSchema {
		return fmt.Errorf("invalid semantic graph schema")
	}
	if err := validateGraphAndCases(Source{
		Nodes: ir.Graph.Nodes, Edges: ir.Graph.Edges, Witnesses: ir.Graph.Witnesses,
		Cases: ir.Cases,
	}); err != nil {
		return err
	}
	graphDigestValue, err := graphDigest(ir.Graph)
	if err != nil {
		return err
	}
	if graphDigestValue != ir.GraphDigest {
		return fmt.Errorf("semantic graph digest mismatch")
	}
	expected, err := unsignedIRDigest(ir)
	if err != nil {
		return err
	}
	if expected != ir.IRDigest {
		return fmt.Errorf("semantic IR digest mismatch")
	}
	return nil
}

func ValidateSource(source Source) error {
	if source.Schema != SourceSchema || source.Version != "v1" || source.DenominatorID == "" || source.CaseCount != FixedCases {
		return fmt.Errorf("invalid source envelope")
	}
	if len(source.Nodes) == 0 || len(source.Edges) == 0 || len(source.Witnesses) == 0 || len(source.Cases) != FixedCases {
		return fmt.Errorf("source graph or fixed cases are incomplete")
	}
	if source.Authority != (Authority{}) {
		return fmt.Errorf("source authority must declare zero writes, local tests, and external gates")
	}
	if !sameStrings(source.Precedence, Precedence) {
		return fmt.Errorf("resolution precedence must be REFUTED>UNKNOWN>CLOSED")
	}
	if !sameStrings(source.UnknownFields, []string{"stage", "step", "reason", "unknown_class", "next_operation", "blocked_by"}) {
		return fmt.Errorf("UNKNOWN six-field contract mismatch")
	}
	if source.Policy.MissingEdge != DecisionUnknown || source.Policy.StaleEdge != DecisionRefuted || !source.Policy.FullFallback || source.Policy.CacheClose || source.Policy.SemanticSource != "GOOO" {
		return fmt.Errorf("selection policy is not fail-closed")
	}
	return validateGraphAndCases(source)
}

func ValidateContract(contract Contract) error {
	if contract.Schema != ContractSchema || contract.Version != "v1" || contract.ID == "" || !contract.Fixed || contract.CaseCount != FixedCases || len(contract.Cases) != FixedCases {
		return fmt.Errorf("invalid fixed denominator contract")
	}
	seen := map[string]bool{}
	for index, item := range contract.Cases {
		if item.Ordinal != index+1 || item.ID == "" || item.Kind == "" || seen[item.ID] || !validDecision(item.Expected) {
			return fmt.Errorf("invalid fixed denominator case %d", index+1)
		}
		seen[item.ID] = true
	}
	for _, required := range requiredMetricNames() {
		if !contains(contract.RequiredMetrics, required) {
			return fmt.Errorf("required metric missing from contract: %s", required)
		}
	}
	return nil
}

func ValidateDeclarations(source Source, contract Contract) error {
	if err := ValidateSource(source); err != nil {
		return err
	}
	if err := ValidateContract(contract); err != nil {
		return err
	}
	if source.DenominatorID != contract.ID || source.CaseCount != contract.CaseCount {
		return fmt.Errorf("source and fixed denominator identity mismatch")
	}
	for index, expected := range contract.Cases {
		actual := source.Cases[index]
		if actual.Ordinal != expected.Ordinal || actual.ID != expected.ID || actual.Kind != expected.Kind || actual.Expected != expected.Expected {
			return fmt.Errorf("source case %d does not match fixed denominator", index+1)
		}
	}
	return nil
}

func validateGraphAndCases(source Source) error {
	nodes := map[string]bool{}
	for _, node := range source.Nodes {
		if node.ID == "" || node.Kind == "" || nodes[node.ID] {
			return fmt.Errorf("invalid or duplicate semantic node")
		}
		nodes[node.ID] = true
	}
	edges := map[string]Edge{}
	edgePairs := map[string]bool{}
	for _, edge := range source.Edges {
		pair := edge.From + "\x00" + edge.To + "\x00" + edge.Kind
		if edge.ID == "" || edge.From == "" || edge.To == "" || edge.Kind == "" || edge.Status != "ACTIVE" || edge.Provenance == "" || !nodes[edge.From] || !nodes[edge.To] || edges[edge.ID].ID != "" || edgePairs[pair] {
			return fmt.Errorf("invalid semantic/provenance edge %s", edge.ID)
		}
		edges[edge.ID] = edge
		edgePairs[pair] = true
	}
	tests := map[string]bool{}
	witnesses := map[string]bool{}
	for _, witness := range source.Witnesses {
		if witness.ID == "" || witness.TestID == "" || witness.Command == "" || witness.Provenance == "" || tests[witness.TestID] || witnesses[witness.ID] || len(witness.Targets) == 0 {
			return fmt.Errorf("invalid or duplicate test witness %s", witness.ID)
		}
		for _, target := range witness.Targets {
			if !nodes[target] {
				return fmt.Errorf("test witness %s targets unknown semantic node %s", witness.ID, target)
			}
		}
		tests[witness.TestID] = true
		witnesses[witness.ID] = true
	}
	caseIDs := map[string]bool{}
	for index, caseDecl := range source.Cases {
		if caseDecl.Ordinal != index+1 || caseDecl.ID == "" || caseDecl.Kind == "" || caseIDs[caseDecl.ID] || !validDecision(caseDecl.Expected) || caseDecl.TerminalReason == "" || caseDecl.Effect == "" || !contains([]string{"none", "MISSING", "STALE"}, caseDecl.EdgeFault) {
			return fmt.Errorf("invalid fixed case %s", caseDecl.ID)
		}
		caseIDs[caseDecl.ID] = true
		for _, changed := range caseDecl.Changed {
			if !nodes[changed] {
				return fmt.Errorf("case %s changes unknown semantic node %s", caseDecl.ID, changed)
			}
		}
		if caseDecl.FaultTest != "" && !tests[caseDecl.FaultTest] {
			return fmt.Errorf("case %s names unknown fault test", caseDecl.ID)
		}
		if caseDecl.EdgeFault == "none" && caseDecl.FaultEdge != "" {
			return fmt.Errorf("case %s has a fault edge without an edge fault", caseDecl.ID)
		}
		if caseDecl.EdgeFault != "none" && caseDecl.FaultEdge == "" {
			return fmt.Errorf("case %s has an edge fault without a fault edge", caseDecl.ID)
		}
		if caseDecl.EdgeFault != "none" && edges[caseDecl.FaultEdge].ID == "" {
			return fmt.Errorf("case %s names unknown fault edge", caseDecl.ID)
		}
	}
	return nil
}

func validateGraph(graph Graph) error {
	nodes := map[string]bool{}
	for _, node := range graph.Nodes {
		if node.ID == "" || node.Kind == "" || nodes[node.ID] {
			return fmt.Errorf("invalid or duplicate semantic node")
		}
		nodes[node.ID] = true
	}
	edges := map[string]bool{}
	for _, edge := range graph.Edges {
		if edge.ID == "" || edge.From == "" || edge.To == "" || edge.Kind == "" || edge.Status != "ACTIVE" || edge.Provenance == "" || !nodes[edge.From] || !nodes[edge.To] || edges[edge.ID] {
			return fmt.Errorf("invalid semantic/provenance edge %s", edge.ID)
		}
		edges[edge.ID] = true
	}
	tests := map[string]bool{}
	for _, witness := range graph.Witnesses {
		if witness.ID == "" || witness.TestID == "" || witness.Command == "" || witness.Provenance == "" || tests[witness.TestID] || len(witness.Targets) == 0 {
			return fmt.Errorf("invalid or duplicate test witness %s", witness.ID)
		}
		for _, target := range witness.Targets {
			if !nodes[target] {
				return fmt.Errorf("test witness %s targets unknown semantic node %s", witness.ID, target)
			}
		}
		tests[witness.TestID] = true
	}
	return nil
}

func contractDigest(contract Contract) string {
	digest, _ := DigestValue(contract)
	return digest
}

func requiredMetricNames() []string {
	return []string{"go_files", "gooo_files", "go_physical_lines", "gooo_physical_lines", "descendant_dirs", "regular_files", "generated_files", "generated_bytes", "wall_ms", "peak_rss_kib", "tests_total", "tests_selected", "tests_executed", "tests_reused", "tests_failed", "tests_unknown", "repository_writes"}
}

func parseInt(values map[string]string, key string) (int, error) {
	value, ok := values[key]
	if !ok {
		return 0, fmt.Errorf("missing %s", key)
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("invalid %s %q", key, value)
	}
	return parsed, nil
}

func parseBool(values map[string]string, key string) (bool, error) {
	value, ok := values[key]
	if !ok {
		return false, fmt.Errorf("missing %s", key)
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("invalid %s %q", key, value)
	}
	return parsed, nil
}

func keyValues(fields []string) (map[string]string, error) {
	values := make(map[string]string, len(fields))
	for _, field := range fields {
		parts := strings.SplitN(field, "=", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid key/value %q", field)
		}
		if _, exists := values[parts[0]]; exists {
			return nil, fmt.Errorf("duplicate key %q", parts[0])
		}
		values[parts[0]] = strings.Trim(parts[1], "\"'")
	}
	return values, nil
}

func stripComment(value string) string {
	for _, marker := range []string{"#", "//"} {
		if index := strings.Index(value, marker); index >= 0 {
			value = value[:index]
		}
	}
	return strings.TrimSpace(value)
}

func parseList(value string) []string {
	if value == "" || value == "-" || value == "none" {
		return []string{}
	}
	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		if part = strings.TrimSpace(part); part != "" && part != "-" {
			result = append(result, part)
		}
	}
	return result
}

func emptyMarker(value string) string {
	if value == "" || value == "none" || value == "-" {
		return ""
	}
	return value
}

func sameStrings(left, right []string) bool {
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

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

func validDecision(value string) bool {
	return value == DecisionClosed || value == DecisionUnknown || value == DecisionRefuted
}
