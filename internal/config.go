package internal

import (
	"encoding/json"
	"io"
)

type JsonConfig struct {
	Kits []JsonKit `json:"kits"`
}

type JsonKit struct {
	Slot     *int             `json:"slot"`
	Name     string           `json:"name"`
	Subtitle string           `json:"subtitle"`
	Click    JsonKitClick     `json:"click"`
	PadLinks []JsonKitPadLink `json:"padLinks"`
}

type JsonKitClick struct {
	Tempo         uint16 `json:"tempo"`
	PadStartRange int    `json:"padStartRange"`
}

type JsonKitPadLink struct {
	PadIndex int `json:"padIndex"`
	Tx       int `json:"tx"`
	Rx       int `json:"rx"`
}

func ReadConfig(in io.Reader) (*JsonConfig, error) {
	all, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	config := JsonConfig{}
	err = json.Unmarshal(all, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
