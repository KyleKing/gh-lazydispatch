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
	Workflow string     `yaml:"workflow"`
	Source   SourceKind `yaml:"source"`
	//nolint:tagliatelle // documented config key, changing breaks user YAML
	WaitFor WaitCondition     `yaml:"wait_for"`
	Inputs  map[string]string `yaml:"inputs"`
	//nolint:tagliatelle // documented config key, changing breaks user YAML
	OnFailure FailureAction `yaml:"on_failure"`
}

// SourceKind specifies where a step's run comes from.
type SourceKind string

// Sources a chain step can adopt its run from.
const (
	// SourceDispatch starts a fresh gh workflow run. The default.
	SourceDispatch SourceKind = "dispatch"
	// SourceExisting adopts the newest queued or in-progress run of the
	// step's workflow already on the branch, rather than starting one.
	SourceExisting SourceKind = "existing"
)

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

// ErrBrokenSymlink indicates the configuration path is a symlink whose target does not exist.
var ErrBrokenSymlink = errors.New("config file is a broken symlink")

// ErrConfigNotMapping indicates the file parsed as a bare scalar rather than a mapping of keys.
var ErrConfigNotMapping = errors.New("config file is not a mapping of keys")

// ErrUnsupportedConfigVersion indicates the configuration file declares an unsupported version.
var ErrUnsupportedConfigVersion = errors.New("unsupported config version (expected 1 or 2)")

// ErrUnknownSource indicates a step's source is neither empty, "dispatch", nor "existing".
var ErrUnknownSource = errors.New("unknown step source")

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
			return nil, missingFileErr(path)
		}

		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if scalar, ok := bareScalar(data); ok {
		return nil, fmt.Errorf("%s: %w: it holds only %q. A symlink written as a text file"+
			" reads this way; recreate it with ln -s", path, ErrConfigNotMapping, scalar)
	}

	var config WfdConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if config.Version != 1 && config.Version != 2 {
		return nil, fmt.Errorf("%w: got %d", ErrUnsupportedConfigVersion, config.Version)
	}

	if err := applyDefaults(&config); err != nil {
		return nil, err
	}

	return &config, nil
}

// applyDefaults fills the optional keys a step or variable left unset.
func applyDefaults(config *WfdConfig) error {
	for name, chain := range config.Chains {
		for i := range chain.Steps {
			if chain.Steps[i].WaitFor == "" {
				chain.Steps[i].WaitFor = WaitSuccess
			}

			if chain.Steps[i].OnFailure == "" {
				chain.Steps[i].OnFailure = FailureAbort
			}

			switch chain.Steps[i].Source {
			case "":
				chain.Steps[i].Source = SourceDispatch
			case SourceDispatch, SourceExisting:
			default:
				return fmt.Errorf("chain %q step %d (%s): %w: %q",
					name, i, chain.Steps[i].Workflow, ErrUnknownSource, chain.Steps[i].Source)
			}
		}

		for i := range chain.Variables {
			if chain.Variables[i].Type == "" {
				chain.Variables[i].Type = "string"
			}
		}

		config.Chains[name] = chain
	}

	return nil
}

// missingFileErr explains an unreadable path: a dangling symlink names its
// target, because a relative target resolves against the link's own directory
// rather than the working directory, and that is the mistake behind it.
func missingFileErr(path string) error {
	target, resolved, ok := symlinkTarget(path)
	if !ok {
		return fmt.Errorf("%s: %w", path, ErrConfigNotFound)
	}

	return fmt.Errorf("%s: %w: its target %q resolves to %s, which does not exist."+
		" A relative target resolves against the link's own directory, %s",
		path, ErrBrokenSymlink, target, resolved, filepath.Dir(path))
}

// symlinkTarget reports the link target at path and where it resolves to,
// or a false third value when path is not a readable symlink.
func symlinkTarget(path string) (string, string, bool) {
	info, err := os.Lstat(path)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return "", "", false
	}

	target, err := os.Readlink(path)
	if err != nil {
		return "", "", false
	}

	resolved := target
	if !filepath.IsAbs(target) {
		resolved = filepath.Join(filepath.Dir(path), target)
	}

	return target, resolved, true
}

// bareScalar reports the document's content when it parsed as a single scalar
// rather than a mapping, which is what a path written into the file looks like.
func bareScalar(data []byte) (string, bool) {
	var doc any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return "", false
	}

	scalar, ok := doc.(string)

	return scalar, ok
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
