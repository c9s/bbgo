package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/c9s/bbgo/pkg/optimizer"
	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func init() {
	hoptimizeCmd.Flags().String("optimizer-config", "optimizer.yaml", "config file")
	hoptimizeCmd.Flags().String("name", "", "assign an optimization session name")
	hoptimizeCmd.Flags().Bool("json-keep-all", false, "keep all results of trials")
	hoptimizeCmd.Flags().String("output", "output", "backtest report output directory")
	hoptimizeCmd.Flags().Bool("json", false, "print optimizer metrics in json format")
	hoptimizeCmd.Flags().Bool("tsv", false, "print optimizer metrics in csv format")
	RootCmd.AddCommand(hoptimizeCmd)
}

var hoptimizeCmd = &cobra.Command{
	Use:   "hoptimize",
	Short: "run hyperparameter optimizer (experimental)",

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

		optSessionName, err := cmd.Flags().GetString("name")
		if err != nil {
			return err
		}

		jsonKeepAll, err := cmd.Flags().GetBool("json-keep-all")
		if err != nil {
			return err
		}

		outputDirectory, err := cmd.Flags().GetString("output")
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

		go func() {
			c := make(chan os.Signal)
			signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
			<-c
			log.Info("Early stop by manual cancelation.")
			cancel()
		}()

		if len(optSessionName) == 0 {
			optSessionName = fmt.Sprintf("bbgo-hpopt-%v", time.Now().UnixMilli())
		}
		tempDirNameFormat := fmt.Sprintf("%s-config-*", optSessionName)
		configDir, err := os.MkdirTemp("", tempDirNameFormat)
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

		optz := &optimizer.HyperparameterOptimizer{
			SessionName: optSessionName,
			Config:      optConfig,
		}

		if err := executor.Prepare(configJson); err != nil {
			return err
		}

		report, err := optz.Run(ctx, executor, configJson)
		log.Info("All test trial finished.")
		if err != nil {
			return err
		}

		if printJsonFormat {
			if !jsonKeepAll {
				report.Trials = nil
			}
			out, err := json.MarshalIndent(report, "", "  ")
			if err != nil {
				return err
			}

			// print report JSON to stdout
			fmt.Println(string(out))
		} else if printTsvFormat {
			if err := optimizer.FormatResultsTsv(os.Stdout, report.Parameters, report.Trials); err != nil {
				return err
			}
		} else {
			color.Green("OPTIMIZER REPORT")
			color.Green("===============================================\n")
			color.Green("SESSION NAME: %s\n", report.Name)
			color.Green("OPTIMIZE OBJECTIVE: %s\n", report.Objective)
			color.Green("BEST OBJECTIVE VALUE: %s\n", report.Best.Value)
			color.Green("OPTIMAL PARAMETERS:")
			for _, selectorConfig := range optConfig.Matrix {
				label := selectorConfig.Label
				if val, exist := report.Best.Parameters[label]; exist {
					color.Green("  - %s: %v", label, val)
				} else {
					color.Red("  - %s: (invalid parameter definition)", label)
				}
			}
		}

		return nil
	},
}
