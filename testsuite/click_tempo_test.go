package testsuite

import (
	"go-spdsxpro"
	"go-spdsxpro/types/clientopts"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestKitClickTempo(t *testing.T) {
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

	for kitIdx := 49; kitIdx < 50; kitIdx++ {
		for _, newTempo := range []float64{20, 80, 120, 162.2, 180} {
			// if err := client.SetKitClickTempo(kitIdx, newTempo); err != nil {
			// 	t.Fatalf("[Kit %d] SetKitClickTempo failed: %v", kitIdx, err)
			// }

			err := client.SetActiveKit(kitIdx)
			require.NoError(t, err)

			mode, err := client.GetKitClickMode(kitIdx)
			require.NoError(t, err)
			require.Equal(t, 0, mode, "[Kit %d] GetKitClickMode failed: %v", kitIdx, mode)

			tempo, err := client.GetKitClickTempo(kitIdx)
			require.NoError(t, err)
			require.Equal(t, newTempo, tempo, "[Kit %d] GetKitClickTempo failed: %v", kitIdx, tempo)
		}

	}

}
