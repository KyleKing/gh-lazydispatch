package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kyleking/gh-lazydispatch/internal/config"
)

func TestLoad_ValidConfig(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 1
chains:
  deploy:
    description: "Deploy workflow"
    steps:
      - workflow: build.yml
        wait_for: success
        on_failure: abort
      - workflow: deploy.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Version != 1 {
		t.Errorf("version: got %d, want 1", cfg.Version)
	}
	if !cfg.HasChains() {
		t.Error("expected HasChains() to return true")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	dir := t.TempDir()
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if cfg != nil {
		t.Error("expected nil config for missing file")
	}
}

func TestLoad_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte("invalid: [yaml: content"), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected error for invalid YAML")
	}
}

func TestLoad_UnsupportedVersion(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 99
chains: {}
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err := config.Load(dir)
	if err == nil {
		t.Error("expected error for unsupported version")
	}
}

func TestLoad_DefaultValues(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 1
chains:
  test:
    steps:
      - workflow: test.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, ok := cfg.GetChain("test")
	if !ok {
		t.Fatal("expected chain 'test' to exist")
	}
	if len(chain.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(chain.Steps))
	}

	step := chain.Steps[0]
	if step.WaitFor != config.WaitSuccess {
		t.Errorf("WaitFor: got %q, want %q", step.WaitFor, config.WaitSuccess)
	}
	if step.OnFailure != config.FailureAbort {
		t.Errorf("OnFailure: got %q, want %q", step.OnFailure, config.FailureAbort)
	}
}

func TestChainNames_Sorted(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 1
chains:
  zebra:
    steps:
      - workflow: z.yml
  alpha:
    steps:
      - workflow: a.yml
  middle:
    steps:
      - workflow: m.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	names := cfg.ChainNames()
	expected := []string{"alpha", "middle", "zebra"}

	if len(names) != len(expected) {
		t.Fatalf("got %d names, want %d", len(names), len(expected))
	}

	for i, name := range names {
		if name != expected[i] {
			t.Errorf("index %d: got %q, want %q", i, name, expected[i])
		}
	}
}

func TestGetChain_Exists(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 1
chains:
  deploy:
    description: "Deploy chain"
    steps:
      - workflow: deploy.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, ok := cfg.GetChain("deploy")
	if !ok {
		t.Error("expected chain 'deploy' to exist")
	}
	if chain.Description != "Deploy chain" {
		t.Errorf("description: got %q, want %q", chain.Description, "Deploy chain")
	}
}

func TestGetChain_NotFound(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 1
chains:
  deploy:
    steps:
      - workflow: deploy.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, ok := cfg.GetChain("nonexistent")
	if ok {
		t.Error("expected chain 'nonexistent' to not exist")
	}
}

func TestHasChains(t *testing.T) {
	tests := []struct {
		name     string
		config   *config.WfdConfig
		expected bool
	}{
		{"nil config", nil, false},
		{"empty chains", &config.WfdConfig{Chains: map[string]config.Chain{}}, false},
		{"with chains", &config.WfdConfig{Chains: map[string]config.Chain{"test": {}}}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.config.HasChains()
			if result != tt.expected {
				t.Errorf("got %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestLoad_Version2WithVariables(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 2
chains:
  deploy:
    description: "Deploy chain"
    variables:
      - name: version
        type: string
        description: Release version
        default: v1.0.0
        required: true
      - name: env
        type: choice
        options: [staging, production]
        default: staging
      - name: dry_run
        type: boolean
        default: "true"
    steps:
      - workflow: deploy.yml
        inputs:
          version: "{{ var.version }}"
          env: "{{ var.env }}"
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected config, got nil")
	}
	if cfg.Version != 2 {
		t.Errorf("version: got %d, want 2", cfg.Version)
	}

	chain, ok := cfg.GetChain("deploy")
	if !ok {
		t.Fatal("expected chain 'deploy' to exist")
	}

	if len(chain.Variables) != 3 {
		t.Fatalf("expected 3 variables, got %d", len(chain.Variables))
	}

	v := chain.Variables[0]
	if v.Name != "version" {
		t.Errorf("variable name: got %q, want %q", v.Name, "version")
	}
	if v.Type != "string" {
		t.Errorf("variable type: got %q, want %q", v.Type, "string")
	}
	if !v.Required {
		t.Error("expected variable to be required")
	}

	v2 := chain.Variables[1]
	if v2.Type != "choice" {
		t.Errorf("variable type: got %q, want %q", v2.Type, "choice")
	}
	if len(v2.Options) != 2 {
		t.Errorf("expected 2 options, got %d", len(v2.Options))
	}
}

func TestLoad_Version2DefaultVariableType(t *testing.T) {
	dir := t.TempDir()
	configDir := filepath.Join(dir, ".github")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("failed to create .github dir: %v", err)
	}

	configContent := `version: 2
chains:
  test:
    variables:
      - name: simple
        description: No type specified
    steps:
      - workflow: test.yml
`
	configPath := filepath.Join(configDir, "lazydispatch.yml")
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	chain, ok := cfg.GetChain("test")
	if !ok {
		t.Fatal("expected chain 'test' to exist")
	}

	if len(chain.Variables) != 1 {
		t.Fatalf("expected 1 variable, got %d", len(chain.Variables))
	}

	v := chain.Variables[0]
	if v.Type != "string" {
		t.Errorf("default type: got %q, want %q", v.Type, "string")
	}
}
