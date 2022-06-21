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

type LocalExecutorConfig struct {
	MaxNumberOfProcesses int `json:"maxNumberOfProcesses" yaml:"maxNumberOfProcesses"`
}

type ExecutorConfig struct {
	Type                string               `json:"type" yaml:"type"`
	LocalExecutorConfig *LocalExecutorConfig `json:"local" yaml:"local"`
}

type Config struct {
	Executor  *ExecutorConfig  `json:"executor" yaml:"executor"`
	MaxThread int              `yaml:"maxThread,omitempty"`
	Matrix    []SelectorConfig `yaml:"matrix"`
}

var defaultExecutorConfig = &ExecutorConfig{
	Type:                "local",
	LocalExecutorConfig: defaultLocalExecutorConfig,
}

var defaultLocalExecutorConfig = &LocalExecutorConfig{
	MaxNumberOfProcesses: 10,
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

	if optConfig.Executor == nil {
		optConfig.Executor = defaultExecutorConfig
	}

	if optConfig.Executor.Type == "" {
		optConfig.Executor.Type = "local"
	}

	if optConfig.Executor.Type == "local" && optConfig.Executor.LocalExecutorConfig == nil {
		optConfig.Executor.LocalExecutorConfig = defaultLocalExecutorConfig
	}

	return &optConfig, nil
}
