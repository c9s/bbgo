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

		outputDirectory, err := cmd.Flags().GetString("output")
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
			Bin:       os.Args[0],
			WorkDir:   ".",
			ConfigDir: configDir,
			OutputDir: outputDirectory,
		}

		optz := &optimizer.GridOptimizer{
			Config: optConfig,
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
		} else {
			for _, m := range metrics {
				fmt.Printf("%v => %v\n", m.Params, m.Value)
			}
		}

		return nil
	},
}
