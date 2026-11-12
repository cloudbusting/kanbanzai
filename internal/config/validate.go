package config

// ValidateText loads the configuration a project would have if text were its .kanbanzai/config.toml, without
// reading or writing any file: the defaults merged with the text, then the same schema and build checks as New. It
// lets `init` prove that the config it is about to write is one the loader reads.
func ValidateText(root, toplevel, gitCommon, text string) (*Project, error) {
	over, err := parseTOML(text)
	if err != nil {
		return nil, errf(".kanbanzai/config.toml: %v", err)
	}
	if err := validateVersion(over, ".kanbanzai/config.toml"); err != nil {
		return nil, err
	}
	if _, err := validateSchema(over, ".kanbanzai/config.toml"); err != nil {
		return nil, err
	}
	project, err := load(root, toplevel, gitCommon, over, false)
	if err != nil {
		return nil, err
	}
	return project, nil
}
