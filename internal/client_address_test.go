package internal

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetKitParamAddress(t *testing.T) {
	client := &linuxMidiClient{}

	tests := []struct {
		name             string
		kitIdx           int
		subsectionOffset uint32
		paramOffset      uint32
		expectedAddress  []byte
	}{
		// --- KIT 0 (Slot 1: Base Address 0x04 0x00 0x00 0x00) ---
		{
			name:             "Kit 0 - Kit Common (Name)",
			kitIdx:           0,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitName,
			expectedAddress:  []byte{0x04, 0x00, 0x00, 0x00},
		},
		{
			name:             "Kit 0 - Kit Common (Subtitle)",
			kitIdx:           0,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitSubTitle,
			expectedAddress:  []byte{0x04, 0x00, 0x00, 0x10},
		},
		{
			name:             "Kit 0 - Kit Common (Tempo)",
			kitIdx:           0,
			subsectionOffset: KitCommon,
			paramOffset:      OffsetTempo,
			expectedAddress:  []byte{0x04, 0x00, 0x00, 0x56},
		},
		{
			name:             "Kit 0 - Kit Click (Mode)",
			kitIdx:           0,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickMode,
			expectedAddress:  []byte{0x04, 0x00, 0x03, 0x00},
		},
		{
			name:             "Kit 0 - Kit Click (Volume)",
			kitIdx:           0,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickVolume,
			expectedAddress:  []byte{0x04, 0x00, 0x03, 0x06},
		},
		{
			name:             "Kit 0 - Kit Control Offset",
			kitIdx:           0,
			subsectionOffset: KitControl,
			paramOffset:      0x00000000,
			expectedAddress:  []byte{0x04, 0x00, 0x02, 0x00},
		},

		// --- KIT 49 (Slot 50: Base Address 0x04 0x62 0x00 0x00) ---
		{
			name:             "Kit 49 - Kit Common (Name)",
			kitIdx:           49,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitName,
			expectedAddress:  []byte{0x04, 0x62, 0x00, 0x00},
		},
		{
			name:             "Kit 49 - Kit Common (Subtitle)",
			kitIdx:           49,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitSubTitle,
			expectedAddress:  []byte{0x04, 0x62, 0x00, 0x10},
		},
		{
			name:             "Kit 49 - Kit Common (Tempo)",
			kitIdx:           49,
			subsectionOffset: KitCommon,
			paramOffset:      OffsetTempo,
			expectedAddress:  []byte{0x04, 0x62, 0x00, 0x56},
		},
		{
			name:             "Kit 49 - Kit Click (Sound)",
			kitIdx:           49,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickSound,
			expectedAddress:  []byte{0x04, 0x62, 0x03, 0x01},
		},
		{
			name:             "Kit 49 - Kit Click (Pan)",
			kitIdx:           49,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickPan,
			expectedAddress:  []byte{0x04, 0x62, 0x03, 0x0B},
		},

		// --- KIT 67 (Slot 68: Base Address 0x05 0x06 0x00 0x00) ---
		// (Requires Byte 1 carry over past 64 kits: 67 * 2 = 134 => Byte1: 0x05, Byte2: 0x06)
		{
			name:             "Kit 67 - Kit Common (Name)",
			kitIdx:           67,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitName,
			expectedAddress:  []byte{0x05, 0x06, 0x00, 0x00},
		},
		{
			name:             "Kit 67 - Kit Common (Subtitle)",
			kitIdx:           67,
			subsectionOffset: KitCommon,
			paramOffset:      ParamOffsetKitSubTitle,
			expectedAddress:  []byte{0x05, 0x06, 0x00, 0x10},
		},
		{
			name:             "Kit 67 - Kit Common (Tempo)",
			kitIdx:           67,
			subsectionOffset: KitCommon,
			paramOffset:      OffsetTempo,
			expectedAddress:  []byte{0x05, 0x06, 0x00, 0x56},
		},
		{
			name:             "Kit 67 - Kit Click (Start Range 1)",
			kitIdx:           67,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickStartRange1,
			expectedAddress:  []byte{0x05, 0x06, 0x03, 0x0E},
		},
		{
			name:             "Kit 67 - Kit Click (Start Range 2)",
			kitIdx:           67,
			subsectionOffset: KitClick,
			paramOffset:      OffsetClickStartRange2,
			expectedAddress:  []byte{0x05, 0x06, 0x03, 0x16},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.getKitParamAddress(tt.kitIdx, tt.subsectionOffset, tt.paramOffset)
			if !bytes.Equal(got[:], tt.expectedAddress[:]) {
				t.Errorf("getKitParamAddress(%d, 0x%08X, 0x%08X)\n  got:  %02X\n  want: %02X",
					tt.kitIdx, tt.subsectionOffset, tt.paramOffset, got, tt.expectedAddress)
			}
		})
	}
}

func TestKitBaseAddress(t *testing.T) {

	tests := []struct {
		kitIdx          int
		expectedAddress []byte
	}{
		{
			kitIdx:          0,
			expectedAddress: []byte{0x04, 0x00, 0x00, 0x00},
		},
		{
			kitIdx:          1,
			expectedAddress: []byte{0x04, 0x02, 0x00, 0x00},
		},
		{
			kitIdx:          64,
			expectedAddress: []byte{0x05, 0x00, 0x00, 0x00},
		},
		{
			kitIdx:          199,
			expectedAddress: []byte{0x07, 0x0E, 0x00, 0x00},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("kit %d", tt.kitIdx), func(t *testing.T) {
			addr := kitBaseAddress(tt.kitIdx)
			require.Equal(t, tt.expectedAddress[:], addr)
		})
	}
}

func TestKitClickTempoAddress(t *testing.T) {

	tests := []struct {
		kitIdx          int
		expectedAddress []byte
	}{
		{
			kitIdx:          0,
			expectedAddress: []byte{0x04, 0x00, 0x00, 0x56},
		},
		{
			kitIdx:          1,
			expectedAddress: []byte{0x04, 0x02, 0x00, 0x56},
		},
		{
			kitIdx:          64,
			expectedAddress: []byte{0x05, 0x00, 0x00, 0x56},
		},
		{
			kitIdx:          199,
			expectedAddress: []byte{0x07, 0x0E, 0x00, 0x56},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("kit %d", tt.kitIdx), func(t *testing.T) {
			addr := kitParamAddress(tt.kitIdx, SectionKitCommon, OffsetKitCommonTempo)
			require.Equal(t, tt.expectedAddress[:], addr)
		})
	}
}

func TestKitVolumeAddress(t *testing.T) {

	tests := []struct {
		kitIdx          int
		expectedAddress []byte
	}{
		{
			kitIdx:          0,
			expectedAddress: []byte{0x04, 0x00, 0x00, 0x50},
		},
		{
			kitIdx:          1,
			expectedAddress: []byte{0x04, 0x02, 0x00, 0x50},
		},
		{
			kitIdx:          64,
			expectedAddress: []byte{0x05, 0x00, 0x00, 0x50},
		},
		{
			kitIdx:          199,
			expectedAddress: []byte{0x07, 0x0E, 0x00, 0x50},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("kit %d", tt.kitIdx), func(t *testing.T) {
			addr := kitParamAddress(tt.kitIdx, 0x00, 0x0050)
			require.Equal(t, tt.expectedAddress[:], addr)
		})
	}
}
