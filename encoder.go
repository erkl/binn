package binn

import (
	"math"
	"reflect"
	"sync"
)

type encoder func(b []byte, v reflect.Value) []byte

type Encoder struct {
	mu sync.RWMutex

	// Quick lookup of compiled encoder functions.
	cache map[reflect.Type]encoder

	// Compilation tasks that have been deferred to deal with recursive
	// type definitions are stored in this map.
	deferred map[reflect.Type][]*encoder
}

func NewEncoder() *Encoder {
	return &Encoder{
		cache: make(map[reflect.Type]encoder),
	}
}

func (e *Encoder) Marshal(prefix []byte, v interface{}) (out []byte, err error) {
	if v == nil {
		return append(prefix, 0x10), nil
	}

	// Catch our own "exceptions".
	defer func() {
		if r := recover(); r != nil {
			if t, ok := r.(thrown); ok {
				err = t.err
			} else {
				panic(r)
			}
		}
	}()

	// Build an appropriate encoder for this type, and use it.
	rv := reflect.ValueOf(v)
	fn := e.compile(rv.Type())

	return fn(prefix, rv), nil
}

func (e *Encoder) compile(t reflect.Type) encoder {
	var f encoder

	// Before doing any work, check the cache.
	e.mu.RLock()
	f = e.cache[t]
	e.mu.RUnlock()

	if f != nil {
		return f
	}

	// Allowing only one compilation at a time makes our code simpler,
	// and should only have a small cost in real-world programs.
	e.mu.Lock()
	defer e.mu.Unlock()

	// Make sure this type wasn't compiled between us releasing the read
	// lock and acquiring the write lock.
	if f = e.cache[t]; f != nil {
		return f
	}

	e.deferred = make(map[reflect.Type][]*encoder)
	e._compile(&f, t)
	e.deferred = nil

	return f
}

func (e *Encoder) _compile(f *encoder, t reflect.Type) {
	if *f = e.cache[t]; *f != nil {
		return
	}

	switch k := t.Kind(); k {
	case reflect.Bool:
		*f = encodeBool
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		*f = encodeInt
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		*f = encodeUint
	case reflect.Float32:
		*f = encodeFloat32
	case reflect.Float64:
		*f = encodeFloat64
	case reflect.String:
		*f = encodeString
	case reflect.Ptr:
		e._compilePointer(f, t)
	case reflect.Interface:
		e._compileInterface(f, t)

	default:
		// If we've already seen this type before, but haven't yet finished
		// compiling an encoder for it, we're dealing with a recursive type.
		if def := e.deferred[t]; def != nil {
			e.deferred[t] = append(def, f)
			return
		}

		// Mark this type as seen.
		e.deferred[t] = []*encoder{}

		switch k {
		case reflect.Slice:
			e._compileSlice(f, t)
		case reflect.Array:
			e._compileArray(f, t)
		case reflect.Map:
			e._compileMap(f, t)
		case reflect.Struct:
			e._compileStruct(f, t)

		default:
			throwf("binn: unsupported type: %s", t)
		}

		// Now that the encoder has been built, fill in any blanks.
		for _, fp := range e.deferred[t] {
			*fp = *f
		}
	}

	e.cache[t] = *f
}

func (e *Encoder) _compilePointer(f *encoder, t reflect.Type) {
	pe := new(pointerEncoder)
	e._compile(&pe.encodeElem, t.Elem())
	*f = pe.encode
}

func (e *Encoder) _compileInterface(f *encoder, t reflect.Type) {
	ie := new(interfaceEncoder)
	ie.encoder = e
	*f = ie.encode
}

func (e *Encoder) _compileSlice(f *encoder, t reflect.Type) {
	if t.Elem().Kind() == reflect.Uint8 {
		*f = encodeByteSlice
	} else {
		le := new(listEncoder)
		e._compile(&le.encodeElem, t.Elem())
		*f = le.encode
	}
}

func (e *Encoder) _compileArray(f *encoder, t reflect.Type) {
	if t.Elem().Kind() == reflect.Uint8 {
		*f = encodeByteArray
	} else {
		le := new(listEncoder)
		e._compile(&le.encodeElem, t.Elem())
		*f = le.encode
	}
}

func (e *Encoder) _compileMap(f *encoder, t reflect.Type) {
	me := new(mapEncoder)
	e._compile(&me.encodeKey, t.Key())
	e._compile(&me.encodeElem, t.Elem())
	*f = me.encode
}

func (e *Encoder) _compileStruct(f *encoder, t reflect.Type) {
	fields := enumStructFields(t)
	se := &structEncoder{
		fields: make([]structEncoderField, len(fields)),
	}

	for i, f := range fields {
		se.fields[i] = structEncoderField{
			name:  encodeString(nil, reflect.ValueOf(f.Name)),
			index: f.Index,
		}
		e._compile(&se.fields[i].encode, t.FieldByIndex(f.Index).Type)
	}

	*f = se.encode
}

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

type listEncoder struct {
	encodeElem encoder
}

func (le *listEncoder) encode(b []byte, v reflect.Value) []byte {
	n := v.Len()

	if n < (0x6c - 0x50) {
		b = append(b, byte(0x50+n))
	} else {
		b = encodeK4(b, 0x6c, uint64(n))
	}

	for i := 0; i < n; i++ {
		b = le.encodeElem(b, v.Index(i))
	}

	return b
}

type mapEncoder struct {
	encodeKey  encoder
	encodeElem encoder
}

func (me *mapEncoder) encode(b []byte, v reflect.Value) []byte {
	n := v.Len()

	if n < (0x4c - 0x30) {
		b = append(b, byte(0x30+n))
	} else {
		b = encodeK4(b, 0x4c, uint64(n))
	}

	for _, k := range v.MapKeys() {
		b = me.encodeKey(b, k)
		b = me.encodeElem(b, v.MapIndex(k))
	}

	return b
}

type structEncoder struct {
	fields []structEncoderField
}

type structEncoderField struct {
	name   []byte
	index  []int
	encode encoder
}

func (se *structEncoder) encode(b []byte, v reflect.Value) []byte {
	n := len(se.fields)

	if n < (0x4c - 0x30) {
		b = append(b, byte(0x30+n))
	} else {
		b = encodeK4(b, 0x4c, uint64(n))
	}

	for _, f := range se.fields {
		b = append(b, f.name...)
		b = f.encode(b, v.FieldByIndex(f.index))
	}

	return b
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
	return encodeK8(b, 0x14, rev32(bits))
}

func encodeFloat64(b []byte, v reflect.Value) []byte {
	bits := uint64(math.Float64bits(float64(v.Float())))
	return encodeK8(b, 0x18, rev64(bits))
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
