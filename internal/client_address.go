package internal

const SectionKitCommon uint32 = 0x00000000

const OffsetKitCommonVolume uint16 = 0x50
const OffsetKitCommonTempo uint16 = 0x56

func kitBaseAddress(kitIdx int) []byte {
	rawByte2 := kitIdx * 2
	b1 := byte(0x04 + (rawByte2 / 128))
	b2 := byte(rawByte2 % 128)

	return []byte{b1, b2, 0x00, 0x00}
}

func kitParamAddress(kitIdx int, subsection uint32, offset uint16) []byte {
	// Start with base address [b1, b2, 0, 0]
	base := kitBaseAddress(kitIdx)

	totalOffset := subsection + uint32(offset)

	// Convert offset into base-128 (7-bit) components
	off4 := totalOffset % 128
	off3 := (totalOffset / 128) % 128
	off2 := (totalOffset / 16384) % 128
	off1 := totalOffset / 2097152

	// Add base bytes and handle carries across 7-bit boundaries
	b4 := off4
	b3 := off3 + (b4 / 128)
	b2 := uint32(base[1]) + off2 + (b3 / 128)
	b1 := uint32(base[0]) + off1 + (b2 / 128)

	return []byte{
		byte(b1 & 0x7F),
		byte(b2 & 0x7F),
		byte(b3 & 0x7F),
		byte(b4 & 0x7F),
	}
}
