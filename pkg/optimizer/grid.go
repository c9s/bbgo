package optimizer

import (
	"fmt"

	"github.com/evanphx/json-patch/v5"

	"github.com/c9s/bbgo/pkg/backtest"
	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type MetricValueFunc func(summaryReport *backtest.SummaryReport) fixedpoint.Value

var TotalProfitMetricValueFunc = func(summaryReport *backtest.SummaryReport) fixedpoint.Value {
	return summaryReport.TotalProfit
}

type Metric struct {
	Params []interface{}
	Value  fixedpoint.Value
}

type GridOptimizer struct {
	Config *Config

	CurrentParams []interface{}

	Metrics []Metric
}

func (o *GridOptimizer) buildOps() []OpFunc {
	var ops []OpFunc

	o.CurrentParams = make([]interface{}, len(o.Config.Matrix))

	for i, selector := range o.Config.Matrix {
		var path = selector.Path
		var ii = i // copy variable because we need to use them in the closure

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
					jsonOp := []byte(fmt.Sprintf(`[ {"op": "replace", "path": "%s", "value": %v } ]`, path, val))
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

					jsonOp := []byte(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": "%s"}]`, path, val))
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

func (o *GridOptimizer) Run(executor Executor, configJson []byte) error {
	o.CurrentParams = make([]interface{}, len(o.Config.Matrix))

	var ops = o.buildOps()
	var app = func(configJson []byte, next func(configJson []byte) error) error {
		summaryReport, err := executor.Execute(configJson)
		if err != nil {
			return err
		}

		// TODO: Add other metric value function
		metricValue := TotalProfitMetricValueFunc(summaryReport)

		o.Metrics = append(o.Metrics, Metric{
			Params: o.CurrentParams,
			Value:  metricValue,
		})

		log.Infof("current params: %+v => %+v", o.CurrentParams, metricValue)
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

	return wrapper(configJson)
}
