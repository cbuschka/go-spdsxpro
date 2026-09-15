package internal

import (
	"errors"
	"fmt"
	"go-spdsxpro/internal/log"
	"os"
	"time"
)

// ModelIDSPDSXPro SPD-SX PRO 5-byte Model ID verified from hardware output
var ModelIDSPDSXPro = []byte{0x00, 0x00, 0x00, 0x00, 0x16}

type Connection struct {
	dev     *os.File
	timeout time.Duration
}

func (c *Connection) readSysEx() ([]byte, error) {
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
						log.Debugf("received msg (%d bytes): %X", len(buf), buf)

						return buf, nil
					}
				} else {
					// skipped
				}
			}
		}
	}
}

func (c *Connection) TransceiveSysEx(msg []byte) ([]byte, error) {
	// Drain lingering incoming bytes with a quick 5ms deadline
	_ = c.dev.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
	discard := make([]byte, 256)
	for {
		n, err := c.dev.Read(discard)
		if n == 0 || err != nil {
			break
		}
	}

	log.Debugf("sending msg: %X", msg)

	_ = c.dev.SetWriteDeadline(time.Now().Add(c.timeout))
	if _, err := c.dev.Write(msg); err != nil {
		return nil, fmt.Errorf("write error: %w", err)
	}

	return c.readSysEx()
}

// SendSysEx validates and transmits a SysEx byte slice to the MIDI device.
func (conn *Connection) sendSysEx(msg []byte) error {
	if len(msg) < 4 {
		return errors.New("sysex message too short")
	}

	// 1. Verify basic SysEx framing
	if msg[0] != RolandHeaderByte {
		return fmt.Errorf("invalid header byte: expected 0xF0, got 0x%02X", msg[0])
	}
	if msg[len(msg)-1] != RolandEOXByte {
		return fmt.Errorf("invalid end byte: expected 0xF7, got 0x%02X", msg[len(msg)-1])
	}

	log.Debugf("sending msg: %X", msg)

	// 2. Write bytes out to the MIDI transport
	n, err := conn.dev.Write(msg)
	if err != nil {
		return fmt.Errorf("failed to write SysEx to MIDI port: %w", err)
	}

	if n != len(msg) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(msg))
	}

	return nil
}

func (c *Connection) verifyResponse(resp []byte) error {
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

func (c *Connection) Close() error {
	if c.dev != nil {
		return c.dev.Close()
	}
	defer func() {
		c.dev = nil
	}()

	return nil
}

// BuildRolandDT1 creates a Roland Data Set 1 (DT1) SysEx message []byte.
// deviceID: typically 0x10 (17 decimal)
// modelID: slice of bytes defining the device model (e.g., []byte{0x00, 0x00, 0x00, 0x??})
// address: 4-byte target memory address []byte
// data: payload bytes to write
func (c *Connection) encodeDT1(deviceID byte, modelID []byte, addr [4]byte, data []byte) []byte {
	// FIXME handle data too large
	msg := []byte{RolandHeaderByte, RolandVendorID, deviceID}
	msg = append(msg, modelID...)
	msg = append(msg, CmdDT1)

	payload := append(addr[:], data[:]...)
	checksum := computeRolandChecksum(payload)

	msg = append(msg, payload...)
	msg = append(msg, checksum, RolandEOXByte)
	return msg
}
