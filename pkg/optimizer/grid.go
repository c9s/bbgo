package optimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/evanphx/json-patch/v5"

	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MetricValueFunc func(summaryReport *backtest.SummaryReport) fixedpoint.Value

var TotalProfitMetricValueFunc = func(summaryReport *backtest.SummaryReport) fixedpoint.Value {
	return summaryReport.TotalProfit
}

type Metric struct {
	Labels []string         `json:"labels,omitempty"`
	Params []interface{}    `json:"params,omitempty"`
	Value  fixedpoint.Value `json:"value,omitempty"`
}

func copyParams(params []interface{}) []interface{} {
	var c = make([]interface{}, len(params))
	copy(c, params)
	return c
}

func copyLabels(labels []string) []string {
	var c = make([]string, len(labels))
	copy(c, labels)
	return c
}

type GridOptimizer struct {
	Config *Config

	ParamLabels   []string
	CurrentParams []interface{}
}

func (o *GridOptimizer) buildOps() []OpFunc {
	var ops []OpFunc

	o.CurrentParams = make([]interface{}, len(o.Config.Matrix))
	o.ParamLabels = make([]string, len(o.Config.Matrix))

	for i, selector := range o.Config.Matrix {
		var path = selector.Path
		var ii = i // copy variable because we need to use them in the closure

		if selector.Label != "" {
			o.ParamLabels[ii] = selector.Label
		} else {
			o.ParamLabels[ii] = selector.Path
		}

		switch selector.Type {
		case "range":
			min := selector.Min
			max := selector.Max
			step := selector.Step
			if step.IsZero() {
				step = fixedpoint.One
			}

			var values []fixedpoint.Value
			for val := min; val.Compare(max) <= 0; val = val.Add(step) {
				values = append(values, val)
			}

			f := func(configJson []byte, next func(configJson []byte) error) error {
				for _, val := range values {
					jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v }]`, path, val)))
					patch, err := jsonpatch.DecodePatch(jsonOp)
					if err != nil {
						return err
					}

					log.Debugf("json op: %s", jsonOp)

					patchedJson, err := patch.ApplyIndent(configJson, "  ")
					if err != nil {
						return err
					}

					valCopy := val
					o.CurrentParams[ii] = valCopy
					if err := next(patchedJson); err != nil {
						return err
					}
				}

				return nil
			}
			ops = append(ops, f)

		case "iterate":
			values := selector.Values
			f := func(configJson []byte, next func(configJson []byte) error) error {
				for _, val := range values {
					log.Debugf("%d %s: %v of %v", ii, path, val, values)

					jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": "%s"}]`, path, val)))
					patch, err := jsonpatch.DecodePatch(jsonOp)
					if err != nil {
						return err
					}

					log.Debugf("json op: %s", jsonOp)

					patchedJson, err := patch.ApplyIndent(configJson, "  ")
					if err != nil {
						return err
					}

					valCopy := val
					o.CurrentParams[ii] = valCopy
					if err := next(patchedJson); err != nil {
						return err
					}
				}

				return nil
			}
			ops = append(ops, f)
		case "bool":
			values := []bool{true, false}
			f := func(configJson []byte, next func(configJson []byte) error) error {
				for _, val := range values {
					log.Debugf("%d %s: %v of %v", ii, path, val, values)

					jsonOp := []byte(reformatJson(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": %v}]`, path, val)))
					patch, err := jsonpatch.DecodePatch(jsonOp)
					if err != nil {
						return err
					}

					log.Debugf("json op: %s", jsonOp)

					patchedJson, err := patch.ApplyIndent(configJson, "  ")
					if err != nil {
						return err
					}

					valCopy := val
					o.CurrentParams[ii] = valCopy
					if err := next(patchedJson); err != nil {
						return err
					}
				}

				return nil
			}
			ops = append(ops, f)
		}
	}
	return ops
}

func (o *GridOptimizer) Run(executor Executor, configJson []byte) (map[string][]Metric, error) {
	o.CurrentParams = make([]interface{}, len(o.Config.Matrix))

	var valueFunctions = map[string]MetricValueFunc{
		"totalProfit": TotalProfitMetricValueFunc,
	}
	var metrics = map[string][]Metric{}

	var ops = o.buildOps()

	var taskC = make(chan BacktestTask, 100)

	var app = func(configJson []byte, next func(configJson []byte) error) error {
		var labels = copyLabels(o.ParamLabels)
		var params = copyParams(o.CurrentParams)
		taskC <- BacktestTask{
			ConfigJson: configJson,
			Params:     params,
			Labels:     labels,
		}
		return nil
	}

	log.Debugf("build %d ops", len(ops))

	var wrapper = func(configJson []byte) error {
		return app(configJson, nil)
	}

	for i := len(ops) - 1; i >= 0; i-- {
		cur := ops[i]
		inner := wrapper
		wrapper = func(configJson []byte) error {
			return cur(configJson, inner)
		}
	}

	resultsC, err := executor.Run(context.Background(), taskC)
	if err != nil {
		return nil, err
	}

	if err := wrapper(configJson); err != nil {
		return nil, err
	}
	close(taskC) // this will shut down the executor

	for result := range resultsC {
		for metricName, metricFunc := range valueFunctions {
			var metricValue = metricFunc(result.Report)
			log.Infof("params: %+v => %s %+v", result.Params, metricName, metricValue)
			metrics[metricName] = append(metrics[metricName], Metric{
				Params: result.Params,
				Labels: result.Labels,
				Value:  metricValue,
			})
		}
	}

	for n := range metrics {
		sort.Slice(metrics[n], func(i, j int) bool {
			a := metrics[n][i].Value
			b := metrics[n][j].Value
			return a.Compare(b) > 0
		})
	}

	return metrics, err
}

func reformatJson(text string) string {
	var a interface{}
	var err = json.Unmarshal([]byte(text), &a)
	if err != nil {
		return "{invalid json}"
	}

	out, _ := json.MarshalIndent(a, "", "  ")
	return string(out)
}
