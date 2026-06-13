package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	yamlPkg "gopkg.in/yaml.v3"
)

var validConfigYAML = `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
    cooldown: 60
    condition:
      type: threshold
      threshold: 5
      window: 60
    action:
      type: log
      template: "fake template"
  `

func loadInvalidConfig(t *testing.T, yaml string) (*Config, error) {
	cfg, err := LoadFromString([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	return cfg, err
}

func loadValidConfig(t *testing.T, yaml string) *Config {
	cfg, err := LoadFromString([]byte(yaml))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = Validate(cfg)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	return cfg
}
func TestConfigFileNotFound(t *testing.T) {
	_, err := LoadFromFile("/tmp/this-file-does-not-exist.yaml")
	if !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("expected 'failed to read', got %v", err)
	}
}

func createInvalidYAMLConfigFile(t *testing.T) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte("invalid yaml"), 0644)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	return configFile
}

func createValidConfigFile(t *testing.T) string {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(validConfigYAML), 0644)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	return configFile
}

func createInvalidDataConfigFile(t *testing.T) string {
	yaml := `rules:
  - file: logs/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
  `
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")
	err := os.WriteFile(configFile, []byte(yaml), 0644)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	return configFile
}
func TestConfigFileValid(t *testing.T) {
	configFile := createValidConfigFile(t)
	_, err := LoadFromFile(configFile)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConfigFileInvalidYAML(t *testing.T) {
	configFile := createInvalidYAMLConfigFile(t)
	_, err := LoadFromFile(configFile)
	if !strings.Contains(err.Error(), "failed to parse YAML data") {
		t.Fatalf("expected 'failed to parse YAML data', got %v", err)
	}
}

func TestConfigFileInvalidData(t *testing.T) {
	configFile := createInvalidDataConfigFile(t)
	_, err := LoadFromFile(configFile)
	if !strings.Contains(err.Error(), "is required") {
		t.Fatalf("expected 'is required', got %v", err)
	}
}

func TestEmptyRules(t *testing.T) {
	yaml := `rules: []`

	cfg, _ := LoadFromString([]byte(yaml))
	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if err.Error() != "no rules found" {
		t.Fatalf("expected 'no rules found', got %v", err)
	}
}

func TestRuleValid(t *testing.T) {
	loadValidConfig(t, validConfigYAML)
}

func TestRuleInvalidCooldown(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
    cooldown: -1
  `
	_, err := LoadFromString([]byte(yaml))
	if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("expected 'cannot unmarshal', got %v", err)
	}
}
func TestRuleMissingName(t *testing.T) {
	yaml := `rules:
  - file: logs/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
`

	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`name` is required") {
		t.Fatalf("expected '`name` is required', got %v", err)
	}
}

func TestRuleMissingFile(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
  `

	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`file` is required") {
		t.Fatalf("expected '`file` is required', got %v", err)
	}
}

func TestRuleInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid/path/error.log")
	invalidFolder := filepath.Join(tmpDir, "invalid/path")

	yaml := fmt.Sprintf(`rules:
  - name: upstream_timeout
    file: %s
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
  `, invalidPath)

	_, err := loadInvalidConfig(t, yaml)
	expectedError := fmt.Sprintf("folder %s does not exist (parent of %s)", invalidFolder, invalidPath)
	if !strings.Contains(
		err.Error(),
		expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestRuleMissingPattern(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: logs/error.log
    group_by: backend
  `

	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`pattern` is required") {
		t.Fatalf("expected '`pattern` is required', got %v", err)
	}
}

func TestRuleGroupByNotInPattern(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<host>[0-9a-z.]+):(?P<timeout>\d+)'
    group_by: backend
  `

	_, err := loadInvalidConfig(t, yaml)
	expectedError := "group by backend is not in pattern"
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestRuleGroupByEmpty(t *testing.T) {
	yaml := `rules:
- name: upstream_timeout
  file: /tmp/error.log
  pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
  condition:
    type: threshold
    threshold: 5
    window: 60
  action:
    type: log
    template: "fake template"
  `

	cfg := loadValidConfig(t, yaml)
	if cfg.Rules[0].GroupBy != "" {
		t.Fatalf("expected '`group_by` to be empty, got %v", cfg.Rules[0].GroupBy)
	}
}

func TestRuleConditionMissing(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
  `

	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`condition` is required") {
		t.Fatalf("expected '`condition` is required', got %v", err)
	}
}

func TestRuleInvalidConditionType(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    condition:
      type: invalid
    action:
      type: log
      template: "fake template"
  `
	_, err := LoadFromString([]byte(yaml))
	expectedError := "condition: unknown type \"invalid\""
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestRuleActionMissing(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    condition:
      type: threshold
      threshold: 5
      window: 60
  `

	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`action` is required") {
		t.Fatalf("expected '`action` is required', got %v", err)
	}
}

func TestRuleInvalidActionType(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)'
    condition:
      type: threshold
      threshold: 5
      window: 60
    action:
      type: invalid
  `
	_, err := LoadFromString([]byte(yaml))
	expectedError := "action: unknown type \"invalid\""
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestConditionConfigSeenValid(t *testing.T) {
	yaml := `type: seen`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionConfigThresholdValid(t *testing.T) {
	yaml := `type: threshold
threshold: 5
window: 60
  `
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionConfigAbsenceValid(t *testing.T) {
	yaml := `type: absence
duration: 60
`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionConfigUnknownType(t *testing.T) {
	yaml := `type: invalid`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	expectedError := "condition: unknown type \"invalid\""
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestConditionConfigThresholdInvalid(t *testing.T) {
	yaml := `type: threshold
threshold: 0
window: 60
`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`threshold` must be greater than 0") {
		t.Fatalf("expected threshold validation error, got %v", err)
	}
}

func TestConditionConfigThresholdInvalidWindow(t *testing.T) {
	yaml := `type: threshold
threshold: 5
window: 0
`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`window` must be greater than 0") {
		t.Fatalf("expected window validation error, got %v", err)
	}
}

func TestConditionConfigAbsenceInvalid(t *testing.T) {
	yaml := `type: absence
duration: 0
`
	var data ConditionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`duration` must be defined and greater than 0") {
		t.Fatalf("expected duration validation error, got %v", err)
	}
}

func TestActionConfigLogValid(t *testing.T) {
	yaml := `type: log
template: "rule={{ .Rule }}"
`
	var data ActionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestActionConfigShellValid(t *testing.T) {
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	yaml := fmt.Sprintf(`type: shell
script: %s
`, script)
	var data ActionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestActionConfigUnknownType(t *testing.T) {
	yaml := `type: invalid`
	var data ActionConfig
	err := yamlPkg.Unmarshal([]byte(yaml), &data)
	expectedError := "action: unknown type \"invalid\""
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected '%s', got %v", expectedError, err)
	}
}

func TestStringMethods(t *testing.T) {
	var nilAction ActionConfig
	if nilAction.String() != "<nil>" {
		t.Fatalf("expected '<nil>', got %q", nilAction.String())
	}

	var nilCondition ConditionConfig
	if nilCondition.String() != "<nil>" {
		t.Fatalf("expected '<nil>', got %q", nilCondition.String())
	}

	rule := Rule{
		Name:    "test_rule",
		File:    "/tmp/error.log",
		Pattern: "error",
		GroupBy: "backend",
	}
	expected := `{Name:"test_rule" File:"/tmp/error.log" Pattern:"error" GroupBy:"backend" Condition:{<nil>} Action:{<nil>}}`
	if rule.String() != expected {
		t.Fatalf("expected %q, got %q", expected, rule.String())
	}

	yaml := `type: seen`
	var condition ConditionConfig
	if err := yamlPkg.Unmarshal([]byte(yaml), &condition); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if condition.String() != "seen()" {
		t.Fatalf("expected 'seen()', got %q", condition.String())
	}

	yaml = `type: log
template: "hello"
`
	var action ActionConfig
	if err := yamlPkg.Unmarshal([]byte(yaml), &action); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if !strings.HasPrefix(action.String(), "log(") {
		t.Fatalf("expected log action string, got %q", action.String())
	}
}

func TestRuleValidSeenCondition(t *testing.T) {
	yaml := `rules:
  - name: connection_failed
    file: /tmp/error.log
    pattern: 'connect failed'
    condition:
      type: seen
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	loadValidConfig(t, yaml)
}

func TestRuleValidAbsenceCondition(t *testing.T) {
	yaml := `rules:
  - name: heartbeat_missing
    file: /tmp/error.log
    pattern: 'heartbeat ok'
    condition:
      type: absence
      duration: 10
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	loadValidConfig(t, yaml)
}

func TestRuleValidShellAction(t *testing.T) {
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	yaml := fmt.Sprintf(`rules:
  - name: alert
    file: /tmp/error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: shell
      script: %s
`, script)
	loadValidConfig(t, yaml)
}

func TestRuleFilePathAbsolute(t *testing.T) {
	yaml := `rules:
  - name: test_rule
    file: error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	cfg := loadValidConfig(t, yaml)
	if !filepath.IsAbs(cfg.Rules[0].File) {
		t.Fatalf("expected absolute file path, got %q", cfg.Rules[0].File)
	}
}

func TestRuleInvalidThresholdCondition(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout'
    condition:
      type: threshold
      threshold: 0
      window: 60
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`threshold` must be greater than 0") {
		t.Fatalf("expected threshold validation error, got %v", err)
	}
}

func TestRuleInvalidThresholdWindow(t *testing.T) {
	yaml := `rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout'
    condition:
      type: threshold
      threshold: 5
      window: 0
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`window` must be greater than 0") {
		t.Fatalf("expected window validation error, got %v", err)
	}
}

func TestRuleInvalidAbsenceCondition(t *testing.T) {
	yaml := `rules:
  - name: heartbeat_missing
    file: /tmp/error.log
    pattern: 'heartbeat ok'
    condition:
      type: absence
      duration: 0
    action:
      type: log
      template: "rule={{ .Rule }}"
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`duration` must be defined and greater than 0") {
		t.Fatalf("expected duration validation error, got %v", err)
	}
}

func TestRuleInvalidLogTemplate(t *testing.T) {
	yaml := `rules:
  - name: test_rule
    file: /tmp/error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: log
      template: "{{invalid"
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "failed to parse log template") {
		t.Fatalf("expected log template parse error, got %v", err)
	}
}

func TestRuleInvalidShellActionMissingScript(t *testing.T) {
	yaml := `rules:
  - name: alert
    file: /tmp/error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: shell
      script: /tmp/does-not-exist.sh
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "does not exist") {
		t.Fatalf("expected script does not exist error, got %v", err)
	}
}

func TestRuleInvalidShellActionNotExecutable(t *testing.T) {
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	yaml := fmt.Sprintf(`rules:
  - name: alert
    file: /tmp/error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: shell
      script: %s
`, script)
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "is not executable") {
		t.Fatalf("expected script is not executable error, got %v", err)
	}
}

func TestRuleInvalidShellActionMissingScriptField(t *testing.T) {
	yaml := `rules:
  - name: alert
    file: /tmp/error.log
    pattern: 'error'
    condition:
      type: seen
    action:
      type: shell
`
	_, err := loadInvalidConfig(t, yaml)
	if !strings.Contains(err.Error(), "`script` is required") {
		t.Fatalf("expected '`script` is required', got %v", err)
	}
}
