package main

import (
	"io/ioutil"

	"github.com/CRASH-Tech/dhcp-operator/cmd/common"
	"gopkg.in/yaml.v2"
)

func readConfig(path string) (common.Config, error) {
	config := common.Config{}

	yamlFile, err := ioutil.ReadFile(path)
	if err != nil {
		return common.Config{}, err
	}
	err = yaml.Unmarshal(yamlFile, &config)
	if err != nil {
		return common.Config{}, err
	}

	return config, err
}
