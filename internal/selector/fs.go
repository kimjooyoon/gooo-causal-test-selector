package selector

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func ensureCallerOutput(root, output string) (string, error) {
	if root == "" || output == "" {
		return "", fmt.Errorf("input root and caller-owned output are required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return "", err
	}
	rootInfo, err := os.Stat(rootAbs)
	if err != nil {
		return "", err
	}
	if !rootInfo.IsDir() {
		return "", fmt.Errorf("input root is not a directory")
	}
	if isWithin(rootAbs, outputAbs) {
		return "", fmt.Errorf("caller-owned output must be outside input repository")
	}
	info, err := os.Stat(outputAbs)
	if os.IsNotExist(err) {
		return outputAbs, nil
	}
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("caller-owned output must be a directory")
	}
	entries, err := os.ReadDir(outputAbs)
	if err != nil {
		return "", err
	}
	if len(entries) != 0 {
		return "", fmt.Errorf("caller-owned output must be empty")
	}
	return outputAbs, nil
}

func ensureCallerFile(root, path string) (string, error) {
	if root == "" || path == "" {
		return "", fmt.Errorf("input root and caller-owned output file are required")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if isWithin(rootAbs, pathAbs) {
		return "", fmt.Errorf("caller-owned output must be outside input repository")
	}
	if _, statErr := os.Stat(pathAbs); statErr == nil {
		return "", fmt.Errorf("caller-owned output file already exists")
	} else if !os.IsNotExist(statErr) {
		return "", statErr
	}
	return pathAbs, nil
}

func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return true
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func inventory(root string) (Inventory, error) {
	var result Inventory
	result.RootReadmeExcluded = true
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path != root && entry.IsDir() && entry.Name() == ".git" {
			return fs.SkipDir
		}
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel == "README.md" {
			return nil
		}
		if entry.IsDir() {
			result.DescendantDirs++
			return nil
		}
		if !entry.Type().IsRegular() {
			return nil
		}
		result.RegularFiles++
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		lines := lineCount(contents)
		switch filepath.Ext(path) {
		case ".go":
			result.GoFiles++
			result.GoPhysicalLines += lines
		case ".gooo":
			result.GoooFiles++
			result.GoooPhysicalLines += lines
		}
		if isGeneratedPath(rel) {
			result.GeneratedFiles++
			result.GeneratedBytes += int64(len(contents))
		}
		return nil
	})
	return result, err
}

func lineCount(contents []byte) int {
	if len(contents) == 0 {
		return 0
	}
	count := strings.Count(string(contents), "\n")
	if contents[len(contents)-1] != '\n' {
		count++
	}
	return count
}

func isGeneratedPath(path string) bool {
	clean := filepath.ToSlash(path)
	return strings.Contains(clean, "/generated/") || strings.HasPrefix(clean, "generated/") || strings.HasSuffix(clean, ".gen.go") || strings.HasSuffix(clean, ".generated.go")
}
