package optimizer

import (
	"io/ioutil"

	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type SelectorConfig struct {
	Type   string           `json:"type" yaml:"type"`
	Label  string           `json:"label,omitempty" yaml:"label,omitempty"`
	Path   string           `json:"path" yaml:"path"`
	Values []string         `json:"values,omitempty" yaml:"values,omitempty"`
	Min    fixedpoint.Value `json:"min,omitempty" yaml:"min,omitempty"`
	Max    fixedpoint.Value `json:"max,omitempty" yaml:"max,omitempty"`
	Step   fixedpoint.Value `json:"step,omitempty" yaml:"step,omitempty"`
}

type Config struct {
	Matrix []SelectorConfig `yaml:"matrix"`
}

func LoadConfig(yamlConfigFileName string) (*Config, error) {
	configYaml, err := ioutil.ReadFile(yamlConfigFileName)
	if err != nil {
		return nil, err
	}

	var optConfig Config
	if err := yaml.Unmarshal(configYaml, &optConfig); err != nil {
		return nil, err
	}

	return &optConfig, nil
}
