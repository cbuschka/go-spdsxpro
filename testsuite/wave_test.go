package testsuite

import (
	"go-spdsxpro/types/clientopts"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKitPadLayerWave(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	client, err := spdsxpro.NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
	if err != nil {
		t.Fatalf("Failed to connect to SPD-SX PRO: %v", err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatalf("Device ping failed: %v", err)
	}

	wave, err := client.GetKitPadLayerWave(50, 1, types.LayerA)
	require.NoErr(err)

	t.Infof("wave %d", wave)
}
