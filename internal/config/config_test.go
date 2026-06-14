package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"pavlov/internal/action"
	"pavlov/internal/condition"
)

const testPatternWithBackend = `timeout: (?P<backend>[0-9a-z.]+):(?P<timeout>\d+)`

func validRule() *Rule {
	return &Rule{
		Name:     "upstream_timeout",
		File:     "/tmp/error.log",
		Pattern:  testPatternWithBackend,
		GroupBy:  "backend",
		Cooldown: 60,
		Condition: ConditionSpec{
			Value: &condition.ThresholdCondition{Threshold: 5, Window: 60},
		},
		Action: ActionSpec{
			Value: &action.LogAction{Template: "fake template"},
		},
	}
}

func configWithRules(rules ...*Rule) *Config {
	return &Config{Rules: rules}
}

func assertValidateOK(t *testing.T, cfg *Config) *Config {
	t.Helper()
	if err := Validate(cfg); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	return cfg
}

func assertValidateError(t *testing.T, cfg *Config, wantSubstring string) error {
	t.Helper()
	err := Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if wantSubstring != "" && !strings.Contains(err.Error(), wantSubstring) {
		t.Fatalf("expected error containing %q, got %v", wantSubstring, err)
	}
	return err
}

func seenRule() *Rule {
	rule := validRule()
	rule.GroupBy = ""
	rule.Condition = ConditionSpec{Value: &condition.SeenCondition{}}
	rule.Action = ActionSpec{Value: &action.LogAction{Template: "rule={{ .Rule }}"}}
	return rule
}

func unmarshalConditionYAML(t *testing.T, yamlData string) ConditionSpec {
	t.Helper()
	var data ConditionSpec
	if err := yaml.Unmarshal([]byte(yamlData), &data); err != nil {
		t.Fatalf("unmarshal condition: %v", err)
	}
	return data
}

func unmarshalActionYAML(t *testing.T, yamlData string) ActionSpec {
	t.Helper()
	var data ActionSpec
	if err := yaml.Unmarshal([]byte(yamlData), &data); err != nil {
		t.Fatalf("unmarshal action: %v", err)
	}
	return data
}

func assertRunOptions(t *testing.T, opts *action.RunOptions, wantTimeout uint, wantStopPrevious bool) {
	t.Helper()
	if *opts.Timeout != wantTimeout {
		t.Fatalf("Timeout should be %d, got %d", wantTimeout, *opts.Timeout)
	}
	if *opts.StopPrevious != wantStopPrevious {
		t.Fatalf("StopPrevious should be %t, got %t", wantStopPrevious, *opts.StopPrevious)
	}
}

func tempExecutableScript(t *testing.T) string {
	t.Helper()
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write script: %v", err)
	}
	return script
}

func testdataPath(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join("testdata", name)
}

func TestConfigFileNotFound(t *testing.T) {
	_, err := LoadFromFile("/tmp/this-file-does-not-exist.yaml")
	if !strings.Contains(err.Error(), "failed to read") {
		t.Fatalf("expected 'failed to read', got %v", err)
	}
}

func TestConfigFileValid(t *testing.T) {
	_, err := LoadFromFile(testdataPath(t, "valid.yaml"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConfigFileInvalidYAML(t *testing.T) {
	_, err := LoadFromFile(testdataPath(t, "invalid-syntax.yaml"))
	if !strings.Contains(err.Error(), "failed to parse YAML data") {
		t.Fatalf("expected 'failed to parse YAML data', got %v", err)
	}
}

func TestConfigFileInvalidData(t *testing.T) {
	_, err := LoadFromFile(testdataPath(t, "invalid-data.yaml"))
	if !strings.Contains(err.Error(), "is required") {
		t.Fatalf("expected 'is required', got %v", err)
	}
}

func TestEmptyRules(t *testing.T) {
	err := Validate(configWithRules())
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "no rules found" {
		t.Fatalf("expected 'no rules found', got %v", err)
	}
}

func TestConfigMissingRulesKey(t *testing.T) {
	cfg, err := LoadFromString([]byte("foo: bar"))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	err = Validate(cfg)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err.Error() != "no rules found" {
		t.Fatalf("expected 'no rules found', got %v", err)
	}
}

func TestRuleValid(t *testing.T) {
	assertValidateOK(t, configWithRules(validRule()))
}

func TestRuleInvalidCooldown(t *testing.T) {
	_, err := LoadFromString([]byte(`rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout'
    cooldown: -1
`))
	if !strings.Contains(err.Error(), "cannot unmarshal") {
		t.Fatalf("expected 'cannot unmarshal', got %v", err)
	}
}

func TestRuleMissingName(t *testing.T) {
	rule := validRule()
	rule.Name = ""
	_ = assertValidateError(t, configWithRules(rule), "`name` is required")
}

func TestRuleMissingFile(t *testing.T) {
	rule := validRule()
	rule.File = ""
	_ = assertValidateError(t, configWithRules(rule), "`file` is required")
}

func TestRuleInvalidFile(t *testing.T) {
	tmpDir := t.TempDir()
	invalidPath := filepath.Join(tmpDir, "invalid/path/error.log")
	invalidFolder := filepath.Join(tmpDir, "invalid/path")

	rule := validRule()
	rule.File = invalidPath
	expectedError := "folder " + invalidFolder + " does not exist (parent of " + invalidPath + ")"
	err := assertValidateError(t, configWithRules(rule), expectedError)
	if !strings.Contains(err.Error(), expectedError) {
		t.Fatalf("expected %q, got %v", expectedError, err)
	}
}

func TestRuleMissingPattern(t *testing.T) {
	rule := validRule()
	rule.Pattern = ""
	_ = assertValidateError(t, configWithRules(rule), "`pattern` is required")
}

func TestRuleInvalidPattern(t *testing.T) {
	rule := validRule()
	rule.Pattern = `timeout: (?P<backend>.*`
	_ = assertValidateError(t, configWithRules(rule), "failed to compile pattern")
}

func TestRuleGroupByNotInPattern(t *testing.T) {
	rule := validRule()
	rule.Pattern = `timeout: (?P<host>[0-9a-z.]+):(?P<timeout>\d+)`
	_ = assertValidateError(t, configWithRules(rule), "group by backend is not in pattern")
}

func TestRuleGroupByEmpty(t *testing.T) {
	rule := validRule()
	rule.GroupBy = ""
	cfg := assertValidateOK(t, configWithRules(rule))
	if cfg.Rules[0].GroupBy != "" {
		t.Fatalf("expected empty group_by, got %q", cfg.Rules[0].GroupBy)
	}
}

func TestRuleConditionMissing(t *testing.T) {
	rule := validRule()
	rule.Condition = ConditionSpec{}
	_ = assertValidateError(t, configWithRules(rule), "`condition` is required")
}

func TestRuleInvalidConditionType(t *testing.T) {
	_, err := LoadFromString([]byte(`rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout'
    condition:
      type: invalid
    action:
      type: log
      template: "fake template"
`))
	if !strings.Contains(err.Error(), `condition: unknown type "invalid"`) {
		t.Fatalf("expected unknown condition type error, got %v", err)
	}
}

func TestRuleActionMissing(t *testing.T) {
	rule := validRule()
	rule.Action = ActionSpec{}
	_ = assertValidateError(t, configWithRules(rule), "`action` is required")
}

func TestRuleInvalidActionType(t *testing.T) {
	_, err := LoadFromString([]byte(`rules:
  - name: upstream_timeout
    file: /tmp/error.log
    pattern: 'timeout'
    condition:
      type: threshold
      threshold: 5
      window: 60
    action:
      type: invalid
`))
	if !strings.Contains(err.Error(), `action: unknown type "invalid"`) {
		t.Fatalf("expected unknown action type error, got %v", err)
	}
}

func TestConditionSpecSeenValid(t *testing.T) {
	data := unmarshalConditionYAML(t, `type: seen`)
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionSpecThresholdValid(t *testing.T) {
	data := unmarshalConditionYAML(t, `type: threshold
threshold: 5
window: 60
`)
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionSpecAbsenceValid(t *testing.T) {
	data := unmarshalConditionYAML(t, `type: absence
duration: 60
`)
	if err := data.Value.Validate(); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestConditionSpecUnknownType(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: invalid`), new(ConditionSpec))
	if !strings.Contains(err.Error(), `condition: unknown type "invalid"`) {
		t.Fatalf("expected unknown condition type error, got %v", err)
	}
}

func TestConditionSpecThresholdInvalid(t *testing.T) {
	c := &condition.ThresholdCondition{Threshold: 0, Window: 60}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`threshold` must be greater than 0") {
		t.Fatalf("expected threshold validation error, got %v", err)
	}
}

func TestConditionSpecThresholdInvalidWindow(t *testing.T) {
	c := &condition.ThresholdCondition{Threshold: 5, Window: 0}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`window` must be greater than 0") {
		t.Fatalf("expected window validation error, got %v", err)
	}
}

func TestConditionSpecAbsenceInvalid(t *testing.T) {
	c := &condition.AbsenceCondition{Duration: 0}
	if err := c.Validate(); err == nil {
		t.Fatal("expected validation error, got nil")
	} else if !strings.Contains(err.Error(), "`duration` must be defined and greater than 0") {
		t.Fatalf("expected duration validation error, got %v", err)
	}
}

func TestConditionSpecMissingType(t *testing.T) {
	err := yaml.Unmarshal([]byte(`threshold: 5
window: 60
`), new(ConditionSpec))
	if !strings.Contains(err.Error(), `condition: unknown type ""`) {
		t.Fatalf("expected missing condition type error, got %v", err)
	}
}

func TestConditionSpecMalformedThreshold(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: threshold
threshold: not-a-number
window: 60
`), new(ConditionSpec))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestConditionSpecMalformedAbsence(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: absence
duration: not-a-number
`), new(ConditionSpec))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestActionSpecLogValid(t *testing.T) {
	tests := []struct {
		name             string
		yaml             string
		wantTimeout      uint
		wantStopPrevious bool
	}{
		{
			name: "defaults",
			yaml: `type: log
template: "rule={{ .Rule }}"
`,
			wantTimeout:      0,
			wantStopPrevious: false,
		},
		{
			name: "explicit values",
			yaml: `type: log
template: "rule={{ .Rule }}"
timeout: 10
stop_previous: true
`,
			wantTimeout:      10,
			wantStopPrevious: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := unmarshalActionYAML(t, tt.yaml)
			if err := data.Value.Validate(); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			assertRunOptions(t, &data.Value.(*action.LogAction).Options, tt.wantTimeout, tt.wantStopPrevious)
		})
	}
}

func TestActionSpecShellValid(t *testing.T) {
	script := tempExecutableScript(t)
	tests := []struct {
		name             string
		extraYAML        string
		wantTimeout      uint
		wantStopPrevious bool
	}{
		{
			name:             "defaults",
			wantTimeout:      0,
			wantStopPrevious: false,
		},
		{
			name: "explicit values",
			extraYAML: `
timeout: 10
stop_previous: true`,
			wantTimeout:      10,
			wantStopPrevious: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := unmarshalActionYAML(t, `type: shell
script: `+script+tt.extraYAML+`
`)
			if err := data.Value.Validate(); err != nil {
				t.Fatalf("expected no error, got %v", err)
			}
			assertRunOptions(t, &data.Value.(*action.ShellAction).Options, tt.wantTimeout, tt.wantStopPrevious)
		})
	}
}

func TestActionSpecUnknownType(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: invalid`), new(ActionSpec))
	if !strings.Contains(err.Error(), `action: unknown type "invalid"`) {
		t.Fatalf("expected unknown action type error, got %v", err)
	}
}

func TestActionSpecMissingType(t *testing.T) {
	err := yaml.Unmarshal([]byte(`template: "hello"`), new(ActionSpec))
	if !strings.Contains(err.Error(), `action: unknown type ""`) {
		t.Fatalf("expected missing action type error, got %v", err)
	}
}

func TestActionSpecMalformedLog(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: log
template:
  invalid: mapping
`), new(ActionSpec))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestActionSpecMalformedShell(t *testing.T) {
	err := yaml.Unmarshal([]byte(`type: shell
script:
  invalid: mapping
`), new(ActionSpec))
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestStringMethods(t *testing.T) {
	var nilAction ActionSpec
	if nilAction.String() != "<nil>" {
		t.Fatalf("expected '<nil>', got %q", nilAction.String())
	}

	var nilCondition ConditionSpec
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

	condition := unmarshalConditionYAML(t, `type: seen`)
	if condition.String() != "seen()" {
		t.Fatalf("expected 'seen()', got %q", condition.String())
	}

	action := unmarshalActionYAML(t, `type: log
template: "hello"
`)
	if err := action.Value.Validate(); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(action.String(), "log(") {
		t.Fatalf("expected log action string, got %q", action.String())
	}
}

func TestRuleValidSeenCondition(t *testing.T) {
	rule := seenRule()
	rule.Name = "connection_failed"
	rule.Pattern = "connect failed"
	assertValidateOK(t, configWithRules(rule))
}

func TestRuleValidAbsenceCondition(t *testing.T) {
	rule := seenRule()
	rule.Name = "heartbeat_missing"
	rule.Pattern = "heartbeat ok"
	rule.Condition = ConditionSpec{Value: &condition.AbsenceCondition{Duration: 10}}
	assertValidateOK(t, configWithRules(rule))
}

func TestRuleValidShellAction(t *testing.T) {
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rule := seenRule()
	rule.Name = "alert"
	rule.Pattern = "error"
	rule.Action = ActionSpec{Value: &action.ShellAction{Script: script}}
	assertValidateOK(t, configWithRules(rule))
}

func TestRuleFilePathAbsolute(t *testing.T) {
	rule := seenRule()
	rule.Name = "test_rule"
	rule.File = "error.log"
	cfg := assertValidateOK(t, configWithRules(rule))
	if !filepath.IsAbs(cfg.Rules[0].File) {
		t.Fatalf("expected absolute file path, got %q", cfg.Rules[0].File)
	}
}

func TestValidateSetsPatternRegexp(t *testing.T) {
	cfg := assertValidateOK(t, configWithRules(validRule()))

	if cfg.Rules[0].PatternRegexp == nil {
		t.Fatal("expected PatternRegexp to be set, got nil")
	}
	if cfg.Rules[0].PatternRegexp.String() != testPatternWithBackend {
		t.Fatalf("expected pattern %q, got %q", testPatternWithBackend, cfg.Rules[0].PatternRegexp.String())
	}
	if !cfg.Rules[0].PatternRegexp.MatchString("timeout: api.example.com:30") {
		t.Fatal("expected compiled pattern to match sample line")
	}
}

func TestRuleInvalidThresholdCondition(t *testing.T) {
	rule := seenRule()
	rule.Pattern = "timeout"
	rule.Condition = ConditionSpec{Value: &condition.ThresholdCondition{Threshold: 0, Window: 60}}
	_ = assertValidateError(t, configWithRules(rule), "`threshold` must be greater than 0")
}

func TestRuleInvalidThresholdWindow(t *testing.T) {
	rule := seenRule()
	rule.Pattern = "timeout"
	rule.Condition = ConditionSpec{Value: &condition.ThresholdCondition{Threshold: 5, Window: 0}}
	_ = assertValidateError(t, configWithRules(rule), "`window` must be greater than 0")
}

func TestRuleInvalidAbsenceCondition(t *testing.T) {
	rule := seenRule()
	rule.Name = "heartbeat_missing"
	rule.Pattern = "heartbeat ok"
	rule.Condition = ConditionSpec{Value: &condition.AbsenceCondition{Duration: 0}}
	_ = assertValidateError(t, configWithRules(rule), "`duration` must be defined and greater than 0")
}

func TestRuleInvalidLogTemplate(t *testing.T) {
	rule := seenRule()
	rule.Name = "test_rule"
	rule.Action = ActionSpec{Value: &action.LogAction{Template: "{{invalid"}}
	_ = assertValidateError(t, configWithRules(rule), "failed to parse log template")
}

func TestRuleInvalidShellActionMissingScript(t *testing.T) {
	rule := seenRule()
	rule.Name = "alert"
	rule.Action = ActionSpec{Value: &action.ShellAction{Script: "/tmp/does-not-exist.sh"}}
	_ = assertValidateError(t, configWithRules(rule), "does not exist")
}

func TestRuleInvalidShellActionNotExecutable(t *testing.T) {
	script := filepath.Join(t.TempDir(), "alert.sh")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"), 0644); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	rule := seenRule()
	rule.Name = "alert"
	rule.Action = ActionSpec{Value: &action.ShellAction{Script: script}}
	_ = assertValidateError(t, configWithRules(rule), "is not executable")
}

func TestRuleFileMissingOK(t *testing.T) {
	tmpDir := t.TempDir()
	missingFile := filepath.Join(tmpDir, "does-not-exist.log")

	rule := seenRule()
	rule.Name = "test_rule"
	rule.File = missingFile
	assertValidateOK(t, configWithRules(rule))
}

func TestValidateMultipleRulesSecondInvalid(t *testing.T) {
	valid := seenRule()
	valid.Name = "valid_rule"

	invalid := seenRule()
	invalid.Name = ""

	_ = assertValidateError(t, configWithRules(valid, invalid), "rule 1: `name` is required")
}

func TestRuleInvalidShellActionMissingScriptField(t *testing.T) {
	rule := seenRule()
	rule.Name = "alert"
	rule.Action = ActionSpec{Value: &action.ShellAction{}}
	_ = assertValidateError(t, configWithRules(rule), "`script` is required")
}
