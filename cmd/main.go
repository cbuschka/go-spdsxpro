package main

import (
	"context"
	"fmt"
	"go-spdsxpro/types/clientopts"
	"log"
	"math"
	"time"

	"go-spdsxpro"
	"go-spdsxpro/types"
)

func main() {
	var err error
	// Create a context with a strict 2-second timeout
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	log.Printf("Attempting to connect to SPD-SX PRO...")

	// This MUST fail if no device is connected
	client, err := spdsxpro.NewClient("/dev/snd/midiC1D0", clientopts.WithDebug(true))
	if err != nil {
		log.Fatalf("FAILED TO CONNECT: %v", err)
	}
	defer client.Close()

	log.Printf("connected")

	layer := types.LayerB

	vol, err := client.GetPadLayerVolume(49, 0, layer)
	if err != nil {
		log.Fatalf("Error fetching pad vol: %v", err)
	}
	log.Printf("vol %d (%s)", vol, ValueToDb(vol))

	const VOL1 = 60
	const VOL2 = 0

	if vol != VOL1 {
		vol = VOL1
	} else {
		vol = VOL2
	}

	err = client.SetPadLayerVolume(49, 0, layer, vol)
	if err != nil {
		log.Fatalf("Error setting pad vol: %v", err)
	}

	vol, err = client.GetPadLayerVolume(49, 0, layer)
	if err != nil {
		log.Fatalf("Error fetching pad vol: %v", err)
	}
	log.Printf("vol %d (%s)", vol, ValueToDb(vol))

	/*
		vol, err = client.GetPadLayerVolume(49, 0, types.LayerB)
		if err != nil {
			log.Fatalf("Error fetching pad vol: %v", err)
		}
		log.Printf("vol %d (%s)", vol, ValueToDb(vol))
	*/
	/*
		err = dumpActiveKit(client)
		if err != nil {
			log.Fatalf("Error fetching active kit: %v", err)
		}

		err = dumpKitList(client)
		if err != nil {
			log.Fatalf("Error fetching kit list: %v", err)
		}

		err = dumpSetList(client)
		if err != nil {
			log.Fatalf("Error fetching set list: %v", err)
		}
	*/
}

func dumpKitList(client types.Client) error {
	var err error
	kits, err := client.GetKitList()
	if err != nil {
		return err
	}

	for idx, kit := range kits {
		fmt.Printf("kit %d: %s - %s\n", idx+1, kit.Name, kit.SubTitle)
	}

	return nil
}

func dumpActiveKit(client types.Client) error {
	var err error
	var kit int
	kit, err = client.GetActiveKit()
	if err != nil {
		return err
	}
	fmt.Printf("active kit %d\n", kit+1)
	return nil
}

func dumpSetList(client types.Client) error {
	var err error
	sets, err := client.GetSetlistList()
	if err != nil {
		return err
	}

	for idx, set := range sets {
		fmt.Printf("setlist %d: %s\n", idx+1, set.Name)
		for _, step := range set.Steps {
			fmt.Printf("    step %d: %d\n", step.StepNumber, step.KitNumber)
		}
	}

	return nil
}

func ValueToDb(val int) string {
	// Roland mute threshold is typically -600 (-60.0 dB) or lower
	if val <= -600 {
		return "-inf dB"
	}

	db := float64(val) / 10.0

	// Handle -0.0 display edge case
	if math.Abs(db) < 0.001 {
		return "0.0 dB"
	}

	if db > 0 {
		return fmt.Sprintf("+%.1f dB", db)
	}

	return fmt.Sprintf("%.1f dB", db)
}
