package internal

import (
	"errors"
	"fmt"
	"io"

	yaml "gopkg.in/yaml.v3"
)

var ErrKitNotFound = errors.New("kit not found")
var ErrWaveNotFound = errors.New("wave not found")

type Config struct {
	Kits     []YamlKit     `json:"kits" yaml:"kits"`
	Setlists []YamlSetlist `json:"setlists" yaml:"setlists"`
	Waves    []YamlWave    `json:"waves" yaml:"waves"`
}

type YamlWave struct {
	Slot *int   `json:"slot" yaml:"slot"`
	Name string `json:"name" yaml:"name"`
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
	Slot     *int         `json:"slot" yaml:"slot"`
	Name     string       `json:"name" yaml:"name"`
	Subtitle string       `json:"subtitle" yaml:"subtitle"`
	Click    YamlKitClick `json:"click" yaml:"click"`
	Pads     []YamlKitPad `json:"pads" yaml:"pads"`
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

type YamlKitPad struct {
	PadIndex    int             `json:"padIndex" yaml:"padIndex"`
	LayerA      YamlKitPadLayer `json:"layerA" yaml:"layerA"`
	LayerB      YamlKitPadLayer `json:"layerB" yaml:"layerB"`
	LinkSend    int             `json:"linkSend" yaml:"linkSend"`
	LinkReceive int             `json:"linkReceive" yaml:"linkReceive"`
	Dynamics    string          `json:"dynamics" yaml:"dynamics"`
}

type YamlKitPadLayer struct {
	Wave        string `json:"wave" yaml:"wave"`
	Poly        bool   `json:"poly" yaml:"poly"`
	Loop        string `json:"loop" yaml:"loop"`
	TriggerType string `json:"triggerType" yaml:"triggerType"`
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

		for _, pad := range kit.Pads {
			if pad.LayerA.Wave != "" {
				_, err := c.GetWaveByName(pad.LayerA.Wave)
				if err != nil {
					return &ConfigError{Field: fmtPath("kits", i, "wave"), Message: "wave not found"}
				}
			}
			if pad.LayerB.Wave != "" {
				_, err := c.GetWaveByName(pad.LayerA.Wave)
				if err != nil {
					return &ConfigError{Field: fmtPath("kits", i, "wave"), Message: "wave not found"}
				}
			}

		}
	}

	for i, setlist := range c.Setlists {
		if setlist.Name == "" {
			return &ConfigError{Field: fmtPath("setlists", i, "name"), Message: "setlist name cannot be empty"}
		}

		for stepIdx, step := range setlist.Steps {
			_, err := c.GetKitByName(step.Kit)
			if err != nil {
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

func (c *Config) GetKitByName(name string) (*YamlKit, error) {
	kitsFound := make([]*YamlKit, 0)
	for _, kit := range c.Kits {
		if kit.Name == name {
			kitsFound = append(kitsFound, &kit)
		}
	}

	if len(kitsFound) == 0 {
		return nil, fmt.Errorf("kit with name %s not found %w", name, ErrKitNotFound)
	}

	if len(kitsFound) > 1 {
		return nil, fmt.Errorf("ambiguous kit name %s %w", name, ErrKitNotFound)
	}

	return kitsFound[0], nil
}

func (c *Config) GetWaveByName(name string) (*YamlWave, error) {
	wavesFound := make([]*YamlWave, 0)
	for _, wave := range c.Waves {
		if wave.Name == name {
			wavesFound = append(wavesFound, &wave)
		}
	}

	if len(wavesFound) == 0 {
		return nil, fmt.Errorf("wave with name %s not found %w", name, ErrWaveNotFound)
	}

	if len(wavesFound) > 1 {
		return nil, fmt.Errorf("ambiguous wave name %s %w", name, ErrWaveNotFound)
	}

	return wavesFound[0], nil
}

func NewEmptyPad(padIdx int) *YamlKitPad {
	return &YamlKitPad{
		PadIndex: padIdx,
		LayerA: YamlKitPadLayer{
			Wave: "",
			Poly: false,
		},
		LayerB: YamlKitPadLayer{
			Wave: "",
			Poly: false,
		},
		LinkSend:    0,
		LinkReceive: 0,
		Dynamics:    "off",
	}
}

func (k *YamlKit) GetPad(padIdx int) *YamlKitPad {
	for _, pad := range k.Pads {
		if pad.PadIndex == padIdx {
			return &pad
		}
	}

	return nil
}
