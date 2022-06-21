package optimizer

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"sync"

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
	// Execute(configJson []byte) (*backtest.SummaryReport, error)
	Run(ctx context.Context, taskC chan BacktestTask) (chan BacktestTask, error)
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
		defer close(handle.Done)
		report, err := e.execute(configJson)
		handle.Error = err
		handle.Report = report
	}()

	return handle
}

func (e *LocalProcessExecutor) readReport(output []byte) (*backtest.SummaryReport, error) {
	summaryReportFilepath := strings.TrimSpace(string(output))
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

func (e *LocalProcessExecutor) Run(ctx context.Context, taskC chan BacktestTask) (chan BacktestTask, error) {
	var maxNumOfProcess = ctx.Value("MaxThread").(int)
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
			log.Infof("starting local worker #%d", id)
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
					log.Infof("local worker #%d received param task: %v", id, task.Params)

					report, err := e.execute(task.ConfigJson)
					if err != nil {
						log.WithError(err).Errorf("execute error")
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

// execute runs the config json and returns the summary report
// this is a blocking operation
func (e *LocalProcessExecutor) execute(configJson []byte) (*backtest.SummaryReport, error) {
	tf, err := jsonToYamlConfig(e.ConfigDir, configJson)
	if err != nil {
		return nil, err
	}

	c := exec.Command(e.Bin, "backtest", "--config", tf.Name(), "--output", e.OutputDir, "--subdir")
	output, err := c.Output()
	if err != nil {
		return nil, err
	}

	return e.readReport(output)
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
