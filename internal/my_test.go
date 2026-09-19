package internal

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKitBaseAddress(t *testing.T) {

	tests := []struct {
		kitIdx          int
		expectedAddress [4]byte
	}{
		{
			kitIdx:          0,
			expectedAddress: [4]byte{0x04, 0x00, 0x00, 0x00},
		},
		{
			kitIdx:          1,
			expectedAddress: [4]byte{0x04, 0x02, 0x00, 0x00},
		},
		{
			kitIdx:          64,
			expectedAddress: [4]byte{0x05, 0x00, 0x00, 0x00},
		},
		{
			kitIdx:          199,
			expectedAddress: [4]byte{0x07, 0x0E, 0x00, 0x00},
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
		expectedAddress [4]byte
	}{
		{
			kitIdx:          0,
			expectedAddress: [4]byte{0x04, 0x00, 0x00, 0x56},
		},
		{
			kitIdx:          1,
			expectedAddress: [4]byte{0x04, 0x02, 0x00, 0x56},
		},
		{
			kitIdx:          64,
			expectedAddress: [4]byte{0x05, 0x00, 0x00, 0x56},
		},
		{
			kitIdx:          199,
			expectedAddress: [4]byte{0x07, 0x0E, 0x00, 0x56},
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
		expectedAddress [4]byte
	}{
		{
			kitIdx:          0,
			expectedAddress: [4]byte{0x04, 0x00, 0x00, 0x50},
		},
		{
			kitIdx:          1,
			expectedAddress: [4]byte{0x04, 0x02, 0x00, 0x50},
		},
		{
			kitIdx:          64,
			expectedAddress: [4]byte{0x05, 0x00, 0x00, 0x50},
		},
		{
			kitIdx:          199,
			expectedAddress: [4]byte{0x07, 0x0E, 0x00, 0x50},
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("kit %d", tt.kitIdx), func(t *testing.T) {
			addr := kitParamAddress(tt.kitIdx, 0x00, 0x0050)
			require.Equal(t, tt.expectedAddress[:], addr)
		})
	}
}
