package optimizer

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/backtest"
)

var log = logrus.WithField("component", "optimizer")

type Executor interface {
	Execute(configJson []byte) (*backtest.SummaryReport, error)
}

type AsyncHandle struct {
	Error  error
	Report *backtest.SummaryReport
	Done   chan struct{}
}

type LocalProcessExecutor struct {
	Bin       string
	WorkDir   string
	ConfigDir string
	OutputDir string
}

func (e *LocalProcessExecutor) ExecuteAsync(configJson []byte) *AsyncHandle {
	handle := &AsyncHandle{
		Done: make(chan struct{}),
	}

	go func() {
		report, err := e.Execute(configJson)
		handle.Error = err
		handle.Report = report
		close(handle.Done)
	}()

	return handle
}

func (e *LocalProcessExecutor) Execute(configJson []byte) (*backtest.SummaryReport, error) {
	tf, err := jsonToYamlConfig(e.ConfigDir, configJson)
	if err != nil {
		return nil, err
	}

	c := exec.Command(e.Bin, "backtest", "--config", tf.Name(), "--output", e.OutputDir, "--subdir")
	output, err := c.Output()
	if err != nil {
		return nil, err
	}

	summaryReportFilepath := strings.TrimSpace(string(output))
	_, err = os.Stat(summaryReportFilepath)
	if os.IsNotExist(err) {
		return nil, err
	}

	summaryReport, err := backtest.ReadSummaryReport(summaryReportFilepath)
	if err != nil {
		return nil, err
	}

	return summaryReport, nil
}

func jsonToYamlConfig(dir string, configJson []byte) (*os.File, error) {
	var o map[string]interface{}
	if err := json.Unmarshal(configJson, &o); err != nil {
		return nil, err
	}

	yamlConfig, err := yaml.Marshal(o)
	if err != nil {
		return nil, err
	}

	tf, err := os.CreateTemp(dir, "bbgo-*.yaml")
	if err != nil {
		return nil, err
	}

	if _, err = tf.Write(yamlConfig); err != nil {
		return nil, err
	}

	if err := tf.Close(); err != nil {
		return nil, err
	}

	return tf, nil
}
