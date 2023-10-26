package csvsource

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

func Download(symbol string, start time.Time) {
	for {
		var (
			path     = fmt.Sprintf("pkg/datasource/csv/testdata/bybit/%s/", symbol)
			fileName = fmt.Sprintf("%s%s.csv", strings.ToUpper(symbol), start.Format("2006-01-02"))
		)

		if fileExists(path + fileName) {
			start = start.AddDate(0, 0, 1)
			continue
		}

		var url = fmt.Sprintf("https://public.bybit.com/trading/%s/%s.gz",
			strings.ToUpper(symbol),
			fileName)

		fmt.Println("fetching ", url)

		err := readCSVFromUrl(url, path, fileName)
		if err != nil {
			fmt.Println(err)
			break
		}

		start = start.AddDate(0, 0, 1)
	}
}

func readCSVFromUrl(url, path, fileName string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	csvContent, err := gUnzipData(body)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); errors.Is(err, os.ErrNotExist) {
		err := os.MkdirAll(path, os.ModePerm)
		if err != nil {
			return err
		}
	}

	err = os.WriteFile(path+fileName, csvContent, 0666)
	if err != nil {
		return err
	}

	return nil
}

func gUnzipData(data []byte) (resData []byte, err error) {
	b := bytes.NewBuffer(data)

	var r io.Reader
	r, err = gzip.NewReader(b)
	if err != nil {
		return
	}

	var resB bytes.Buffer
	_, err = resB.ReadFrom(r)
	if err != nil {
		return
	}

	resData = resB.Bytes()

	return
}

func fileExists(fileName string) bool {
	info, err := os.Stat(fileName)
	if os.IsNotExist(err) {
		return false
	}
	return !info.IsDir()
}
