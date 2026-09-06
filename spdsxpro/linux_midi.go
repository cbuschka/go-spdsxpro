package spdsxpro

import (
	"bytes"
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
	// Device IDs
	DefaultDeviceID byte = 0x10

	CmdRQ1 = 0x11 // Request Data 1
	CmdDT1 = 0x12 // Data Set 1 (Response/Write)

	TotalKits = 200 // SPD-SX PRO supports 200 kits

	// SPD-SX PRO Kit Name is 16 characters long, which requires 32 nibble bytes in SysEx
	KitNameLength     = 16
	KitSubTitleLength = 32
	KitBlockLength    = KitNameLength + KitSubTitleLength // 44 bytes (0x2C)

	// Roland ID is 0x41
	rolandVendorID = 0x41
)

// Universal Non-Realtime SysEx Identity Request (Ping)
// Header: 0xF0, Non-Realtime ID: 0x7E, Target Device ID: 0x7F (All Call), General Info: 0x06, Identity Request: 0x01, EOX: 0xF7
var sysExPing = []byte{0xF0, 0x7E, 0x7F, 0x06, 0x01, 0xF7}

// ModelIDSPDSXPro SPD-SX PRO 5-byte Model ID verified from hardware output
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

func (c *linuxMidiClient) readSysEx() ([]byte, error) {
	_ = c.dev.SetReadDeadline(time.Now().Add(c.timeout))
	defer c.dev.SetReadDeadline(time.Time{})

	var buf []byte
	inSysEx := false
	tmp := make([]byte, 256)

	for {
		n, err := c.dev.Read(tmp)
		if err != nil {
			return nil, fmt.Errorf("read timeout or error: %w", err)
		}

		for i := 0; i < n; i++ {
			b := tmp[i]

			if b >= 0xF8 && b != RolandEOXByte {
				continue
			}

			if b == RolandHeaderByte {
				if inSysEx {
					return nil, fmt.Errorf("new header while in sysex")
				}
				inSysEx = true
				buf = []byte{b}
			} else {
				if inSysEx {
					buf = append(buf, b)
					if b == RolandEOXByte {
						log.Printf("received msg (%d bytes): %X", len(buf), buf)

						return buf, nil
					}
				} else {
					// skipped
				}
			}
		}
	}
}

func (c *linuxMidiClient) TransceiveSysEx(msg []byte) ([]byte, error) {
	// Drain lingering incoming bytes with a quick 5ms deadline
	_ = c.dev.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	discard := make([]byte, 256)
	for {
		n, err := c.dev.Read(discard)
		if n == 0 || err != nil {
			break
		}
	}

	log.Printf("sending msg: %X", msg)

	if _, err := c.dev.Write(msg); err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}

	return c.readSysEx()
}

func (c *linuxMidiClient) GetActiveKit() (int, error) {
	// Address: Current Kit Number (0x00, 0x00, 0x00, 0x00)
	addr := [4]byte{0x00, 0x00, 0x00, 0x00}

	// MUST request 4 bytes to match the 4-nibble DT1 payload returned by SPD-SX PRO
	size := [4]byte{0x00, 0x00, 0x00, 0x04}
	rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

	resp, err := c.TransceiveSysEx(rq1Query)
	if err != nil {
		log.Fatalf("Failed to query active kit: %v", err)
	}

	kitNum, err := parseActiveKitResponse(resp)
	if err != nil {
		return -1, fmt.Errorf("Response validation failed: %v", err)
	}

	return kitNum, nil
}

func parseKitNameResponse(resp []byte) (string, string, error) {
	minLen := 3 + len(ModelIDSPDSXPro) + 1 + 4 + 1 + 1
	if len(resp) < minLen {
		return "", "", fmt.Errorf("payload short (%d bytes)", len(resp))
	}

	if resp[0] != RolandHeaderByte || resp[len(resp)-1] != RolandEOXByte {
		return "", "", errors.New("invalid framing")
	}

	cmdIdx := 3 + len(ModelIDSPDSXPro)
	if resp[cmdIdx] != CmdDT1 {
		return "", "", fmt.Errorf("expected DT1 (0x12), got 0x%02X", resp[cmdIdx])
	}

	checksumIdx := len(resp) - 2
	payloadForChecksum := resp[cmdIdx+1 : checksumIdx]
	if computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
		return "", "", errors.New("checksum mismatch")
	}

	// Data payload sits between 4-byte address and 1-byte checksum
	dataBytes := resp[cmdIdx+5 : checksumIdx]

	if len(dataBytes) < KitNameLength {
		return "", "", fmt.Errorf("expected at least %d data bytes, got %d", KitNameLength, len(dataBytes))
	}

	// Extract Kit Name (First 12 bytes)
	name := cleanASCII(dataBytes[:KitNameLength])

	// Extract Subtitle (Remaining bytes up to offset 44)
	var subTitle string
	if len(dataBytes) > KitNameLength {
		endIdx := len(dataBytes)
		if endIdx > KitBlockLength {
			endIdx = KitBlockLength
		}
		subTitle = cleanASCII(dataBytes[KitNameLength:endIdx])
	}

	return name, subTitle, nil
}

func cleanASCII(b []byte) string {
	var out []byte
	for _, c := range b {
		if c >= 32 && c <= 126 { // Printable ASCII range
			out = append(out, c)
		}
	}
	return string(bytes.TrimSpace(out))
}

// getKitNameAddress computes the 4-byte Roland address for Kit N (1-indexed: 1..200)
func (c *linuxMidiClient) getKitNameAddress(kitNum int) [4]byte {
	idx := kitNum - 1 // 0-based index

	// Each kit increments Byte 2 by 0x02
	stride := idx * 2

	b1 := byte(0x04 + (stride / 128)) // Carry over to B1 after 64 kits
	b2 := byte(stride % 128)          // B2 steps by 0x02 per kit
	b3 := byte(0x00)
	b4 := byte(0x00) // Kit Name sub-offset

	return [4]byte{b1, b2, b3, b4}
}

func (c *linuxMidiClient) GetKitList() ([]Kit, error) {
	var kits []Kit
	// Request 44 bytes total (12 bytes Name + 32 bytes SubTitle)
	size := [4]byte{0x00, 0x00, 0x00, byte(KitBlockLength)}

	for i := 1; i <= TotalKits; i++ {
		addr := c.getKitNameAddress(i)
		rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

		resp, err := c.TransceiveSysEx(rq1Query)
		if err != nil {
			log.Printf("Stopped at kit %d: %v", i, err)
			break
		}

		name, subTitle, err := parseKitNameResponse(resp)
		if err != nil {
			log.Printf("Failed to parse kit %d: %v", i, err)
			continue
		}

		if name == "" {
			name = "<Empty>"
		}

		kits = append(kits, Kit{
			Number:   i,
			Name:     name,
			SubTitle: subTitle,
		})

		time.Sleep(10 * time.Millisecond)
	}

	return kits, nil
}
