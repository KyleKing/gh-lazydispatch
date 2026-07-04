// Package config provides configuration file parsing and management for workflow dispatch chains.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ConfigFilename is the default name for the lazydispatch configuration file.
const ConfigFilename = ".github/lazydispatch.yml"

// WfdConfig represents the lazydispatch configuration file.
type WfdConfig struct {
	Chains  map[string]Chain `yaml:"chains"`
	Version int              `yaml:"version"`
}

// ChainVariable represents a variable that can be set when running a chain.
type ChainVariable struct {
	Name        string   `yaml:"name"`
	Type        string   `yaml:"type"`
	Description string   `yaml:"description"`
	Default     string   `yaml:"default"`
	Options     []string `yaml:"options"`
	Required    bool     `yaml:"required"`
}

// Chain represents a workflow chain definition.
type Chain struct {
	Description string          `yaml:"description"`
	Variables   []ChainVariable `yaml:"variables"`
	Steps       []ChainStep     `yaml:"steps"`
}

// ChainStep represents a single step in a workflow chain.
type ChainStep struct {
	Workflow string `yaml:"workflow"`
	//nolint:tagliatelle // documented config key, changing breaks user YAML
	WaitFor WaitCondition     `yaml:"wait_for"`
	Inputs  map[string]string `yaml:"inputs"`
	//nolint:tagliatelle // documented config key, changing breaks user YAML
	OnFailure FailureAction `yaml:"on_failure"`
}

// WaitCondition specifies when to proceed to the next step.
type WaitCondition string

// Wait conditions for a chain step.
const (
	WaitSuccess    WaitCondition = "success"
	WaitCompletion WaitCondition = "completion"
	WaitNone       WaitCondition = "none"
)

// FailureAction specifies what to do when a step fails.
type FailureAction string

// Failure actions for a chain step.
const (
	FailureAbort    FailureAction = "abort"
	FailureSkip     FailureAction = "skip"
	FailureContinue FailureAction = "continue"
)

// ErrConfigNotFound indicates the configuration file does not exist at the given path.
var ErrConfigNotFound = errors.New("config file not found")

// ErrUnsupportedConfigVersion indicates the configuration file declares an unsupported version.
var ErrUnsupportedConfigVersion = errors.New("unsupported config version (expected 1 or 2)")

// Load loads the configuration from the default location.
func Load(repoRoot string) (*WfdConfig, error) {
	configPath := filepath.Join(repoRoot, ConfigFilename)
	return LoadFrom(configPath)
}

// LoadFrom loads the configuration from a specific path.
// Returns ErrConfigNotFound if no file exists at path.
func LoadFrom(path string) (*WfdConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path built from repo root + fixed config filename
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s: %w", path, ErrConfigNotFound)
		}

		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config WfdConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Version != 1 && config.Version != 2 {
		return nil, fmt.Errorf("%w: got %d", ErrUnsupportedConfigVersion, config.Version)
	}

	for name, chain := range config.Chains {
		for i := range chain.Steps {
			if chain.Steps[i].WaitFor == "" {
				chain.Steps[i].WaitFor = WaitSuccess
			}

			if chain.Steps[i].OnFailure == "" {
				chain.Steps[i].OnFailure = FailureAbort
			}
		}

		for i := range chain.Variables {
			if chain.Variables[i].Type == "" {
				chain.Variables[i].Type = "string"
			}
		}

		config.Chains[name] = chain
	}

	return &config, nil
}

// GetChain returns a chain by name.
func (c *WfdConfig) GetChain(name string) (*Chain, bool) {
	if c == nil || c.Chains == nil {
		return nil, false
	}

	chain, ok := c.Chains[name]
	if !ok {
		return nil, false
	}

	return &chain, true
}

// ChainNames returns a sorted list of chain names.
func (c *WfdConfig) ChainNames() []string {
	if c == nil || c.Chains == nil {
		return nil
	}

	names := make([]string, 0, len(c.Chains))
	for name := range c.Chains {
		names = append(names, name)
	}

	sort.Strings(names)

	return names
}

// HasChains returns true if any chains are defined.
func (c *WfdConfig) HasChains() bool {
	return c != nil && len(c.Chains) > 0
}
