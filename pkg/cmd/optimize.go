package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	jsonpatch "github.com/evanphx/json-patch/v5"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/optimizer"
)

func init() {
	optimizeCmd.Flags().String("optimizer-config", "optimizer.yaml", "config file")
	RootCmd.AddCommand(optimizeCmd)
}

var optimizeCmd = &cobra.Command{
	Use:   "optimize",
	Short: "run optimizer",

	// SilenceUsage is an option to silence usage when an error occurs.
	SilenceUsage: true,

	RunE: func(cmd *cobra.Command, args []string) error {
		optimizerConfigFilename, err := cmd.Flags().GetString("optimizer-config")
		if err != nil {
			return err
		}

		configFile, err := cmd.Flags().GetString("config")
		if err != nil {
			return err
		}

		yamlBody, err := ioutil.ReadFile(configFile)
		if err != nil {
			return err
		}
		var obj map[string]interface{}
		if err := yaml.Unmarshal(yamlBody, &obj); err != nil {
			return err
		}
		delete(obj, "notifications")
		delete(obj, "sync")

		optConfig, err := optimizer.LoadConfig(optimizerConfigFilename)
		if err != nil {
			return err
		}

		// the config json template used for patch
		configJson, err := json.MarshalIndent(obj, "", "  ")
		if err != nil {
			return err
		}
		log.Info(string(configJson))

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = ctx

		log.Info(os.Args)
		binary := os.Args[0]
		_ = binary

		type OpFunc func(configJson []byte, next func(configJson []byte) error) error
		var ops []OpFunc

		for _, selector := range optConfig.Matrix {
			path := selector.Path

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

					log.Infof("ranged values: %v", values)
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
					log.Infof("iterate values: %v", values)
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

		var last = func(configJson []byte, next func(configJson []byte) error) error {
			log.Info("configJson", string(configJson))
			return nil
		}
		ops = append(ops, last)

		log.Infof("%d ops: %v", len(ops), ops)

		var wrapper = func(configJson []byte) error { return nil }
		for i := len(ops) - 1; i > 0; i-- {
			next := ops[i]
			cur := ops[i-1]
			inner := wrapper
			a := i
			wrapper = func(configJson []byte) error {
				log.Infof("wrapper fn #%d", a)
				return cur(configJson, func(configJson []byte) error {
					return next(configJson, inner)
				})
			}
		}

		return wrapper(configJson)
	},
}
