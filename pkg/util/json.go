package util

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
)

func WriteJsonFile(p string, obj interface{}) error {
	out, err := json.Marshal(obj)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(p, out, 0644)
}

func ReadJsonFile(file string, obj interface{}) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}

	defer f.Close()

	byteResult, err := io.ReadAll(f)
	if err != nil {
		return err
	}

	err = json.Unmarshal([]byte(byteResult), obj)
	if err != nil {
		return err
	}

	return nil
}
