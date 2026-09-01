package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/kimjooyoon/gooo-causal-test-selector/internal/selector"
)

func main() {
	if len(os.Args) < 2 {
		fatal("usage: gooo-causal-test-selector compile|conformance ...")
	}
	switch os.Args[1] {
	case "compile":
		compile(os.Args[2:])
	case "conformance":
		conformance(os.Args[2:])
	default:
		fatal("usage: gooo-causal-test-selector compile|conformance ...")
	}
}

func compile(args []string) {
	flags := flag.NewFlagSet("compile", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", ".", "input repository root")
	sourcePath := flags.String("source", "", "path to the authoritative .gooo source")
	contractPath := flags.String("contract", "", "path to the fixed denominator contract")
	outputPath := flags.String("output-ir", "", "caller-owned output IR path")
	if err := flags.Parse(args); err != nil {
		os.Exit(2)
	}
	if *sourcePath == "" || *contractPath == "" || *outputPath == "" {
		fatal("--source, --contract, and --output-ir are required")
	}
	output, err := selector.Compile(*root, *sourcePath, *contractPath, *outputPath)
	if err != nil {
		fatal(err.Error())
	}
	printJSON(output)
}

func conformance(args []string) {
	flags := flag.NewFlagSet("conformance", flag.ContinueOnError)
	flags.SetOutput(os.Stderr)
	root := flags.String("root", ".", "input repository root")
	sourcePath := flags.String("source", "", "path to the authoritative .gooo source")
	contractPath := flags.String("contract", "", "path to the fixed denominator contract")
	outputPath := flags.String("output-dir", "", "caller-owned empty output directory")
	runID := flags.String("run-id", "", "CI run identity shared by the full and selected pair")
	if err := flags.Parse(args); err != nil {
		os.Exit(2)
	}
	if *sourcePath == "" || *contractPath == "" || *outputPath == "" {
		fatal("--source, --contract, and --output-dir are required")
	}
	source, err := selector.LoadSource(*sourcePath)
	if err != nil {
		fatal(err.Error())
	}
	contract, err := selector.LoadContract(*contractPath)
	if err != nil {
		fatal(err.Error())
	}
	report, err := selector.Execute(source, contract, *root, *outputPath, *runID)
	if err != nil {
		fatal(err.Error())
	}
	printJSON(report.Summary)
}

func printJSON(value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		fatal(err.Error())
	}
	fmt.Println(string(raw))
}

func fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}
