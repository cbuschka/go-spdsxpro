package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"go-spdsxpro/spdsxpro"
)

func main() {
	var err error
	// Create a context with a strict 2-second timeout
	_, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	fmt.Println("Attempting to connect to SPD-SX PRO...")

	// This MUST fail if no device is connected
	client, err := spdsxpro.NewClient("/dev/snd/midiC1D0")
	if err != nil {
		log.Fatalf("FAILED TO CONNECT: %v", err)
	}
	defer client.Close()

	fmt.Printf("connected\n")
	var kit int
	kit, err = client.GetActiveKit()
	if err != nil {
		log.Fatalf("Error fetching active kit: %v", err)
	}
	fmt.Printf("active kit %d\n", kit)

}
