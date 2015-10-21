package binn

import (
	"math"
	"reflect"
)

type pointerEncoder struct {
	encodeElem encoder
}

func (pe *pointerEncoder) encode(b []byte, v reflect.Value) []byte {
	if v.IsNil() {
		return append(b, 0x10)
	} else {
		return pe.encodeElem(b, v.Elem())
	}
}

type interfaceEncoder struct {
	encoder *Encoder
}

func (ie *interfaceEncoder) encode(b []byte, v reflect.Value) []byte {
	if v.IsNil() {
		return append(b, 0x10)
	} else {
		v = v.Elem()
		return ie.encoder.compile(v.Type())(b, v)
	}
}

func encodeBool(b []byte, v reflect.Value) []byte {
	if v.Bool() {
		return append(b, 0x12)
	} else {
		return append(b, 0x11)
	}
}

func encodeFloat32(b []byte, v reflect.Value) []byte {
	bits := uint64(math.Float32bits(float32(v.Float())))

	rev := (bits & 0x000000ff) << 24
	rev |= (bits & 0x0000ff00) << 8
	rev |= (bits & 0x00ff0000) >> 8
	rev |= (bits & 0xff000000) >> 24

	return encodeK8(b, 0x14, rev)
}

func encodeFloat64(b []byte, v reflect.Value) []byte {
	bits := uint64(math.Float64bits(float64(v.Float())))

	rev := (bits & 0x00000000000000ff) << 56
	rev |= (bits & 0x000000000000ff00) << 40
	rev |= (bits & 0x0000000000ff0000) << 24
	rev |= (bits & 0x00000000ff000000) << 8
	rev |= (bits & 0x000000ff00000000) >> 8
	rev |= (bits & 0x0000ff0000000000) >> 24
	rev |= (bits & 0x00ff000000000000) >> 40
	rev |= (bits & 0xff00000000000000) >> 56

	return encodeK8(b, 0x18, rev)
}

func encodeInt(b []byte, v reflect.Value) []byte {
	if x := v.Int(); x < 0 {
		if x >= -8 {
			return append(b, byte(7-x))
		} else {
			return encodeK8(b, 0x20, uint64(-(x+1))+1)
		}
	} else {
		if x <= 7 {
			return append(b, byte(x))
		} else {
			return encodeK8(b, 0x28, uint64(x))
		}
	}
}

func encodeUint(b []byte, v reflect.Value) []byte {
	if x := v.Uint(); x <= 7 {
		return append(b, byte(x))
	} else {
		return encodeK8(b, 0x28, x)
	}
}

func encodeString(b []byte, v reflect.Value) []byte {
	x := v.String()

	if n := len(x); n < (0xdc - 0x70) {
		b = append(b, byte(0x70+n))
	} else {
		b = encodeK4(b, 0xdc, uint64(n))
	}

	return append(b, x...)
}

func encodeByteSlice(b []byte, v reflect.Value) []byte {
	x := string(v.Bytes())

	if n := len(x); n < (0xec - 0xe0) {
		b = append(b, byte(0xe0+n))
	} else {
		b = encodeK4(b, 0xec, uint64(n))
	}

	return append(b, x...)
}

func encodeByteArray(b []byte, v reflect.Value) []byte {
	n := v.Len()

	if n < (0xec - 0xe0) {
		b = append(b, byte(0xe0+n))
	} else {
		b = encodeK4(b, 0xec, uint64(n))
	}

	// Fast path for when the array is addressable (which it almost
	// always will be).
	if v.CanAddr() {
		return append(b, v.Slice(0, n).Bytes()...)
	}

	i := len(b)
	j := i + n

	if j > cap(b) {
		t := make([]byte, i, j)
		copy(t, b)
		b = t
	}

	reflect.Copy(reflect.ValueOf(b[i:j]), v)
	return b[:j]
}

func encodeK4(b []byte, k uint8, x uint64) []byte {
	switch {
	case x <= 0xff:
		return append(b, k+0, byte(x))
	case x <= 0xffff:
		return append(b, k+1, byte(x), byte(x>>8))
	case x <= 0xffffffff:
		return append(b, k+2, byte(x), byte(x>>8), byte(x>>16), byte(x>>24))
	default:
		return append(b, k+3, byte(x), byte(x>>8), byte(x>>16), byte(x>>24), byte(x>>32), byte(x>>40), byte(x>>48), byte(x>>56))
	}
}

func encodeK8(b []byte, k uint8, x uint64) []byte {
	switch {
	case x <= 0xff:
		return append(b, k+0, byte(x))
	case x <= 0xffff:
		return append(b, k+1, byte(x), byte(x>>8))
	case x <= 0xffffff:
		return append(b, k+2, byte(x), byte(x>>8), byte(x>>16))
	case x <= 0xffffffff:
		return append(b, k+3, byte(x), byte(x>>8), byte(x>>16), byte(x>>24))
	case x <= 0xffffffffff:
		return append(b, k+4, byte(x), byte(x>>8), byte(x>>16), byte(x>>24), byte(x>>32))
	case x <= 0xffffffffffff:
		return append(b, k+5, byte(x), byte(x>>8), byte(x>>16), byte(x>>24), byte(x>>32), byte(x>>40))
	case x <= 0xffffffffffffff:
		return append(b, k+6, byte(x), byte(x>>8), byte(x>>16), byte(x>>24), byte(x>>32), byte(x>>40), byte(x>>48))
	default:
		return append(b, k+7, byte(x), byte(x>>8), byte(x>>16), byte(x>>24), byte(x>>32), byte(x>>40), byte(x>>48), byte(x>>56))
	}
}
