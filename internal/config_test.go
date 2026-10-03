package internal

import (
	"strings"
	"testing"
)

func TestReadConfig(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantErr     bool
		checkResult func(t *testing.T, cfg *Config)
	}{
		{
			name: "valid minimal config",
			input: `
kits:
  - name: "Kit 1"
setlists:
  - name: "Setlist 1"
    steps:
      - kit: "Kit 1"
`,
			wantErr: false,
			checkResult: func(t *testing.T, cfg *Config) {
				if len(cfg.Kits) != 1 || cfg.Kits[0].Name != "Kit 1" {
					t.Errorf("unexpected kits: %+v", cfg.Kits)
				}
				if len(cfg.Setlists) != 1 || cfg.Setlists[0].Name != "Setlist 1" {
					t.Errorf("unexpected setlists: %+v", cfg.Setlists)
				}
			},
		},
		{
			name: "malformed yaml syntax",
			input: `
kits:
  - name: [invalid yaml structure
`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := strings.NewReader(tt.input)
			cfg, err := ReadConfig(r)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.checkResult != nil {
				tt.checkResult(t, cfg)
			}
		})
	}
}

func TestConfigValidation(t *testing.T) {
	invalidSlot := 105
	validSlot := 10

	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "valid config with all fields",
			cfg: Config{
				Kits: []YamlKit{
					{Slot: &validSlot, Name: "Valid Kit"},
				},
				Setlists: []YamlSetlist{
					{Name: "Valid Setlist",
						Steps: []YamlSetlistStep{
							{Kit: "Valid Kit"},
						}},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid kit slot range",
			cfg: Config{
				Kits: []YamlKit{
					{Slot: &invalidSlot, Name: "Bad Slot Kit"},
				},
			},
			wantErr: true,
		},
		{
			name: "empty kit name",
			cfg: Config{
				Kits: []YamlKit{
					{Name: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "invalid kit in step",
			cfg: Config{
				Kits: []YamlKit{
					{Slot: &validSlot, Name: "Valid Kit"},
				},
				Setlists: []YamlSetlist{
					{Name: "Valid Setlist",
						Steps: []YamlSetlistStep{
							{Kit: "Invalid Kit"},
						}},
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cfg.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
