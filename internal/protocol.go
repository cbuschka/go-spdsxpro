package internal

import "bytes"

const (
	RolandHeaderByte = 0xF0
	RolandEOXByte    = 0xF7
	RolandVendorID   = 0x41

	CmdRQ1 = 0x11 // Request Data 1
	CmdDT1 = 0x12 // Data Set 1 (Response/Write)
)

// Universal Non-Realtime SysEx Identity Request (Ping)
// Header: 0xF0, Non-Realtime ID: 0x7E, Target Device ID: 0x7F (All Call), General Info: 0x06, Identity Request: 0x01, EOX: 0xF7
var sysExPing = []byte{0xF0, 0x7E, 0x7F, 0x06, 0x01, 0xF7}

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

// computeRolandChecksum calculates the Roland 7-bit checksum:
// 128 - ((sum of address + size/data bytes) % 128)
func computeRolandChecksum(data []byte) byte {
	var sum int
	for _, b := range data {
		sum += int(b)
	}
	return byte((128 - (sum % 128)) & 0x7F)
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
