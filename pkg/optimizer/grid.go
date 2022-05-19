package optimizer

import (
	"fmt"

	"github.com/evanphx/json-patch/v5"

	"github.com/c9s/bbgo/pkg/fixedpoint"
)

type GridOptimizer struct {
	Config *Config
}

func (o *GridOptimizer) buildOps() []OpFunc {
	var ops []OpFunc
	for _, selector := range o.Config.Matrix {
		var path = selector.Path

		switch selector.Type {
		case "range":
			min := selector.Min
			max := selector.Max
			step := selector.Step
			if step.IsZero() {
				step = fixedpoint.One
			}

			f := func(configJson []byte, next func(configJson []byte) error) error {
				var values []fixedpoint.Value
				for val := min; val.Compare(max) < 0; val = val.Add(step) {
					values = append(values, val)
				}

				log.Debugf("ranged values: %v", values)
				for _, val := range values {
					jsonOp := []byte(fmt.Sprintf(`[ {"op": "replace", "path": "%s", "value": %v } ]`, path, val))
					patch, err := jsonpatch.DecodePatch(jsonOp)
					if err != nil {
						return err
					}

					log.Debugf("json op: %s", jsonOp)

					configJson, err := patch.ApplyIndent(configJson, "  ")
					if err != nil {
						return err
					}

					if err := next(configJson); err != nil {
						return err
					}
				}

				return nil
			}
			ops = append(ops, f)

		case "iterate":
			values := selector.Values
			f := func(configJson []byte, next func(configJson []byte) error) error {
				log.Debugf("iterate values: %v", values)
				for _, val := range values {
					jsonOp := []byte(fmt.Sprintf(`[{"op": "replace", "path": "%s", "value": "%s"}]`, path, val))
					patch, err := jsonpatch.DecodePatch(jsonOp)
					if err != nil {
						return err
					}

					log.Debugf("json op: %s", jsonOp)

					configJson, err := patch.ApplyIndent(configJson, "  ")
					if err != nil {
						return err
					}

					if err := next(configJson); err != nil {
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
	var ops = o.buildOps()
	var last = func(configJson []byte, next func(configJson []byte) error) error {
		return executor.Execute(configJson)
	}
	ops = append(ops, last)

	var wrapper = func(configJson []byte) error { return nil }
	for i := len(ops) - 1; i > 0; i-- {
		next := ops[i]
		cur := ops[i-1]
		inner := wrapper
		wrapper = func(configJson []byte) error {
			return cur(configJson, func(configJson []byte) error {
				return next(configJson, inner)
			})
		}
	}

	return wrapper(configJson)
}
