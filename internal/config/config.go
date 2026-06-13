package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	yaml "gopkg.in/yaml.v3"

	"pavlov/internal/action"
	"pavlov/internal/condition"
)

type ActionConfig struct {
	Value action.Action
}

func (c *ActionConfig) String() string {
	if c.Value == nil {
		return "<nil>"
	}
	return fmt.Sprint(c.Value)
}

type ConditionConfig struct {
	Value condition.Condition
}

func (c *ConditionConfig) String() string {
	if c.Value == nil {
		return "<nil>"
	}
	return fmt.Sprint(c.Value)
}

func (r Rule) String() string {
	return fmt.Sprintf("{Name:%q File:%q Pattern:%q GroupBy:%q Condition:%s Action:%s}",
		r.Name, r.File, r.Pattern, r.GroupBy, r.Condition, r.Action)
}

type Rule struct {
	Name      string          `yaml:"name"`
	File      string          `yaml:"file"`
	Pattern   string          `yaml:"pattern"`
	GroupBy   string          `yaml:"group_by"`
	Cooldown  uint            `yaml:"cooldown"`
	Condition ConditionConfig `yaml:"condition"`
	Action    ActionConfig    `yaml:"action"`
}

type Config struct {
	Rules []*Rule `yaml:"rules"`
}

func (c *ActionConfig) UnmarshalYAML(value *yaml.Node) error {
	var discriminator struct {
		Type string `yaml:"type"`
	}
	if err := value.Decode(&discriminator); err != nil {
		return err
	}

	switch discriminator.Type {
	case "shell":
		v := &action.ShellAction{}
		if err := value.Decode(v); err != nil {
			return err
		}
		c.Value = v
	case "log":
		v := &action.LogAction{}
		if err := value.Decode(v); err != nil {
			return err
		}
		c.Value = v
	default:
		return fmt.Errorf("action: unknown type %q", discriminator.Type)
	}

	return nil
}

func (c *ConditionConfig) UnmarshalYAML(value *yaml.Node) error {
	var discriminator struct {
		Type string `yaml:"type"`
	}
	if err := value.Decode(&discriminator); err != nil {
		return err
	}

	switch discriminator.Type {
	case "seen":
		v := &condition.SeenCondition{}
		if err := value.Decode(v); err != nil {
			return err
		}
		c.Value = v
	case "threshold":
		v := &condition.ThresholdCondition{}
		if err := value.Decode(v); err != nil {
			return err
		}
		c.Value = v
	case "absence":
		v := &condition.AbsenceCondition{}
		if err := value.Decode(v); err != nil {
			return err
		}
		c.Value = v
	default:
		return fmt.Errorf("action: unknown type %q", discriminator.Type)
	}

	return nil
}

func validateRuleFile(rule *Rule) error {
	// Ensure the containing folder exists. We allow the file itself to be
	// missing on startup.
	abs, err := filepath.Abs(filepath.Clean(rule.File))
	if err != nil {
		return fmt.Errorf("failed to get absolute path of %s: %w", rule.File, err)
	}
	folder := filepath.Dir(abs)
	if _, err := os.Stat(folder); os.IsNotExist(err) {
		return fmt.Errorf("folder %s does not exist (parent of %s)", folder, rule.File)
	}
	return nil
}

func validateRuleGroupBy(rule *Rule) error {
	// Group by is optional, but if it is set, it must be in the pattern
	if rule.GroupBy == "" {
		return nil
	}
	pattern := fmt.Sprintf("(?P<%s>", rule.GroupBy)
	if !strings.Contains(rule.Pattern, pattern) {
		return fmt.Errorf("rule %s: group by %s is not in pattern %s", rule.Name, rule.GroupBy, rule.Pattern)
	}
	return nil
}

func Validate(config *Config) error {
	if len(config.Rules) == 0 {
		return fmt.Errorf("no rules found")
	}

	for idx, rule := range config.Rules {
		// Name is required
		if rule.Name == "" {
			return fmt.Errorf("rule %d: `name` is required", idx)
		}

		// File is required
		if rule.File == "" {
			return fmt.Errorf("rule %d: `file` is required", idx)
		}

		// Pattern is required
		if rule.Pattern == "" {
			return fmt.Errorf("rule %d: `pattern` is required", idx)
		}

		// Get absolute path of file
		file, err := filepath.Abs(filepath.Clean(rule.File))
		if err != nil {
			return fmt.Errorf("rule %d: failed to get absolute path of %s: %w", idx, rule.File, err)
		}
		rule.File = file

		err = validateRuleFile(rule)
		if err != nil {
			return fmt.Errorf("rule %s: %w", rule.Name, err)
		}

		err = validateRuleGroupBy(rule)
		if err != nil {
			return fmt.Errorf("rule %s: %w", rule.Name, err)
		}

		if rule.Condition.Value == nil {
			return fmt.Errorf("rule %s: `condition` is required", rule.Name)
		}

		err = rule.Condition.Value.Validate()
		if err != nil {
			return fmt.Errorf("rule %s: %w", rule.Name, err)
		}

		if rule.Action.Value == nil {
			return fmt.Errorf("rule %s: `action` is required", rule.Name)
		}

		err = rule.Action.Value.Validate()
		if err != nil {
			return fmt.Errorf("rule %s: %w", rule.Name, err)
		}
	}

	return nil
}

func LoadFromString(data []byte) (*Config, error) {
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func LoadFromFile(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	cfg, err := LoadFromString(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML data: %w", err)
	}

	err = Validate(cfg)
	if err != nil {
		return nil, err
	}

	slog.Info("config loaded", "file", filename)

	return cfg, nil
}
