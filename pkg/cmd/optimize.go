package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/optimizer"
)

func init() {
	optimizeCmd.Flags().String("optimizer-config", "optimizer.yaml", "config file")
	optimizeCmd.Flags().String("output", "output", "backtest report output directory")
	optimizeCmd.Flags().Bool("json", false, "print optimizer metrics in json format")
	optimizeCmd.Flags().Bool("tsv", false, "print optimizer metrics in csv format")
	optimizeCmd.Flags().Int("limit", 50, "limit how many results to print pr metric")
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

		printJsonFormat, err := cmd.Flags().GetBool("json")
		if err != nil {
			return err
		}

		printTsvFormat, err := cmd.Flags().GetBool("tsv")
		if err != nil {
			return err
		}

		outputDirectory, err := cmd.Flags().GetString("output")
		if err != nil {
			return err
		}

		resultLimit, err := cmd.Flags().GetInt("limit")
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

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_ = ctx

		configDir, err := os.MkdirTemp("", "bbgo-config-*")
		if err != nil {
			return err
		}

		executor := &optimizer.LocalProcessExecutor{
			Config:    optConfig.Executor.LocalExecutorConfig,
			Bin:       os.Args[0],
			WorkDir:   ".",
			ConfigDir: configDir,
			OutputDir: outputDirectory,
		}

		optz := &optimizer.GridOptimizer{
			Config: optConfig,
		}

		if err := executor.Prepare(configJson); err != nil {
			return err
		}

		metrics, err := optz.Run(executor, configJson)
		if err != nil {
			return err
		}

		if printJsonFormat {
			out, err := json.MarshalIndent(metrics, "", "  ")
			if err != nil {
				return err
			}

			// print metrics JSON to stdout
			fmt.Println(string(out))
		} else if printTsvFormat {
			if err := optimizer.FormatMetricsTsv(os.Stdout, metrics); err != nil {
				return err
			}
		} else {
			for n, values := range metrics {
				if len(values) == 0 {
					continue
				}

				if len(values) > resultLimit && resultLimit != 0 {
					values = values[:resultLimit]
				}

				fmt.Printf("%v => %s\n", values[0].Labels, n)
				for _, m := range values {
					fmt.Printf("%v => %s %v\n", m.Params, n, m.Value)
				}
			}
		}

		return nil
	},
}
