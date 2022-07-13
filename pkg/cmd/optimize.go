package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"github.com/c9s/bbgo/pkg/optimizer"
)

func init() {
	optimizeCmd.Flags().String("optimizer-config", "optimizer.yaml", "config file")
	optimizeCmd.Flags().String("output", "output", "backtest report output directory")
	optimizeCmd.Flags().Bool("json", false, "print optimizer metrics in json format")
	optimizeCmd.Flags().Bool("tsv", false, "print optimizer metrics in csv format")
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
			if err := formatMetricsTsv(metrics, os.Stdout); err != nil {
				return err
			}
		} else {
			for n, values := range metrics {
				if len(values) == 0 {
					continue
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

func transformMetricsToRows(metrics map[string][]optimizer.Metric) (headers []string, rows [][]interface{}) {
	var metricsKeys []string
	for k := range metrics {
		metricsKeys = append(metricsKeys, k)
	}

	var numEntries int
	var paramLabels []string
	for _, ms := range metrics {
		for _, m := range ms {
			paramLabels = m.Labels
			break
		}

		numEntries = len(ms)
		break
	}

	headers = append(paramLabels, metricsKeys...)
	rows = make([][]interface{}, numEntries)

	var metricsRows = make([][]interface{}, numEntries)

	// build params into the rows
	for i, m := range metrics[metricsKeys[0]] {
		rows[i] = m.Params
	}

	for _, metricKey := range metricsKeys {
		for i, ms := range metrics[metricKey] {
			if len(metricsRows[i]) == 0 {
				metricsRows[i] = make([]interface{}, 0, len(metricsKeys))
			}
			metricsRows[i] = append(metricsRows[i], ms.Value)
		}
	}

	// merge rows
	for i := range rows {
		rows[i] = append(rows[i], metricsRows[i]...)
	}

	return headers, rows
}

func formatMetricsTsv(metrics map[string][]optimizer.Metric, writer io.WriteCloser) error {
	headers, rows := transformMetricsToRows(metrics)
	w := tsv.NewWriter(writer)
	if err := w.Write(headers); err != nil {
		return err
	}

	for _, row := range rows {
		var cells []string
		for _, o := range row {
			cell, err := castCellValue(o)
			if err != nil {
				return err
			}
			cells = append(cells, cell)
		}

		if err := w.Write(cells); err != nil {
			return err
		}
	}
	return w.Close()
}

func castCellValue(a interface{}) (string, error) {
	switch tv := a.(type) {
	case fixedpoint.Value:
		return tv.String(), nil
	case float64:
		return strconv.FormatFloat(tv, 'f', -1, 64), nil
	case int64:
		return strconv.FormatInt(tv, 10), nil
	case int32:
		return strconv.FormatInt(int64(tv), 10), nil
	case int:
		return strconv.Itoa(tv), nil
	case bool:
		return strconv.FormatBool(tv), nil
	case string:
		return tv, nil
	case []byte:
		return string(tv), nil
	default:
		return "", fmt.Errorf("unsupported object type: %T value: %v", tv, tv)
	}
}
