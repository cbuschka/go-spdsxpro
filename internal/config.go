package internal

import (
	"io"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kits []YamlKit `json:"kits" yaml:"kits"`
}

type YamlKit struct {
	Slot     *int             `json:"slot" yaml:"slot"`
	Name     string           `json:"name" yaml:"name"`
	Subtitle string           `json:"subtitle" yaml:"subtitle"`
	Click    YamlKitClick     `json:"click" yaml:"click"`
	PadLinks []YamlKitPadLink `json:"padLinks" yaml:"padLinks"`
}

type YamlKitClickStartPad struct {
	From int `json:"from" yaml:"from"`
	To   int `json:"to" yaml:"to"`
}

type YamlKitClick struct {
	Mode     uint8                `json:"mode" yaml:"mode"`
	Sound    uint8                `json:"sound" yaml:"sound"`
	Volume   int16                `json:"volume" yaml:"volume"`
	Tempo    uint16               `json:"tempo" yaml:"tempo"`
	StartPad YamlKitClickStartPad `json:"startPad" yaml:"startPad"`
}

type YamlKitPadLink struct {
	PadIndex int `json:"padIndex" yaml:"padIndex"`
	Tx       int `json:"tx" yaml:"tx"`
	Rx       int `json:"rx" yaml:"rx"`
}

func ReadConfig(in io.Reader) (*Config, error) {
	all, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	config := Config{}
	err = yaml.Unmarshal(all, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
