package internal

import (
	"bytes"
	"testing"
)

func TestComputeRolandChecksum(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected byte
	}{
		{
			name:     "Empty slice",
			input:    []byte{},
			expected: 0x00, // 128 - (0 % 128) = 128 -> 128 & 0x7F = 0
		},
		{
			name:     "Single zero byte",
			input:    []byte{0x00},
			expected: 0x00, // 128 - (0 % 128) = 128 -> 0x80 & 0x7F = 0
		},
		{
			name:     "Single non-zero byte",
			input:    []byte{0x10},
			expected: 0x70, // 128 - (16 % 128) = 112 (0x70)
		},
		{
			name:     "Standard Roland System Exclusive message bytes",
			input:    []byte{0x10, 0x00, 0x00, 0x00}, // Address sum = 16
			expected: 0x70,                           // 128 - 16 = 112 (0x70)
		},
		{
			name:     "Sum exactly equals 128 (Edge Case)",
			input:    []byte{0x40, 0x40}, // Sum = 64 + 64 = 128
			expected: 0x00,               // 128 - (128 % 128) = 128 -> 0x80 & 0x7F = 0
		},
		{
			name:     "Sum wraps multiple times beyond 128",
			input:    []byte{0x7F, 0x7F, 0x7F}, // Sum = 127 + 127 + 127 = 381; 381 % 128 = 125
			expected: 0x03,                     // 128 - 125 = 3
		},
		{
			name:     "Typical Patch Address + Data payload",
			input:    []byte{0x01, 0x00, 0x00, 0x05, 0x20, 0x40}, // Sum = 1 + 5 + 32 + 64 = 102
			expected: 0x1A,                                       // 128 - 102 = 26 (0x1A)
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := computeRolandChecksum(tt.input)
			if got != tt.expected {
				t.Errorf("computeRolandChecksum(%v) = 0x%02X; want 0x%02X", tt.input, got, tt.expected)
			}
		})
	}
}

func TestEncodeRQ1(t *testing.T) {
	tests := []struct {
		name     string
		deviceID byte
		modelID  []byte
		addr     [4]byte
		size     [4]byte
		expected []byte
	}{
		{
			name:     "Standard GS/GM Request (Single model byte)",
			deviceID: 0x10,
			modelID:  []byte{0x42}, // e.g., Roland GS Model ID
			addr:     [4]byte{0x40, 0x00, 0x00, 0x00},
			size:     [4]byte{0x00, 0x00, 0x00, 0x04},
			// Payload sum: 0x40 + 0x04 = 0x44 (68)
			// Checksum: 128 - 68 = 60 (0x3C)
			expected: []byte{
				0xF0, 0x41, 0x10, // Header, Vendor, Device
				0x42,                   // Model ID
				0x11,                   // CmdRQ1
				0x40, 0x00, 0x00, 0x00, // Address
				0x00, 0x00, 0x00, 0x04, // Size
				0x3C, // Checksum
				0xF7, // EOX
			},
		},
		{
			name:     "Multi-byte Model ID (3-byte extension)",
			deviceID: 0x00,
			modelID:  []byte{0x00, 0x00, 0x64}, // e.g., modern Roland synth family
			addr:     [4]byte{0x01, 0x00, 0x00, 0x00},
			size:     [4]byte{0x00, 0x00, 0x02, 0x00},
			// Payload sum: 0x01 + 0x02 = 0x03 (3)
			// Checksum: 128 - 3 = 125 (0x7D)
			expected: []byte{
				0xF0, 0x41, 0x00,
				0x00, 0x00, 0x64, // 3-byte Model ID
				0x11,
				0x01, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x02, 0x00,
				0x7D,
				0xF7,
			},
		},
		{
			name:     "Payload Sum Modulo 128 Zero Edge Case",
			deviceID: 0x10,
			modelID:  []byte{0x42},
			addr:     [4]byte{0x40, 0x00, 0x00, 0x00},
			size:     [4]byte{0x40, 0x00, 0x00, 0x00},
			// Payload sum: 0x40 + 0x40 = 0x80 (128)
			// Checksum: 128 - (128 % 128) = 128 -> 128 & 0x7F = 0x00
			expected: []byte{
				0xF0, 0x41, 0x10,
				0x42,
				0x11,
				0x40, 0x00, 0x00, 0x00,
				0x40, 0x00, 0x00, 0x00,
				0x00, // Checksum wraps to 0x00
				0xF7,
			},
		},
		{
			name:     "Empty Model ID",
			deviceID: 0x7F, // Broadcast ID
			modelID:  []byte{},
			addr:     [4]byte{0x00, 0x00, 0x00, 0x00},
			size:     [4]byte{0x00, 0x00, 0x00, 0x00},
			// Payload sum: 0
			// Checksum: 128 - 0 = 128 -> 0x00
			expected: []byte{
				0xF0, 0x41, 0x7F,
				0x11,
				0x00, 0x00, 0x00, 0x00,
				0x00, 0x00, 0x00, 0x00,
				0x00,
				0xF7,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeRQ1(tt.deviceID, tt.modelID, tt.addr, tt.size)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("encodeRQ1() failed\nGot:      %X\nExpected: %X", got, tt.expected)
			}
		})
	}
}

func TestCleanASCII(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected string
	}{
		{
			name:     "Standard printable ASCII string",
			input:    []byte("Roland D-50"),
			expected: "Roland D-50",
		},
		{
			name:     "Strip control characters (NULL, CR, LF, TAB)",
			input:    []byte("Synth\x00\r\n\tSound"),
			expected: "SynthSound",
		},
		{
			name:     "Trim leading and trailing whitespace",
			input:    []byte("   JV-1080   "),
			expected: "JV-1080",
		},
		{
			name:     "Strip non-ASCII / high-bit bytes (> 126)",
			input:    []byte("Caf\xc3\xa9 \x80\xffPreset"), // UTF-8 'é' bytes and binary flags
			expected: "Caf Preset",
		},
		{
			name:     "Mix of non-printable bytes and external padding",
			input:    []byte("\x00\x05  Patch Name 01  \x1F\x7F"), // 0x7F is DEL
			expected: "Patch Name 01",
		},
		{
			name:     "Empty slice input",
			input:    []byte{},
			expected: "",
		},
		{
			name:     "Slice containing only invalid non-printable characters",
			input:    []byte{0x00, 0x01, 0x1F, 0x7F, 0x80, 0xFF},
			expected: "",
		},
		{
			name:     "Slice containing only spaces",
			input:    []byte("     "),
			expected: "",
		},
		{
			name:     "Printable ASCII boundary values (32 = Space, 126 = Tilde ~)",
			input:    []byte(" ~ "),
			expected: "~",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cleanASCII(tt.input)
			if got != tt.expected {
				t.Errorf("cleanASCII(%q) = %q; want %q", tt.input, got, tt.expected)
			}
		})
	}
}
