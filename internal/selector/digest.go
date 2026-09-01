package selector

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

func DigestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func DigestValue(value any) (string, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return DigestBytes(raw), nil
}

func WriteJSON(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	raw = append(raw, '\n')
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, raw, 0o644)
}

func ReadJSON(path string, destination any) error {
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(raw, destination); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}

func unsignedIRDigest(ir IR) (string, error) {
	ir.IRDigest = ""
	return DigestValue(ir)
}

func canonicalGraph(graph Graph) Graph {
	result := graph
	result.Nodes = append([]Node(nil), graph.Nodes...)
	result.Edges = append([]Edge(nil), graph.Edges...)
	result.Witnesses = append([]Witness(nil), graph.Witnesses...)
	return result
}

func graphDigest(graph Graph) (string, error) {
	return DigestValue(canonicalGraph(graph))
}

func scenarioDigest(source Source, caseDecl Case) (string, error) {
	return DigestValue(struct {
		Denominator string `json:"denominator"`
		Graph       Graph  `json:"graph"`
		Case        Case   `json:"case"`
		Policy      Policy `json:"policy"`
	}{source.DenominatorID, Graph{Schema: GraphSchema, Nodes: source.Nodes, Edges: source.Edges, Witnesses: source.Witnesses}, caseDecl, source.Policy})
}
