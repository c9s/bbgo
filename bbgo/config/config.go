package config

import (
	"encoding/json"
	"io/ioutil"
)

func LoadConfigFile(filename string, v interface{}) error {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

func SaveConfigFile(filename string, v interface{}) error {
	out, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(filename, out, 0644)
}
