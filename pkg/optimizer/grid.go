package optimizer

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/cheggaaa/pb/v3"

	jsonpatch "github.com/evanphx/json-patch/v5"

	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MetricValueFunc func(summaryReport *backtest.SummaryReport) fixedpoint.Value

var TotalProfitMetricValueFunc = func(summaryReport *backtest.SummaryReport) fixedpoint.Value {
	return summaryReport.TotalProfit
}

var TotalVolume = func(summaryReport *backtest.SummaryReport) fixedpoint.Value {
	if len(summaryReport.SymbolReports) == 0 {
		return fixedpoint.Zero
	}

	buyVolume := summaryReport.SymbolReports[0].PnL.BuyVolume
	sellVolume := summaryReport.SymbolReports[0].PnL.SellVolume
	return buyVolume.Add(sellVolume)
}

var TotalEquityDiff = func(summaryReport *backtest.SummaryReport) fixedpoint.Value {
	if len(summaryReport.SymbolReports) == 0 {
		return fixedpoint.Zero
	}

	initEquity := summaryReport.InitialEquityValue
	finalEquity := summaryReport.FinalEquityValue
	return finalEquity.Sub(initEquity)
}

type Metric struct {
	// Labels is the labels of the given parameters
	Labels []string `json:"labels,omitempty"`

	// Params is the parameters used to output the metrics result
	Params []interface{} `json:"params,omitempty"`

	// Key is the metric name
	Key string `json:"key"`

	// Value is the metric value of the metric
	Value fixedpoint.Value `json:"value,omitempty"`
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
		case selectorTypeRange, selectorTypeRangeFloat, selectorTypeRangeInt:
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

		case selectorTypeIterate, selectorTypeString:
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
		case selectorTypeBool:
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
		"totalProfit":     TotalProfitMetricValueFunc,
		"totalVolume":     TotalVolume,
		"totalEquityDiff": TotalEquityDiff,
	}
	var metrics = map[string][]Metric{}

	var ops = o.buildOps()

	var taskC = make(chan BacktestTask, 10000)

	var taskCnt = 0
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
	var appCnt = func(configJson []byte, next func(configJson []byte) error) error {
		taskCnt++
		return nil
	}

	log.Debugf("build %d ops", len(ops))

	var wrapper = func(configJson []byte) error {
		return app(configJson, nil)
	}
	var wrapperCnt = func(configJson []byte) error {
		return appCnt(configJson, nil)
	}

	for i := len(ops) - 1; i >= 0; i-- {
		cur := ops[i]
		inner := wrapper
		innerCnt := wrapperCnt
		wrapper = func(configJson []byte) error {
			return cur(configJson, inner)
		}
		wrapperCnt = func(configJson []byte) error {
			return cur(configJson, innerCnt)
		}
	}

	if err := wrapperCnt(configJson); err != nil {
		return nil, err
	}
	var bar = pb.Full.New(taskCnt)
	bar.SetTemplateString(`{{ string . "log" | green}} | {{counters . }} {{bar . }} {{percent . }} {{etime . }} {{rtime . "ETA %s"}}`)

	ctx := context.Background()
	var taskGenErr error
	go func() {
		taskGenErr = wrapper(configJson)
		close(taskC) // this will shut down the executor
	}()

	resultsC, err := executor.Run(ctx, taskC, bar)
	if err != nil {
		return nil, err
	}

	for result := range resultsC {
		bar.Increment()

		if result.Report == nil {
			log.Errorf("no summaryReport found for params: %+v", result.Params)
			continue
		}

		for metricKey, metricFunc := range valueFunctions {
			var metricValue = metricFunc(result.Report)
			bar.Set("log", fmt.Sprintf("params: %+v => %s %+v", result.Params, metricKey, metricValue))

			metrics[metricKey] = append(metrics[metricKey], Metric{
				Params: result.Params,
				Labels: result.Labels,
				Key:    metricKey,
				Value:  metricValue,
			})
		}
	}
	bar.Finish()

	for n := range metrics {
		sort.Slice(metrics[n], func(i, j int) bool {
			a := metrics[n][i].Value
			b := metrics[n][j].Value
			return a.Compare(b) > 0
		})
	}

	if taskGenErr != nil {
		return metrics, taskGenErr
	} else {
		return metrics, err
	}
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
