package internal

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"go-spdsxpro/internal/log"
	"go-spdsxpro/types"
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

	TotalSetlists     = 32 // SPD-SX PRO supports up to 32 Setlists
	SetlistNameLength = 12 // Setlist Name is 12 ASCII bytes
	SetlistMaxSteps   = 32 // Up to 32 kit steps per setlist
)

// Universal Non-Realtime SysEx Identity Request (Ping)
// Header: 0xF0, Non-Realtime ID: 0x7E, Target Device ID: 0x7F (All Call), General Info: 0x06, Identity Request: 0x01, EOX: 0xF7
var sysExPing = []byte{0xF0, 0x7E, 0x7F, 0x06, 0x01, 0xF7}

func (c *linuxMidiClient) parseActiveKitResponse(resp []byte) (int, error) {
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
	if c.conn.computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
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
	conn     *Connection
	timeout  time.Duration
}

func (c *linuxMidiClient) Connect() error {
	dev, err := os.OpenFile(c.path, os.O_RDWR, 0)
	if err != nil {
		return err
	}

	c.conn = &Connection{dev: dev, timeout: c.timeout}

	return nil
}

func (c *linuxMidiClient) Ping() error {
	reply, err := c.conn.TransceiveSysEx(sysExPing)
	if err != nil {
		return err
	}

	return c.conn.verifyResponse(reply)

}

func (c *linuxMidiClient) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	}

	return nil
}

func (c *linuxMidiClient) GetActiveKit() (int, error) {
	// Address: Current Kit Number (0x00, 0x00, 0x00, 0x00)
	addr := [4]byte{0x00, 0x00, 0x00, 0x00}

	// MUST request 4 bytes to match the 4-nibble DT1 payload returned by SPD-SX PRO
	size := [4]byte{0x00, 0x00, 0x00, 0x04}
	rq1Query := c.conn.encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

	resp, err := c.conn.TransceiveSysEx(rq1Query)
	if err != nil {
		log.Fatalf("Failed to query active kit: %v", err)
	}

	kitNum, err := c.parseActiveKitResponse(resp)
	if err != nil {
		return -1, fmt.Errorf("Response validation failed: %v", err)
	}

	return kitNum, nil
}

func (c *linuxMidiClient) parseKitNameResponse(resp []byte) (string, string, error) {
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
	if c.conn.computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
		return "", "", errors.New("checksum mismatch")
	}

	// Data payload sits between 4-byte address and 1-byte checksum
	dataBytes := resp[cmdIdx+5 : checksumIdx]

	if len(dataBytes) < KitNameLength {
		return "", "", fmt.Errorf("expected at least %d data bytes, got %d", KitNameLength, len(dataBytes))
	}

	// Extract Kit Name (First 12 bytes)
	name := c.cleanASCII(dataBytes[:KitNameLength])

	// Extract Subtitle (Remaining bytes up to offset 44)
	var subTitle string
	if len(dataBytes) > KitNameLength {
		endIdx := len(dataBytes)
		if endIdx > KitBlockLength {
			endIdx = KitBlockLength
		}
		subTitle = c.cleanASCII(dataBytes[KitNameLength:endIdx])
	}

	return name, subTitle, nil
}

func (c *linuxMidiClient) cleanASCII(b []byte) string {
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

	/*
		idx := kitNum - 1 // 0-based index

		// Each kit increments Byte 2 by 0x02
		stride := idx * 2

		b1 := byte(0x04 + (stride / 128)) // Carry over to B1 after 64 kits
		b2 := byte(stride % 128)          // B2 steps by 0x02 per kit
		b3 := byte(0x00)
		b4 := byte(0x00) // Kit Name sub-offset

		return [4]byte{b1, b2, b3, b4}
	*/

	baseAddr := 0x04000000 + uint32(kitNum-1)*0x00020000
	nameOffset := uint32(0x00000000) // Kit Name parameter offset within Kit structure

	addr := baseAddr + nameOffset

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, addr)
	return [4]byte(b)
}

func (c *linuxMidiClient) GetKitList() ([]types.Kit, error) {
	var kits []types.Kit
	// Request 44 bytes total (12 bytes Name + 32 bytes SubTitle)
	size := [4]byte{0x00, 0x00, 0x00, byte(KitBlockLength)}

	for i := 1; i <= TotalKits; i++ {
		addr := c.getKitNameAddress(i)
		rq1Query := c.conn.encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

		resp, err := c.conn.TransceiveSysEx(rq1Query)
		if err != nil {
			return nil, fmt.Errorf("Stopped at kit %d: %v", i, err)
		}

		name, subTitle, err := c.parseKitNameResponse(resp)
		if err != nil {
			return nil, fmt.Errorf("Failed to parse kit %d: %v", i, err)
		}

		if name == "" {
			name = "<Empty>"
		}

		kits = append(kits, types.Kit{
			Number:   i,
			Name:     name,
			SubTitle: subTitle,
		})

		time.Sleep(10 * time.Millisecond)
	}

	return kits, nil
}

// GetSetlists queries and returns all available setlists
func (c *linuxMidiClient) GetSetlistList() ([]types.Setlist, error) {
	var setlists []types.Setlist

	for i := 1; i <= TotalSetlists; i++ {
		setlist, err := c.GetSetlist(i)
		if err != nil {
			return nil, fmt.Errorf("Error fetching setlist %d: %v", i, err)
		}

		setlists = append(setlists, *setlist)
		time.Sleep(10 * time.Millisecond)
	}

	return setlists, nil
}

func (c *linuxMidiClient) GetSetlist(setlistNum int) (*types.Setlist, error) {
	if setlistNum < 1 || setlistNum > TotalSetlists {
		return nil, fmt.Errorf("setlist ID out of bounds (1..%d)", TotalSetlists)
	}

	// 1. Query Setlist Name (12 bytes at sub-offset 0x00)
	nameAddr := c.getSetlistAddress(setlistNum)
	nameSize := [4]byte{0x00, 0x00, 0x00, 0x18}
	rq1Name := c.conn.encodeRQ1(c.deviceID, ModelIDSPDSXPro, nameAddr, nameSize)

	respName, err := c.conn.TransceiveSysEx(rq1Name)
	if err != nil {
		return nil, fmt.Errorf("failed to query setlist %d name: %w", setlistNum, err)
	}

	name, err := c.parseSetNameResponse(respName)
	if err != nil {
		return nil, fmt.Errorf("parsing setlist %d name failed: %w", setlistNum, err)
	}

	// 2. Query Setlist Kit Steps (32 steps * 4 bytes = 128 bytes at sub-offset B4 = 0x10)
	stepsAddr := c.getSetlistStepsAddress(setlistNum)
	stepsSize := [4]byte{0x00, 0x00, 0x01, 0x00}
	rq1Steps := c.conn.encodeRQ1(c.deviceID, ModelIDSPDSXPro, stepsAddr, stepsSize)

	respSteps, err := c.conn.TransceiveSysEx(rq1Steps)
	if err != nil {
		return nil, fmt.Errorf("failed to query setlist %d steps: %w", setlistNum, err)
	}

	steps, err := c.parseSetlistSteps(respSteps)
	if err != nil {
		return nil, fmt.Errorf("failed to parse setlist %d steps: %w", setlistNum, err)
	}

	return &types.Setlist{
		ID:    setlistNum,
		Name:  name,
		Steps: steps,
	}, nil
}

// parseSetlistSteps decodes 4-nibble encoded 16-bit kit numbers from the step payload
func (c *linuxMidiClient) parseSetlistSteps(resp []byte) ([]types.SetlistStep, error) {
	minLen := 3 + len(ModelIDSPDSXPro) + 1 + 4 + 1 + 1
	if len(resp) < minLen {
		return nil, fmt.Errorf("payload short (%d bytes)", len(resp))
	}

	cmdIdx := 3 + len(ModelIDSPDSXPro)
	checksumIdx := len(resp) - 2
	dataBytes := resp[cmdIdx+5 : checksumIdx]

	// 16-byte header offset (4 metadata slots of 4 nibbles each)
	const stepHeaderOffset = 16
	if len(dataBytes) < stepHeaderOffset {
		return nil, fmt.Errorf("payload too short for step header (got %d bytes)", len(dataBytes))
	}

	stepPayload := dataBytes[stepHeaderOffset:]

	var steps []types.SetlistStep

	// Each Kit ID is encoded as 4 nibble bytes: [d0, d1, d2, d3]
	for i := 0; i+3 < len(stepPayload); i += 4 {
		hardwareStepNum := (i / 4)

		kitID := (int(stepPayload[i]) << 12) |
			(int(stepPayload[i+1]) << 8) |
			(int(stepPayload[i+2]) << 4) |
			int(stepPayload[i+3])

		if kitID > 0 && kitID <= TotalKits {
			steps = append(steps, types.SetlistStep{
				StepNumber: hardwareStepNum,
				KitNumber:  kitID,
			})
		}
	}

	return steps, nil
}

// parseSetNameResponse extracts the nibble-encoded Setlist name from a DT1 SysEx response
func (c *linuxMidiClient) parseSetNameResponse(resp []byte) (string, error) {
	minLen := 3 + len(ModelIDSPDSXPro) + 1 + 4 + 1 + 1
	if len(resp) < minLen {
		return "", fmt.Errorf("payload short (%d bytes)", len(resp))
	}

	if resp[0] != RolandHeaderByte || resp[len(resp)-1] != RolandEOXByte {
		return "", errors.New("invalid framing")
	}

	cmdIdx := 3 + len(ModelIDSPDSXPro)
	if resp[cmdIdx] != CmdDT1 {
		return "", fmt.Errorf("expected DT1 (0x12), got 0x%02X", resp[cmdIdx])
	}

	checksumIdx := len(resp) - 2
	payloadForChecksum := resp[cmdIdx+1 : checksumIdx]
	if c.conn.computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
		return "", errors.New("checksum mismatch")
	}

	// Data payload sits between 4-byte address and 1-byte checksum
	dataBytes := resp[cmdIdx+5 : checksumIdx]

	// Check if data is 4-bit nibble packed
	var decoded []byte
	if len(dataBytes)%2 == 0 {
		// Combine pairs of nibbles: (high_nibble << 4) | low_nibble
		for i := 0; i < len(dataBytes); i += 2 {
			charByte := (dataBytes[i] << 4) | (dataBytes[i+1] & 0x0F)
			decoded = append(decoded, charByte)
		}
	} else {
		// Fallback for raw byte strings
		decoded = dataBytes
	}

	return c.cleanASCII(decoded), nil
}

// getSetlistAddress calculates the 4-byte Roland address for Setlist N (1..32)
func (c *linuxMidiClient) getSetlistAddress(setlistNum int) [4]byte {
	idx := setlistNum - 1 // 0-based index (0..31)

	b2 := byte(idx / 8)          // Bank 0..3: 0x00, 0x01, 0x02, 0x03
	b3 := byte((idx % 8) * 0x10) // 0x00, 0x10, 0x20, 0x30, 0x40, 0x50, 0x60, 0x70

	return [4]byte{
		0x03, // Block 03
		b2,
		b3,
		0x00, // Name offset (0x00)
	}
}

// getSetlistStepsAddress calculates the address for step data of Setlist N (1..32)
func (c *linuxMidiClient) getSetlistStepsAddress(setlistNum int) [4]byte {
	addr := c.getSetlistAddress(setlistNum)
	addr[3] = 0x10 // Step offset is 0x10 relative to the setlist base address
	return addr
}

// getPadParamAddress constructs the 4-byte Roland address.
func (c *linuxMidiClient) getPadParamAddress(kitIdx int, padIdx int, layer types.PadLayer, subAddrB4 byte) [4]byte {
	// Kit Base Offset (Bytes 1 & 2)
	kitStride := kitIdx * 2
	b1 := byte(0x04 + (kitStride / 128))
	b2 := byte(kitStride % 128)

	// Pad & Layer Offset (Bytes 3 & 4)
	// Each Pad strides by 0x0200 (Pad 1: 0x40/0x41, Pad 2: 0x42/0x43, etc.)
	b3 := byte(layer) + byte(padIdx*2)
	b4 := subAddrB4

	return [4]byte{b1, b2, b3, b4}
}

// GetPadLayerVolume fetches volume for a specific layer on a pad.
func (c *linuxMidiClient) GetPadLayerVolume(kitIdx int, padIdx int, layer types.PadLayer) (int, error) {
	// Sub-address 0x05 holds volume within the layer block
	addr := c.getPadParamAddress(kitIdx, padIdx, layer, 0x05)

	size := [4]byte{0x00, 0x00, 0x00, 0x04}
	rq1Query := c.conn.encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

	resp, err := c.conn.TransceiveSysEx(rq1Query)
	if err != nil {
		return 0, fmt.Errorf("failed to query pad volume: %w", err)
	}

	return c.extractVolumeFromDT1(resp)
}

// extractVolumeFromDT1 parses 4-nibble signed integer payloads.
func (c *linuxMidiClient) extractVolumeFromDT1(resp []byte) (int, error) {
	minLen := 3 + len(ModelIDSPDSXPro) + 1 + 4 + 1 + 1 + 1
	if len(resp) < minLen {
		return 0, fmt.Errorf("payload short (%d bytes)", len(resp))
	}

	cmdIdx := 3 + len(ModelIDSPDSXPro)
	addrIdx := cmdIdx + 1
	dataStart := addrIdx + 4
	checksumIdx := len(resp) - 2

	dataBytes := resp[dataStart:checksumIdx]
	if len(dataBytes) < 4 {
		return 0, fmt.Errorf("expected 4 data bytes, got %d", len(dataBytes))
	}

	// Reconstruct unsigned 16-bit integer from 4 nibbles
	rawVal := (int(dataBytes[0]&0x0F) << 12) |
		(int(dataBytes[1]&0x0F) << 8) |
		(int(dataBytes[2]&0x0F) << 4) |
		int(dataBytes[3]&0x0F)

	// Handle 16-bit signed negative values (2's complement extension)
	if rawVal&0x8000 != 0 {
		rawVal = rawVal - 0x10000
	}

	return rawVal, nil
}

func (c *linuxMidiClient) SetPadLayerVolume(kitIdx int, padIdx int, layer types.PadLayer, rawVal int) error {
	addr := c.getPadParamAddress(kitIdx, padIdx, layer, 0x05)

	// Ensure 16-bit range
	if rawVal < 0 {
		rawVal = rawVal + 0x10000
	}

	// Split into 4 Roland 7-bit nibbles
	data := []byte{
		byte((rawVal >> 12) & 0x0F),
		byte((rawVal >> 8) & 0x0F),
		byte((rawVal >> 4) & 0x0F),
		byte(rawVal & 0x0F),
	}

	dt1Msg := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(dt1Msg)
}

// encodeNibbleASCII converts an ASCII string into a Roland 4-bit nibble byte slice.
// If paddedLen > 0, it right-pads the string with spaces to fill the exact byte count.
func (c *linuxMidiClient) encodeNibbleASCII(str string, paddedLen int) []byte {
	// Truncate if string exceeds expected length
	if paddedLen > 0 && len(str) > paddedLen {
		str = str[:paddedLen]
	}

	// Pad with spaces to match expected byte length
	if paddedLen > 0 && len(str) < paddedLen {
		str = fmt.Sprintf("%-*s", paddedLen, str)
	}

	encoded := make([]byte, 0, len(str)*2)
	for _, ch := range []byte(str) {
		// High nibble (bits 7-4), Low nibble (bits 3-0)
		encoded = append(encoded, byte((ch>>4)&0x0F), byte(ch&0x0F))
	}

	return encoded
}

func kitNameAddress(kitNum int) []byte {

	baseAddr := 0x04000000 + uint32(kitNum-1)*0x00020000
	nameOffset := uint32(0x00000000) // Kit Name parameter offset within Kit structure

	addr := baseAddr + nameOffset

	b := make([]byte, 4)
	binary.BigEndian.PutUint32(b, addr)
	return b
}

func (c *linuxMidiClient) SetKitName(kitNum int, name string) error {
	if kitNum < 1 || kitNum > TotalKits {
		return fmt.Errorf("kit number %d out of bounds (1..%d)", kitNum, TotalKits)
	}

	addr := c.getKitNameAddress(kitNum)

	// Truncate or pad string to 16 ASCII characters (Roland standard kit name length)
	padded := make([]byte, 16)
	for i := 0; i < len(padded); i++ {
		if i < len(name) {
			padded[i] = name[i]
		} else {
			padded[i] = 0x20 // Space padding
		}
	}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, padded)
	return c.conn.sendSysEx(sysex)
}
