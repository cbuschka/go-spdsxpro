package internal

import (
	"errors"
	"fmt"
	"go-spdsxpro/internal/log"
	"go-spdsxpro/types"
	"os"
	"strings"
	"time"
	"unicode"
)

const (
	DeviceIDAll = 0x10 // Default Roland Device ID (Base 17 / 0x10)
	// Device IDs
	DefaultDeviceID byte = 0x10

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

	ParamOffsetKitName     = uint32(0x00000000)
	ParamOffsetKitSubTitle = uint32(0x00000010)

	// Sub-section offsets (B3 position)
	KitCommon  uint32 = 0x00000000 // 00 00 00 00
	KitControl uint32 = 0x00000200 // 00 00 02 00
	KitClick   uint32 = 0x00000300 // 00 00 03 00
	KitMidi    uint32 = 0x00000400 // 00 00 04 00

	KitBaseAddres = uint32(0x04000000)
	KitSize       = uint32(0x00020000)

	OffsetClickMode        uint32 = 0x00000000         // 00 00
	OffsetClickSound       uint32 = 0x00000001         // 00 01
	OffsetClickVolume             = uint32(0x00000006) // 4 nibbles: -601..60 (-INF, -60.0dB..+6.0dB)
	OffsetClickPan                = uint32(0x0000000B) // 4 nibbles: -15..15 (L15..C..R15)
	OffsetClickStartRange1        = uint32(0x0000000E) // Offset within KitClick section
	OffsetClickStartRange2        = uint32(0x00000016) // Offset within KitClick section
	OffsetTempo                   = uint32(0x00000056) // 4-nibble 20.0-260.0 (200-2600)
	OffsetPadLinkTx               = uint32(0x0000000C)
	OffsetPadLinkRx               = uint32(0x0000000D)
)

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
	rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

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

func (c *linuxMidiClient) getKitParamAddress(kitNum int, subsectionOffset uint32, paramOffset uint32) [4]byte {
	idx := kitNum - 1 // 1-based (1..200) -> 0-based (0..199)

	// Base Byte 1 starts at 0x04. Carries over to 0x05 at Kit 65, 0x06 at Kit 129
	b1 := byte(0x04 + (idx / 64))

	// Base Byte 2 increments by 0x02 per kit (wraps every 64 kits at 128/0x80)
	b2 := byte((idx * 2) % 128)

	// Combine section offset and parameter offset cleanly
	totalOffset := subsectionOffset + paramOffset

	// Extract offset byte additions
	offB2 := byte((totalOffset >> 16) & 0x7F)
	offB3 := byte((totalOffset >> 8) & 0x7F)
	offB4 := byte(totalOffset & 0x7F)

	return [4]byte{
		b1,
		b2 + offB2,
		offB3,
		offB4,
	}
}

/*
func (c *linuxMidiClient) getKitParamAddress(kitIdx int, subsectionOffset uint32, paramOffset uint32) [4]byte {

	// Base Address 0x04 0x00 0x00 0x00 in 7-bit linear space:
	kitBaseAddr := uint32(0x04) << 21

	// Kit Stride = 0x02 in Byte 2 position
	// In 7-bit space, shifting left by 14 bits targets Byte 2:
	kitStride := uint32(2) << 14

	// Calculate total address in 7-bit linear space
	totalAddr := kitBaseAddr + (uint32(kitIdx) * kitStride) + subsectionOffset + paramOffset

	// Convert 7-bit linear integer back into 4 discrete 7-bit bytes
	return [4]byte{
		byte((totalAddr >> 21) & 0x7F), // Byte 1 (Carries 0x04 -> 0x05 at Kit 65)
		byte((totalAddr >> 14) & 0x7F), // Byte 2 (Increments 0x00, 0x02, ..., wraps at 0x7E)
		byte((totalAddr >> 7) & 0x7F),  // Byte 3
		byte(totalAddr & 0x7F),         // Byte 4
	}
}
*/

func (c *linuxMidiClient) GetKitList() ([]types.Kit, error) {
	var kits []types.Kit
	// Request 44 bytes total (12 bytes Name + 32 bytes SubTitle)
	size := [4]byte{0x00, 0x00, 0x00, byte(KitBlockLength)}

	for i := 1; i <= TotalKits; i++ {
		addr := c.getKitParamAddress(i, KitCommon, ParamOffsetKitName)
		rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

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
	rq1Name := encodeRQ1(c.deviceID, ModelIDSPDSXPro, nameAddr, nameSize)

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
	rq1Steps := encodeRQ1(c.deviceID, ModelIDSPDSXPro, stepsAddr, stepsSize)

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
	if computeRolandChecksum(payloadForChecksum) != resp[checksumIdx] {
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

	return cleanASCII(decoded), nil
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

// getPadLayerParamAddress constructs the 4-byte Roland address.
func (c *linuxMidiClient) getPadLayerParamAddress(kitIdx int, padIdx int, layer types.PadLayer, subAddrB4 byte) [4]byte {
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
	addr := c.getPadLayerParamAddress(kitIdx, padIdx, layer, 0x05)

	size := [4]byte{0x00, 0x00, 0x00, 0x04}
	rq1Query := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)

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
	addr := c.getPadLayerParamAddress(kitIdx, padIdx, layer, 0x05)

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

func (c *linuxMidiClient) SetKitName(kitNum int, name string) error {
	addr := c.getKitParamAddress(kitNum, KitCommon, ParamOffsetKitName)

	name = keepASCIIOnly(name)

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

func (c *linuxMidiClient) SetKitSubTitle(kitNum int, subTitle string) error {
	addr := c.getKitParamAddress(kitNum, KitCommon, ParamOffsetKitSubTitle)

	// Truncate or pad string to 16 ASCII characters
	padded := make([]byte, 16)
	for i := 0; i < len(padded); i++ {
		if i < len(subTitle) {
			padded[i] = subTitle[i]
		} else {
			padded[i] = 0x20 // Space padding
		}
	}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, padded)
	return c.conn.sendSysEx(sysex)
}

func (c *linuxMidiClient) encodeNibbledUint16(val uint16) []byte {
	return []byte{
		byte((val >> 12) & 0x0F),
		byte((val >> 8) & 0x0F),
		byte((val >> 4) & 0x0F),
		byte(val & 0x0F),
	}
}

func (c *linuxMidiClient) GetKitClickTempo(kitIdx int) (float64, error) {

	addr := c.getKitParamAddress(kitIdx, KitCommon, OffsetTempo)
	size := [4]byte{0x00, 0x00, 0x00, 0x04}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return -1, err
	}

	data := reply[13:17]
	rawVal := uint16(data[0]&0x0F)<<12 |
		uint16(data[1]&0x0F)<<8 |
		uint16(data[2]&0x0F)<<4 |
		uint16(data[3]&0x0F)

	return float64(rawVal) / 10.0, nil
}

func (c *linuxMidiClient) SetKitClickTempo(kitIdx int, bpm float64) error {
	// Scaled value: BPM * 10 (e.g., 120.0 BPM = 1200)
	scaledValue := uint16(bpm * 10.0)

	addr := c.getKitParamAddress(kitIdx, KitCommon, OffsetTempo)
	data := c.encodeNibbledUint16(scaledValue)

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

func decodeNibbledInt(b []byte) int {
	if len(b) < 4 {
		return 0
	}
	return int(b[0]&0x0F)<<12 |
		int(b[1]&0x0F)<<8 |
		int(b[2]&0x0F)<<4 |
		int(b[3]&0x0F)
}

func decodeNibbledUint32(b []byte) uint32 {
	if len(b) < 4 {
		return 0
	}
	return uint32(b[0]&0x0F)<<12 |
		uint32(b[1]&0x0F)<<8 |
		uint32(b[2]&0x0F)<<4 |
		uint32(b[3]&0x0F)
}

func decodeNibbledInt16Signed(b []byte) int16 {
	if len(b) < 4 {
		return 0
	}
	uVal := uint16(b[0]&0x0F)<<12 |
		uint16(b[1]&0x0F)<<8 |
		uint16(b[2]&0x0F)<<4 |
		uint16(b[3]&0x0F)
	return int16(uVal)
}

// GetKitPadLink fetches PadLinkTx and PadLinkRx for a specified kit (1..200) and pad (1..19)
func (c *linuxMidiClient) GetKitPadLinkSend(kitIdx int, padIdx int) (int, error) {
	padOffset := uint32(0x00002000) + uint32(padIdx)*uint32(0x00000100)
	addr := c.getKitParamAddress(kitIdx, padOffset, 0x0000000C)
	size := [4]byte{0x00, 0x00, 0x00, 0x02}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return -1, err
	}

	if len(reply) < 17 {
		return -1, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	return int(reply[13]), err
}

// GetKitPadLink fetches PadLinkTx and PadLinkRx for a specified kit (1..200) and pad (1..19)
func (c *linuxMidiClient) GetKitPadLinkReceive(kitIdx int, padIdx int) (int, error) {
	padOffset := uint32(0x00002000) + uint32(padIdx)*uint32(0x00000100)
	addr := c.getKitParamAddress(kitIdx, padOffset, 0x0000000C)
	size := [4]byte{0x00, 0x00, 0x00, 0x02}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return -1, err
	}

	if len(reply) < 17 {
		return -1, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	return int(reply[14]), err
}

// SetKitPadLink updates both PadLink Tx and Rx simultaneously for a kit and pad
func (c *linuxMidiClient) SetKitPadLinkSend(kitIdx int, padIdx int, tx int) error {
	padOffset := uint32(0x00002000) + uint32(padIdx)*uint32(0x00000100)
	addr := c.getKitParamAddress(kitIdx, padOffset, OffsetPadLinkTx)

	data := []byte{byte(tx)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

func (c *linuxMidiClient) SetKitPadLinkReceive(kitIdx int, padIdx int, rx int) error {
	padOffset := uint32(0x00002000) + uint32(padIdx)*uint32(0x00000100)
	addr := c.getKitParamAddress(kitIdx, padOffset, OffsetPadLinkRx)

	data := []byte{byte(rx)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

// SetKitClickStartRange sets the Click Start Pad Range1 (0..19) for a given kit.
func (c *linuxMidiClient) SetKitClickStartRangeFrom(kitIdx int, from int) error {
	if from > 19 {
		return fmt.Errorf("pad range %d out of bounds (0..19)", from)
	}

	// Calculate target address within KitClick section (KitClick = 0x00000300)
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickStartRange2)

	data := []byte{byte(from)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

// SetKitClickStartRange sets the Click Start Pad Range1 (0..19) for a given kit.
func (c *linuxMidiClient) SetKitClickStartRangeTo(kitIdx int, to int) error {
	if to > 19 {
		return fmt.Errorf("pad range %d out of bounds (0..19)", to)
	}

	// Calculate target address within KitClick section (KitClick = 0x00000300)
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickStartRange1)

	data := []byte{byte(to)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

// SetKitClickMode sets Click Mode for a kit (0 = PRESET, 1 = WAVE, 2 = CLICK-TRACK)
func (c *linuxMidiClient) SetKitClickMode(kitNum int, mode int) error {
	if mode < 0 || mode > 2 {
		return fmt.Errorf("click mode %d out of bounds (0..2)", mode)
	}

	addr := c.getKitParamAddress(kitNum, KitClick, OffsetClickMode)
	data := []byte{byte(mode)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

// GetKitClickMode retrieves the Click Mode for a kit
func (c *linuxMidiClient) GetKitClickMode(kitIdx int) (int, error) {
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickMode)
	size := [4]byte{0x00, 0x00, 0x00, 0x01}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return 0, err
	}

	if len(reply) < 16 {
		return 0, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	return int(reply[13]), nil
}

// GetKitClickVolume retrieves the Click Volume (-601 = -INF, -600..60 = -60.0dB..+6.0dB)
func (c *linuxMidiClient) GetKitClickVolume(kidIdx int) (int, error) {
	addr := c.getKitParamAddress(kidIdx, KitClick, OffsetClickVolume)
	size := [4]byte{0x00, 0x00, 0x00, 0x04}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return 0, err
	}

	if len(reply) < 17 {
		return 0, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	data := reply[13:17]
	rawVal := uint16(data[0]&0x0F)<<12 |
		uint16(data[1]&0x0F)<<8 |
		uint16(data[2]&0x0F)<<4 |
		uint16(data[3]&0x0F)

	return int(rawVal), nil
}

// SetKitClickPan sets the Click Pan position (-15 = L15, 0 = Center, 15 = R15)
func (c *linuxMidiClient) SetKitClickPan(kitIdx int, pan int8) error {
	if pan < -15 || pan > 15 {
		return fmt.Errorf("pan %d out of bounds (-15..15)", pan)
	}

	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickPan)
	data := c.encodeNibbledUint16(uint16(int16(pan)))

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

// GetKitClickPan retrieves the Click Pan position (-15..15)
func (c *linuxMidiClient) GetKitClickPan(kitIdx int) (int8, error) {
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickPan)
	size := [4]byte{0x00, 0x00, 0x00, 0x04}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return 0, err
	}

	if len(reply) < 17 {
		return 0, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	data := reply[13:17]
	rawVal := uint16(data[0]&0x0F)<<12 |
		uint16(data[1]&0x0F)<<8 |
		uint16(data[2]&0x0F)<<4 |
		uint16(data[3]&0x0F)

	return int8(int16(rawVal)), nil
}

func keepASCIIOnly(s string) string {
	var builder strings.Builder
	builder.Grow(len(s))

	for _, r := range s {
		if r <= unicode.MaxASCII {
			builder.WriteRune(r)
		}
	}

	return builder.String()
}

func (c *linuxMidiClient) SetKitClickSound(kitIdx int, sound int) error {
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickSound)
	data := []byte{byte(sound)}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}

func (c *linuxMidiClient) GetKitClickSound(kitIdx int) (int, error) {
	addr := c.getKitParamAddress(kitIdx, KitClick, OffsetClickSound)
	size := [4]byte{0x00, 0x00, 0x00, 0x01}

	sysex := encodeRQ1(c.deviceID, ModelIDSPDSXPro, addr, size)
	reply, err := c.conn.TransceiveSysEx(sysex)
	if err != nil {
		return 0, err
	}

	if len(reply) < 16 {
		return 0, fmt.Errorf("unexpected reply length: %d bytes", len(reply))
	}

	return int(reply[13]), nil
}

// SetKitClickVolume sets Click Volume (-600 to +60, corresponding to -60.0 dB to +6.0 dB)
func (c *linuxMidiClient) SetKitClickVolume(kitNum int, volume int) error {
	if volume < -600 || volume > 60 {
		return fmt.Errorf("click volume %d out of bounds (-600..60)", volume)
	}

	addr := c.getKitParamAddress(kitNum, KitClick, OffsetClickVolume)
	if volume < 0 {
		volume = volume + 0x10000
	}

	// Split into 4 Roland 7-bit nibbles
	data := []byte{
		byte((volume >> 12) & 0x0F),
		byte((volume >> 8) & 0x0F),
		byte((volume >> 4) & 0x0F),
		byte(volume & 0x0F),
	}

	sysex := c.conn.encodeDT1(c.deviceID, ModelIDSPDSXPro, addr, data)
	return c.conn.sendSysEx(sysex)
}
