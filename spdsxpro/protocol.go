package spdsxpro

import (
	"bytes"
	"fmt"
)

// SysEx & MIDI Constants
const (
	SysExStart byte = 0xF0
	SysExEnd   byte = 0xF7

	// Roland & Universal SysEx Identifiers
	RolandManufacturerID    byte = 0x41 // Roland Corporation
	UniversalNonRealtime    byte = 0x7E // Universal Non-Realtime SysEx Header
	IdentityReplyCommand    byte = 0x06 // General Information Sub-ID1
	IdentityReplySubCommand byte = 0x02 // Identity Reply Sub-ID2

	// Device IDs
	DefaultDeviceID   byte = 0x10
	BroadcastDeviceID byte = 0x7F

	// Roland SPD-SX PRO Device Family Identification
	SPDSXPROFamilyLSB byte = 0x6B
	SPDSXPROFamilyMSB byte = 0x02

	CommandDataRequest byte = 0x11 // RQ1: Request Data
	CommandDataSet     byte = 0x12 // DT1: Set/Receive Data
)

// Actual 5-Byte Model ID used by SPD-SX PRO SysEx engine
var SPDSXPROModelIDBytes = []byte{0x00, 0x00, 0x00, 0x00, 0x16}

type SysExMessage struct {
	DeviceID       byte
	ManufacturerID byte
	FamilyCode     uint16
	FamilyMember   uint16
	SoftwareVer    [4]byte
	RawPayload     []byte
}

func BuildPingCommand(deviceID byte) []byte {
	return []byte{
		SysExStart,
		UniversalNonRealtime,
		deviceID,
		0x06, // Sub-ID1: General Information
		0x01, // Sub-ID2: Identity Request
		SysExEnd,
	}
}

// DecodeAndValidatePingReply validates Identity Reply messages
func DecodeAndValidatePingReply(raw []byte, expectedDeviceID byte) (*SysExMessage, error) {
	startIdx := bytes.IndexByte(raw, SysExStart)
	if startIdx == -1 {
		return nil, fmt.Errorf("no SysEx start byte (0xF0) found")
	}

	frame := raw[startIdx:]
	endIdx := bytes.IndexByte(frame, SysExEnd)
	if endIdx == -1 {
		return nil, fmt.Errorf("incomplete SysEx frame: missing 0xF7")
	}
	frame = frame[:endIdx+1]

	if len(frame) < 15 {
		return nil, fmt.Errorf("invalid response length: got %d bytes", len(frame))
	}

	if frame[1] != UniversalNonRealtime {
		return nil, fmt.Errorf("not a Universal SysEx message: got 0x%02X", frame[1])
	}

	replyDeviceID := frame[2]
	if expectedDeviceID != BroadcastDeviceID && replyDeviceID != expectedDeviceID {
		return nil, fmt.Errorf("device ID mismatch: got 0x%02X, expected 0x%02X", replyDeviceID, expectedDeviceID)
	}

	if frame[3] != IdentityReplyCommand || frame[4] != IdentityReplySubCommand {
		return nil, fmt.Errorf("not an identity reply: got Sub-IDs [0x%02X, 0x%02X]", frame[3], frame[4])
	}

	manufacturerID := frame[5]
	if manufacturerID != RolandManufacturerID {
		return nil, fmt.Errorf("non-Roland device detected: manufacturer ID 0x%02X", manufacturerID)
	}

	familyCode := uint16(frame[6]) | (uint16(frame[7]) << 8)
	familyMember := uint16(frame[8]) | (uint16(frame[9]) << 8)

	expectedFamily := uint16(SPDSXPROFamilyLSB) | (uint16(SPDSXPROFamilyMSB) << 8)
	if familyCode != expectedFamily {
		return nil, fmt.Errorf("device family mismatch: got 0x%04X, expected 0x%04X", familyCode, expectedFamily)
	}

	var swVer [4]byte
	copy(swVer[:], frame[10:14])

	return &SysExMessage{
		DeviceID:       replyDeviceID,
		ManufacturerID: manufacturerID,
		FamilyCode:     familyCode,
		FamilyMember:   familyMember,
		SoftwareVer:    swVer,
		RawPayload:     bytes.Clone(frame),
	}, nil
}

// Calculate Roland Checksum strictly over Address + Size / Payload
func CalculateChecksum(data []byte) byte {
	var sum int
	for _, b := range data {
		sum += int(b)
	}
	remainder := sum % 128
	if remainder == 0 {
		return 0
	}
	return byte(128 - remainder)
}

// BuildGetActiveKitCommand creates the exact 18-byte RQ1 frame required by the SPD-SX PRO.
func BuildGetActiveKitCommand(deviceID byte) ([]byte, error) {
	// Construct payload for checksum: Address (00 00 00 00) + Size (00 00 00 02)
	chkPayload := []byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x02}

	// Calculate Roland Checksum strictly over Address + Size (results in 0x7E)
	checksum := CalculateChecksum(chkPayload)

	// Explicit 18-byte array construction to eliminate model ID byte length bugs
	frame := []byte{
		SysExStart,             // 0: 0xF0
		RolandManufacturerID,   // 1: 0x41
		deviceID,               // 2: 0x10 (or 0x7F broadcast)
		0x00, 0x00, 0x00, 0x16, // 3..6: 4-Byte Model ID for SPD-SX PRO
		CommandDataRequest,     // 7: 0x11 (RQ1)
		0x00, 0x00, 0x00, 0x00, // 8..11: Active Kit Address
		0x00, 0x00, 0x00, 0x02, // 12..15: Size (2 Bytes)
		checksum, // 16: 0x7E
		SysExEnd, // 17: 0xF7
	}

	return frame, nil
}

// DecodeActiveKitReply searches the raw serial buffer for the DT1 frame and extracts active kit
func DecodeActiveKitReply(raw []byte, expectedDeviceID byte) (int, error) {
	startIdx := bytes.IndexByte(raw, SysExStart)
	if startIdx == -1 {
		return 0, fmt.Errorf("no SysEx start byte (0xF0) found")
	}

	frame := raw[startIdx:]
	endIdx := bytes.IndexByte(frame, SysExEnd)
	if endIdx == -1 {
		return 0, fmt.Errorf("incomplete SysEx frame: missing 0xF7")
	}
	frame = frame[:endIdx+1]

	// Check frame header length
	if len(frame) < 17 {
		return 0, fmt.Errorf("frame too short (%d bytes)", len(frame))
	}

	if frame[1] != RolandManufacturerID {
		return 0, fmt.Errorf("invalid manufacturer 0x%02X", frame[1])
	}

	if expectedDeviceID != BroadcastDeviceID && frame[2] != expectedDeviceID {
		return 0, fmt.Errorf("device ID mismatch: got 0x%02X, expected 0x%02X", frame[2], expectedDeviceID)
	}

	// Verify 5-Byte Model ID: 00 00 00 00 16
	if !bytes.Equal(frame[3:8], SPDSXPROModelIDBytes) {
		return 0, fmt.Errorf("model ID mismatch: got %X, expected %X", frame[3:8], SPDSXPROModelIDBytes)
	}

	// Verify Command byte is DT1 (0x12) at offset 8
	if frame[8] != CommandDataSet {
		return 0, fmt.Errorf("unexpected command byte: got 0x%02X (expected 0x12 DT1)", frame[8])
	}

	// Verify Address is 00 00 00 00 (offsets 9..12)
	if frame[9] != 0x00 || frame[10] != 0x00 || frame[11] != 0x00 || frame[12] != 0x00 {
		return 0, fmt.Errorf("not an Active Kit state address")
	}

	// Payload data bytes are at offsets 13 and 14
	highByte := int(frame[13])
	lowByte := int(frame[14])

	// Nibble decoding: Kit Number = (High * 16) + Low + 1
	zeroBasedIndex := (highByte * 16) + lowByte
	return zeroBasedIndex + 1, nil
}
