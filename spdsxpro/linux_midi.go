package spdsxpro

import (
	"errors"
	"fmt"
	"log"
	"os"
	"time"
)

const (
	RolandHeaderByte = 0xF0
	RolandEOXByte    = 0xF7
	RolandVendorID   = 0x41
	DeviceIDAll      = 0x10 // Default Roland Device ID (Base 17 / 0x10)

	CmdRQ1 = 0x11 // Request Data 1
	CmdDT1 = 0x12 // Data Set 1 (Response/Write)
)

// SPD-SX PRO 5-byte Model ID verified from hardware output
var ModelIDSPDSXPro = []byte{0x00, 0x00, 0x00, 0x00, 0x16}

// computeRolandChecksum calculates the Roland 7-bit checksum:
// 128 - ((sum of address + size/data bytes) % 128)
func computeRolandChecksum(data []byte) byte {
	var sum int
	for _, b := range data {
		sum += int(b)
	}
	return byte((128 - (sum % 128)) & 0x7F)
}

func encodeRQ1(deviceID byte, modelID []byte, addr [4]byte, size [4]byte) []byte {
	msg := []byte{RolandHeaderByte, RolandVendorID, deviceID}
	msg = append(msg, modelID...)
	msg = append(msg, CmdRQ1)

	payload := append(addr[:], size[:]...)
	checksum := computeRolandChecksum(payload)

	msg = append(msg, payload...)
	msg = append(msg, checksum, RolandEOXByte)
	return msg
}

func parseActiveKitResponse(resp []byte) (int, error) {
	minLen := 3 + len(ModelIDSPDSXPro) + 1 + 4 + 4 + 1 + 1 // Header+Vendor+Dev + ModelID + Cmd + Addr + Data(4) + CS + EOX
	if len(resp) < minLen {
		return 0, fmt.Errorf("response payload too short (%d bytes)", len(resp))
	}

	if resp[0] != RolandHeaderByte || resp[len(resp)-1] != RolandEOXByte {
		return 0, errors.New("invalid SysEx framing")
	}

	cmdIdx := 3 + len(ModelIDSPDSXPro)
	if resp[cmdIdx] != CmdDT1 {
		return 0, fmt.Errorf("expected DT1 response (0x12), got 0x%02X", resp[cmdIdx])
	}

	checksumIdx := len(resp) - 2
	payloadForChecksum := resp[cmdIdx+1 : checksumIdx]
	if computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
		return 0, errors.New("checksum mismatch")
	}

	// Data payload sits between 4-byte address and 1-byte checksum
	dataBytes := resp[cmdIdx+5 : checksumIdx]
	if len(dataBytes) < 4 {
		return 0, fmt.Errorf("expected 4 data bytes, got %d", len(dataBytes))
	}

	// Decode Roland 4-nibble 16-bit integer: (d2 * 16) + d3
	kitIndex := (int(dataBytes[2]) << 4) | int(dataBytes[3])

	return kitIndex, nil
}

type linuxMidiClient struct {
	path     string
	deviceID byte
	dev      *os.File
	timeout  time.Duration
}

// Universal Non-Realtime SysEx Identity Request (Ping)
// Header: 0xF0, Non-Realtime ID: 0x7E, Target Device ID: 0x7F (All Call), General Info: 0x06, Identity Request: 0x01, EOX: 0xF7
var sysExPing = []byte{0xF0, 0x7E, 0x7F, 0x06, 0x01, 0xF7}

// Roland ID is 0x41
const rolandVendorID = 0x41

func (c *linuxMidiClient) Connect() error {
	dev, err := os.OpenFile(c.path, os.O_RDWR, 0)
	if err != nil {
		return err
	}

	c.dev = dev

	return nil
}

func (c *linuxMidiClient) Ping() error {
	reply, err := c.TransceiveSysEx(sysExPing)
	if err != nil {
		return err
	}

	return verifyResponse(reply)

}

func (c *linuxMidiClient) Close() error {
	if c.dev != nil {
		return c.dev.Close()
	}

	return nil
}

func verifyResponse(resp []byte) error {
	if len(resp) < 6 {
		return fmt.Errorf("[-] Payload too short to be a valid SysEx response.")
	}

	// Check SysEx framing: Start (0xF0) and End (0xF7)
	if resp[0] != 0xF0 || resp[len(resp)-1] != 0xF7 {
		return fmt.Errorf("[-] Malformed SysEx message framing.")
	}

	// Format: F0 7E <deviceID> 06 02 <manufacturerID> ... F7
	if resp[1] == 0x7E && resp[3] == 0x06 && resp[4] == 0x02 {
		fmt.Println("[+] Received Universal Identity Reply.")

		if resp[5] == rolandVendorID {
			fmt.Println("[+] Roland Vendor ID matched (0x41).")
			if len(resp) >= 14 {
				// Roland Identity Reply payloads include Family ID (2 bytes) and Model Number (2 bytes)
				family := resp[6:8]
				model := resp[8:10]
				revision := resp[10:14]
				fmt.Printf("[+] Device Identity Verified:\n    - Family Code: %X\n    - Model Number: %X\n    - Software Revision: %X\n", family, model, revision)
			}
		} else {
			fmt.Printf("[!] Responded by non-Roland device (Vendor ID: 0x%02X).\n", resp[5])
		}
	} else {
		fmt.Println("[!] Received a non-identity SysEx message.")
	}

	return nil
}

// readSysEx reads from an os.File using deadlines, assembling a full SysEx message (0xF0...0xF7)
func (c *linuxMidiClient) readSysEx() ([]byte, error) {
	_ = c.dev.SetReadDeadline(time.Now().Add(c.timeout))
	defer c.dev.SetReadDeadline(time.Time{})

	var buf []byte
	inSysEx := false
	tmp := make([]byte, 128)

	for {
		n, err := c.dev.Read(tmp)
		if err != nil {
			return nil, fmt.Errorf("read error/timeout: %w", err)
		}

		for i := 0; i < n; i++ {
			b := tmp[i]

			// Skip MIDI Realtime status bytes (Active Sensing 0xFE, Timing Clock 0xF8, etc.)
			if b >= 0xF8 && b != RolandEOXByte {
				continue
			}

			if b == RolandHeaderByte {
				inSysEx = true
				buf = []byte{b}
			} else if inSysEx {
				buf = append(buf, b)
				if b == RolandEOXByte {
					return buf, nil
				}
			}
		}
	}
}

// TransceiveSysEx flushes stale data, writes the message, and uses readSysEx to capture the response
func (c *linuxMidiClient) TransceiveSysEx(msg []byte) ([]byte, error) {
	// Flush stale bytes in input buffer
	_ = c.dev.SetReadDeadline(time.Now().Add(c.timeout))
	discard := make([]byte, 256)
	for {
		n, _ := c.dev.Read(discard)
		if n == 0 {
			break
		}
	}

	// Write SysEx command
	if _, err := c.dev.Write(msg); err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}

	// Read returning SysEx payload via ReadSysEx
	return c.readSysEx()
}
func (c *linuxMidiClient) GetActiveKit() (int, error) {
	// Address: Current Kit Number (0x00, 0x00, 0x00, 0x00)
	addr := [4]byte{0x00, 0x00, 0x00, 0x00}

	// MUST request 4 bytes to match the 4-nibble DT1 payload returned by SPD-SX PRO
	size := [4]byte{0x00, 0x00, 0x00, 0x04}
	rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	fmt.Printf("Sending Active Kit RQ1 Query: %X\n", rq1Query)

	resp, err := c.TransceiveSysEx(rq1Query)
	if err != nil {
		log.Fatalf("Failed to query active kit: %v", err)
	}

	fmt.Printf("Received Response (%d bytes): %X\n", len(resp), resp)

	kitNum, err := parseActiveKitResponse(resp)
	if err != nil {
		return -1, fmt.Errorf("Response validation failed: %v", err)
	}

	return kitNum, nil
}
