package spdsxpro

import (
	"fmt"
	"go-spdsxpro/types"
	"go-spdsxpro/types/clientopts"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// KitPropertyTestCase defines all configurable properties for a Kit.
type KitPropertyTestCase struct {
	Index        int
	Name         string
	Subtitle     string
	ClickTempo   float64
	ClickMode    int
	ClickSound   int
	ClickVolume  int
	PadStartFrom int
	PadStartTo   int
	PadIndex     int
	PadLinkTx    int
	PadLinkRx    int
	PadLayer     types.PadLayer
	LayerVolume  int
}

func TestActiveKit(t *testing.T) {
	client, err := NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
	if err != nil {
		t.Fatalf("Failed to connect to SPD-SX PRO: %v", err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatalf("Device ping failed: %v", err)
	}

	err = client.SetActiveKit(50)
	require.NoError(t, err)
}

func TestKitClickTempo(t *testing.T) {
	client, err := NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
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

func TestKits49To67_AllPropertiesSetAndGet(t *testing.T) {
	client, err := NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
	if err != nil {
		t.Fatalf("Failed to connect to SPD-SX PRO: %v", err)
	}
	defer client.Close()

	if err := client.Ping(); err != nil {
		t.Fatalf("Device ping failed: %v", err)
	}

	var tests []KitPropertyTestCase

	for kitIdx := 49; kitIdx <= 67; kitIdx++ {
		tests = append(tests, KitPropertyTestCase{
			Index:        kitIdx,
			Name:         fmt.Sprintf("KIT_%dnew", kitIdx+1),
			Subtitle:     fmt.Sprintf("SUB_%d", kitIdx+1),
			ClickTempo:   float64(kitIdx + 1),
			ClickMode:    1,
			ClickSound:   (kitIdx % 3) + 1, // Rotating sounds 1-3
			ClickVolume:  -100 + kitIdx,
			PadStartFrom: 1,
			PadStartTo:   9,
			PadIndex:     0,
			PadLinkTx:    2,
			PadLinkRx:    1,
			PadLayer:     types.LayerB,
			LayerVolume:  -120, // -12.0 dB
		})
	}

	for _, p := range tests {
		if err := client.SetKitName(p.Index, p.Name); err != nil {
			t.Fatalf("[Kit %d] SetKitName failed: %v", p.Index, err)
		}
		/*
			if err := client.SetKitSubTitle(p.Index, p.Subtitle); err != nil {
				t.Fatalf("[Kit %d] SetKitSubTitle failed: %v", p.Index, err)
			}*/

		// Click Tempo
		err := client.SetKitClickTempo(p.Index, p.ClickTempo)
		require.NoError(t, err, "[Kit %d] SetKitClickTempo failed: %v", p.Index, err)
		time.Sleep(30 * time.Millisecond)
		gotTempo, err := client.GetKitClickTempo(p.Index)
		require.NoError(t, err, "[Kit %d] GetKitClickTempo failed: %v", p.Index, err)
		require.Equal(t, p.ClickTempo, gotTempo, "[Kit %d] ClickTempo mismatch: got %.1f, want %.1f", p.Index, gotTempo, p.ClickTempo)
	}

	for _, p := range tests {

		gotTempo, err := client.GetKitClickTempo(p.Index)
		require.NoError(t, err, "[Kit %d] GetKitClickTempo failed: %v", p.Index, err)
		require.Equal(t, p.ClickTempo, gotTempo, "[Kit %d] ClickTempo mismatch: got %.1f, want %.1f", p.Index, gotTempo, p.ClickTempo)

		/*
			// Click Mode
			if err := client.SetKitClickMode(kIdx, p.ClickMode); err != nil {
				t.Fatalf("[Kit %d] SetKitClickMode failed: %v", tt.Index, err)
			}
			// Click Sound
			if err := client.SetKitClickSound(kIdx, p.ClickSound); err != nil {
				t.Fatalf("[Kit %d] SetKitClickSound failed: %v", tt.Index, err)
			}
			// Click Volume
			if err := client.SetKitClickVolume(kIdx, p.ClickVolume); err != nil {
				t.Fatalf("[Kit %d] SetKitClickVolume failed: %v", tt.Index, err)
			}
			// Pad Layer Volume
			if err := client.SetPadLayerVolume(kIdx, p.PadIndex, p.PadLayer, p.LayerVolume); err != nil {
				t.Fatalf("[Kit %d] SetPadLayerVolume failed: %v", tt.Index, err)
			}

			if err := client.SetKitClickStartRangeFrom(kIdx, p.PadStartFrom); err != nil {
				t.Fatalf("[Kit %d] SetKitClickStartRangeFrom failed: %v", tt.Index, err)
			}
			if err := client.SetKitClickStartRangeTo(kIdx, p.PadStartTo); err != nil {
				t.Fatalf("[Kit %d] SetKitClickStartRangeTo failed: %v", tt.Index, err)
			}
			if err := client.SetKitPadLinkSend(kIdx, p.PadIndex, p.PadLinkTx); err != nil {
				t.Fatalf("[Kit %d] SetKitPadLinkSend failed: %v", tt.Index, err)
			}
			if err := client.SetKitPadLinkReceive(kIdx, p.PadIndex, p.PadLinkRx); err != nil {
				t.Fatalf("[Kit %d] SetKitPadLinkReceive failed: %v", tt.Index, err)
			}
		*/
	}

	/*
		for _, p := range tests {
			// Click Tempo
			gotTempo, err := client.GetKitClickTempo(p.Index)
			require.NoError(t, err, "[Kit %d] GetKitClickTempo failed: %v", p.Index, err)
			require.Equal(t, p.ClickTempo, gotTempo, "[Kit %d] ClickTempo mismatch: got %.1f, want %.1f", p.Index, gotTempo, p.ClickTempo)

			/*
				// Click Mode
				gotMode, err := client.GetKitClickMode(kIdx)
				if err != nil {
					t.Errorf("[Kit %d] GetKitClickMode failed: %v", tt.Index, err)
				} else if gotMode != p.ClickMode {
					t.Errorf("[Kit %d] ClickMode mismatch: got %d, want %d", tt.Index, gotMode, p.ClickMode)
				}

				// Click Sound
				gotSound, err := client.GetKitClickSound(kIdx)
				if err != nil {
					t.Errorf("[Kit %d] GetKitClickSound failed: %v", tt.Index, err)
				} else if gotSound != p.ClickSound {
					t.Errorf("[Kit %d] ClickSound mismatch: got %d, want %d", tt.Index, gotSound, p.ClickSound)
				}

				// Click Volume
				gotVolume, err := client.GetKitClickVolume(kIdx)
				if err != nil {
					t.Errorf("[Kit %d] GetKitClickVolume failed: %v", tt.Index, err)
				} else if gotVolume != p.ClickVolume {
					t.Errorf("[Kit %d] ClickVolume mismatch: got %d, want %d", tt.Index, gotVolume, p.ClickVolume)
				}

				// Pad Layer Volume
				gotLayerVol, err := client.GetPadLayerVolume(kIdx, p.PadIndex, p.PadLayer)
				if err != nil {
					t.Errorf("[Kit %d] GetPadLayerVolume failed: %v", tt.Index, err)
				} else if gotLayerVol != p.LayerVolume {
					t.Errorf("[Kit %d] LayerVolume mismatch: got %d, want %d", tt.Index, gotLayerVol, p.LayerVolume)
				}

		}

	*/
}
