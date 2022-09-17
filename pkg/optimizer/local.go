package optimizer

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/cheggaaa/pb/v3"
	"github.com/pkg/errors"

	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/backtest"
)

var log = logrus.WithField("component", "optimizer")

type BacktestTask struct {
	ConfigJson []byte
	Params     []interface{}
	Labels     []string
	Report     *backtest.SummaryReport
	Error      error
}

type Executor interface {
	Execute(configJson []byte) (*backtest.SummaryReport, error)
	Run(ctx context.Context, taskC chan BacktestTask, bar *pb.ProgressBar) (chan BacktestTask, error)
}

type AsyncHandle struct {
	Error  error
	Report *backtest.SummaryReport
	Done   chan struct{}
}

type LocalProcessExecutor struct {
	Config    *LocalExecutorConfig
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
		defer close(handle.Done)
		report, err := e.Execute(configJson)
		handle.Error = err
		handle.Report = report
	}()

	return handle
}

func (e *LocalProcessExecutor) readReport(reportPath string) (*backtest.SummaryReport, error) {
	summaryReportFilepath := strings.TrimSpace(reportPath)
	_, err := os.Stat(summaryReportFilepath)
	if os.IsNotExist(err) {
		return nil, err
	}

	summaryReport, err := backtest.ReadSummaryReport(summaryReportFilepath)
	if err != nil {
		return nil, err
	}

	return summaryReport, nil
}

// Prepare prepares the environment for the following back tests
// this is a blocking operation
func (e *LocalProcessExecutor) Prepare(configJson []byte) error {
	log.Debugln("syncing backtest data before starting backtests...")
	tf, err := jsonToYamlConfig(e.ConfigDir, configJson)
	if err != nil {
		return err
	}

	c := exec.Command(e.Bin, "backtest", "--sync", "--sync-only", "--config", tf.Name())
	output, err := c.Output()
	if err != nil {
		return errors.Wrapf(err, "failed to sync backtest data: %s", string(output))
	}

	return nil
}

func (e *LocalProcessExecutor) Run(ctx context.Context, taskC chan BacktestTask, bar *pb.ProgressBar) (chan BacktestTask, error) {
	var maxNumOfProcess = e.Config.MaxNumberOfProcesses
	var resultsC = make(chan BacktestTask, maxNumOfProcess*2)

	wg := sync.WaitGroup{}
	wg.Add(maxNumOfProcess)

	go func() {
		wg.Wait()
		close(resultsC)
	}()

	for i := 0; i < maxNumOfProcess; i++ {
		// fork workers
		go func(id int, taskC chan BacktestTask) {
			taskCnt := 0
			bar.Set("log", fmt.Sprintf("starting local worker #%d", id))
			bar.Write()
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return

				case task, ok := <-taskC:
					if !ok {
						return
					}

					taskCnt++
					bar.Set("log", fmt.Sprintf("local worker #%d received param task: %v", id, task.Params))
					bar.Write()

					report, err := e.Execute(task.ConfigJson)
					if err != nil {
						if err2, ok := err.(*exec.ExitError); ok {
							log.WithError(err).Errorf("execute error: %s", err2.Stderr)
						} else {
							log.WithError(err).Errorf("execute error")
						}
					}

					task.Error = err
					task.Report = report

					resultsC <- task
				}
			}
		}(i+1, taskC)
	}

	return resultsC, nil
}

// Execute runs the config json and returns the summary report. This is a blocking operation.
func (e *LocalProcessExecutor) Execute(configJson []byte) (*backtest.SummaryReport, error) {
	tf, err := jsonToYamlConfig(e.ConfigDir, configJson)
	if err != nil {
		return nil, err
	}

	c := exec.Command(e.Bin, "backtest", "--config", tf.Name(), "--output", e.OutputDir, "--subdir")
	output, err := c.Output()
	if err != nil {
		log.WithError(err).WithField("command", []string{e.Bin, "backtest", "--config", tf.Name(), "--output", e.OutputDir, "--subdir"}).Errorf("failed to execute backtest")
		return nil, err
	}

	// the last line is the report path
	scanner := bufio.NewScanner(bytes.NewBuffer(output))
	var reportFilePath string
	for scanner.Scan() {
		reportFilePath = scanner.Text()
	}
	return e.readReport(reportFilePath)
}

// jsonToYamlConfig translate json format config into a YAML format config file
// The generated file is a temp file
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
