package selector

func Compile(root, sourcePath, contractPath, outputPath string) (IR, error) {
	if _, err := ensureCallerFile(root, outputPath); err != nil {
		return IR{}, err
	}
	source, err := LoadSource(sourcePath)
	if err != nil {
		return IR{}, err
	}
	contract, err := LoadContract(contractPath)
	if err != nil {
		return IR{}, err
	}
	ir, err := Lower(source, contract)
	if err != nil {
		return IR{}, err
	}
	if err := WriteJSON(outputPath, ir); err != nil {
		return IR{}, err
	}
	return ir, nil
}
