package binn

// skip seeks past the next value in b.
func skip(b []byte) []byte {
	n, m := scan(b)
	b = b[:n]

	for i := 0; i < m; i++ {
		b = skip(b)
	}

	return b
}

// scan returns the size of the next encoded value in bytes. For lists
// and maps, the number of child elements is returned as well.
func scan(b []byte) (int, int) {
	switch k := b[0]; {
	// Single-byte values.
	case k <= 0x12:
		return 1, 0

	// Times.
	case k == 0x13:
		panic("todo: scan timestamp/duration")

	// Numbers.
	case k <= 0x17:
		return 1 + (1 + int(k)&3), 0
	case k <= 0x2f:
		return 1 + (1 + int(k)&7), 0

	// Maps.
	case k <= 0x4b:
		return 1, int(k - 0x30)
	case k <= 0x4f:
		if n, r := decodeK4(b, 0x4c); n > maxInt/2 {
			throwf("binn: map value size overflow (%d elements)", n)
		} else {
			return len(b) - len(r), int(2 * n)
		}

	// Lists.
	case k <= 0x6b:
		return 1, int(k - 0x50)
	case k <= 0x6f:
		if n, r := decodeK4(b, 0x6c); n > maxInt {
			throwf("binn: list value size overflow (%d elements)", n)
		} else {
			return len(b) - len(r), int(n)
		}

	// Strings.
	case k <= 0xdb:
		return 1 + int(k-0x70), 0
	case k <= 0xdf:
		n, r := decodeK4(b, 0xdc)
		m := uint64(len(b) - len(r))

		if n > maxInt-m {
			throwf("binn: string value size overflow (%d bytes)", n)
		} else {
			return int(m + n), 0
		}

	// Binary.
	case k <= 0xeb:
		return 1 + int(k-0xe0), 0
	case k <= 0xef:
		n, r := decodeK4(b, 0xec)
		m := uint64(len(b) - len(r))

		if n > maxInt-m {
			throwf("binn: binary value size overflow (%d bytes)", n)
		} else {
			return int(m + n), 0
		}

	// Extension.
	case k <= 0xfb:
		return 1 + int(k-0xf0), 0
	default:
		n, r := decodeK4(b, 0xfc)
		m := uint64(len(b) - len(r))

		if n > maxInt-m {
			throwf("binn: extension value size overflow (%d bytes)", n)
		} else {
			return int(m + n), 0
		}
	}

	panic("unreachable")
}
