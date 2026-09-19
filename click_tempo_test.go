package spdsxpro

import (
	"fmt"
	"go-spdsxpro/types/clientopts"
	"math/rand/v2"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSetGetClickTempo(t *testing.T) {
	client, err := NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
	if err != nil {
		t.Fatalf("Failed to connect to SPD-SX PRO: %v", err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatalf("Device ping failed: %v", err)
	}

	randomNum := rand.N(10) + 1

	tests := []struct {
		Name   string
		KitIdx int
		Tempo  int
	}{
		{
			Name:   "Kit 50",
			KitIdx: 49,
			Tempo:  50 + randomNum,
		},
		{
			Name:   "Kit 51",
			KitIdx: 50,
			Tempo:  51 + randomNum,
		},
		{
			Name:   "Kit 52",
			KitIdx: 51,
			Tempo:  52 + randomNum,
		},
		{
			Name:   "Kit 53",
			KitIdx: 52,
			Tempo:  53 + randomNum,
		},
		{
			Name:   "Kit 65",
			KitIdx: 64,
			Tempo:  55 + randomNum,
		},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("change %s", tt.Name), func(t *testing.T) {

			err := client.SetActiveKit(tt.KitIdx)
			require.NoError(t, err)

			err = client.SetKitClickTempo(tt.KitIdx, float64(tt.Tempo))
			require.NoError(t, err)
		})
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("verify %s", tt.Name), func(t *testing.T) {
			err := client.SetActiveKit(tt.KitIdx)
			require.NoError(t, err)

			tempo, err := client.GetKitClickTempo(tt.KitIdx)
			require.NoError(t, err)
			require.Equal(t, float64(tt.Tempo), tempo)
		})
	}
}
