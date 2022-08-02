package optimizer

import (
	"fmt"
	"github.com/c9s/bbgo/pkg/data/tsv"
	"github.com/c9s/bbgo/pkg/fixedpoint"
	"io"
	"strconv"
)

func FormatResultsTsv(writer io.WriteCloser, labelPaths map[string]string, results []*HyperparameterOptimizeTrialResult) error {
	headerLen := len(labelPaths)
	headers := make([]string, 0, headerLen)
	for label := range labelPaths {
		headers = append(headers, label)
	}

	rows := make([][]interface{}, len(labelPaths))
	for ri, result := range results {
		row := make([]interface{}, headerLen)
		for ci, columnKey := range headers {
			var ok bool
			if row[ci], ok = result.Parameters[columnKey]; !ok {
				return fmt.Errorf(`missing parameter "%s" from trial result (%v)`, columnKey, result.Parameters)
			}
		}
		rows[ri] = row
	}

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

func FormatMetricsTsv(writer io.WriteCloser, metrics map[string][]Metric) error {
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

func transformMetricsToRows(metrics map[string][]Metric) (headers []string, rows [][]interface{}) {
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
