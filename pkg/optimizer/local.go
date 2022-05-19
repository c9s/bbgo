package optimizer

import (
	"encoding/json"
	"os"
	"os/exec"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var log = logrus.WithField("component", "optimizer")

type Executor interface {
	Execute(configJson []byte) error
}

type LocalProcessExecutor struct {
	Bin           string
	WorkDir       string
	ConfigDir     string
	OutputDir     string
	CombineOutput bool
}

func (e *LocalProcessExecutor) Execute(configJson []byte) error {
	var o map[string]interface{}
	if err := json.Unmarshal(configJson, &o); err != nil {
		return err
	}

	yamlConfig, err := yaml.Marshal(o)
	if err != nil {
		return err
	}

	tf, err := os.CreateTemp(e.ConfigDir, "bbgo-*.yaml")
	if err != nil {
		return err
	}

	if _, err = tf.Write(yamlConfig); err != nil {
		return err
	}

	c := exec.Command(e.Bin, "backtest", "--config", tf.Name(), "--output", e.OutputDir, "--subdir")
	log.Infof("cmd: %+v", c)

	if e.CombineOutput {
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
	}

	return c.Run()
}
