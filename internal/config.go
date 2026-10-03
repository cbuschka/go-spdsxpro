package internal

import (
	"io"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Kits     []YamlKit     `json:"kits" yaml:"kits"`
	Setlists []YamlSetlist `json:"setlists" yaml:"setlists"`
}

type YamlSetlist struct {
	Slot  *int              `json:"slot" yaml:"slot"`
	Name  string            `json:"name" yaml:"name"`
	Steps []YamlSetlistStep `json:"steps" yaml:"steps"`
}

type YamlSetlistStep struct {
	Kit string `json:"kit" yaml:"kit"`
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

func (c *Config) Validate() error {
	for i, kit := range c.Kits {
		if kit.Slot != nil && (*kit.Slot < 1 || *kit.Slot > 100) {
			return &ConfigError{Field: fmtPath("kits", i, "slot"), Message: "slot must be between 1 and 100"}
		}
		if kit.Name == "" {
			return &ConfigError{Field: fmtPath("kits", i, "name"), Message: "kit name cannot be empty"}
		}
	}

	for i, setlist := range c.Setlists {
		if setlist.Name == "" {
			return &ConfigError{Field: fmtPath("setlists", i, "name"), Message: "setlist name cannot be empty"}
		}

		for stepIdx, step := range setlist.Steps {
			_, found := c.GetKitByName(step.Kit)
			if !found {
				return &ConfigError{Field: fmtPath("setlists", i, fmtPath("steps", stepIdx, "kit")), Message: "setlist name cannot be empty"}
			}
		}
	}

	return nil
}

type ConfigError struct {
	Field   string
	Message string
}

func (e *ConfigError) Error() string {
	return e.Field + ": " + e.Message
}

func fmtPath(section string, index int, field string) string {
	return string([]byte(section)) + "[" + string(rune('0'+index)) + "]." + field
}

func (c *Config) GetKitByName(name string) (*YamlKit, bool) {
	for _, kit := range c.Kits {
		if kit.Name == name {
			return &kit, true
		}
	}

	return nil, false
}
