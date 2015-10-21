package binn

import (
	"reflect"
	"sync"
)

type decoder func(b []byte, v reflect.Value) []byte

type Decoder struct {
	mu sync.RWMutex

	// Quick lookup of compiled decoder functions.
	cache map[reflect.Type]decoder

	// Compilation tasks that have been deferred to deal with recursive
	// type definitions are stored in this map.
	deferred map[reflect.Type][]*decoder
}

func NewDecoder() *Decoder {
	return &Decoder{
		cache: make(map[reflect.Type]decoder),
	}
}

func (d *Decoder) Unmarshal(buf []byte, v interface{}) (err error) {
	// Catch our own "exceptions".
	defer func() {
		if r := recover(); r != nil {
			if t, ok := r.(thrown); ok {
				err = t.err
			} else if isRangePanic(r) {
				err = errorf("binn: input too short")
			} else {
				panic(r)
			}
		}
	}()

	// Make sure the destination value is addressable.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Ptr || rv.IsNil() {
		return errorf("binn: can only unmarshal into non-nil pointer")
	}

	rv = rv.Elem()

	// Build an appropriate decoder for this type, and use it.
	fn := d.compile(rv.Type())
	rem := fn(buf, rv)

	if len(rem) > 0 {
		return errorf("binn: input contained redundant data")
	}

	return nil
}

func (d *Decoder) compile(t reflect.Type) decoder {
	var f decoder

	// Before doing any work, check the cache.
	d.mu.RLock()
	f = d.cache[t]
	d.mu.RUnlock()

	if f != nil {
		return f
	}

	// Allowing only one compilation at a time makes our code simpler,
	// and should only have a small cost in real-world programs.
	d.mu.Lock()
	defer d.mu.Unlock()

	// Make sure this type wasn't processed between us releasing the read
	// lock and acquiring the write lock.
	if f = d.cache[t]; f != nil {
		return f
	}

	d.deferred = make(map[reflect.Type][]*decoder)
	d._compile(&f, t)
	d.deferred = nil

	return f
}

func (d *Decoder) _compile(f *decoder, t reflect.Type) {
	if *f = d.cache[t]; *f != nil {
		return
	}

	switch k := t.Kind(); k {
	case reflect.Bool:
		*f = decodeBool

	case reflect.Int:
		*f = decodeInt
	case reflect.Int8:
		*f = decodeInt
	case reflect.Int16:
		*f = decodeInt
	case reflect.Int32:
		*f = decodeInt
	case reflect.Int64:
		*f = decodeInt

	case reflect.Uint:
		*f = decodeUint
	case reflect.Uint8:
		*f = decodeUint
	case reflect.Uint16:
		*f = decodeUint
	case reflect.Uint32:
		*f = decodeUint
	case reflect.Uint64:
		*f = decodeUint

	case reflect.Float32:
		*f = decodeFloat
	case reflect.Float64:
		*f = decodeFloat

	case reflect.String:
		*f = decodeString

	case reflect.Ptr:
		d._compilePointer(f, t)
	case reflect.Interface:
		d._compileInterface(f, t)

	default:
		// If we've already seen this type before, but haven't yet finished
		// compiling an decoder for it, it's a recursive type.
		if def := d.deferred[t]; def != nil {
			d.deferred[t] = append(def, f)
			return
		}

		// Mark this type as seen.
		d.deferred[t] = []*decoder{}

		switch k {
		case reflect.Slice:
			d._compileSlice(f, t)
		case reflect.Array:
			d._compileArray(f, t)

		case reflect.Map:
			d._compileMap(f, t)
		case reflect.Struct:
			d._compileStruct(f, t)

		default:
			throwf("binn: unsupported type: %s", t)
		}

		// Now that the decoder has been built, fill in any blanks.
		for _, fp := range d.deferred[t] {
			*fp = *f
		}
	}

	d.cache[t] = *f
}

func (d *Decoder) _compilePointer(f *decoder, t reflect.Type) {
	throwf("todo: decode pointer")
}

func (d *Decoder) _compileInterface(f *decoder, t reflect.Type) {
	throwf("todo: decode interface")
}

func (d *Decoder) _compileSlice(f *decoder, t reflect.Type) {
	if t.Elem().Kind() == reflect.Uint8 {
		*f = decodeByteSlice
	} else {
		throwf("todo: decode slice")
	}
}

func (d *Decoder) _compileArray(f *decoder, t reflect.Type) {
	if t.Elem().Kind() == reflect.Uint8 {
		*f = decodeByteArray
	} else {
		throwf("todo: decode slice")
	}
}

func (d *Decoder) _compileMap(f *decoder, t reflect.Type) {
	throwf("todo: decode map")
}

func (d *Decoder) _compileStruct(f *decoder, t reflect.Type) {
	throwf("todo: decode struct")
}

func decodeBool(b []byte, v reflect.Value) []byte {
	throwf("todo: decode bool")
}

func decodeFloat(b []byte, v reflect.Value) []byte {
	throwf("todo: decode float")
}

func decodeInt(b []byte, v reflect.Value) []byte {
	throwf("todo: decode int")
}

func decodeUint(b []byte, v reflect.Value) []byte {
	throwf("todo: decode uint")
}

func decodeString(b []byte, v reflect.Value) []byte {
	throwf("todo: decode string")
}

func decodeByteSlice(b []byte, v reflect.Value) []byte {
	throwf("todo: decode byte slice")
}

func decodeByteArray(b []byte, v reflect.Value) []byte {
	throwf("todo: decode byte array")
}

func decodeK4(b []byte, k uint8) (uint64, []byte) {
	switch b[0] - k {
	case 0:
		return uint64(b[1]), b[2:]
	case 1:
		return uint64(b[1]) | uint64(b[2])<<8, b[3:]
	case 2:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24, b[5:]
	default:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24 | uint64(b[5])<<32 | uint64(b[6])<<40 | uint64(b[7])<<48 | uint64(b[8])<<56, b[9:]
	}
}

func decodeK8(b []byte, k uint8) (uint64, []byte) {
	switch b[0] - k {
	case 0:
		return uint64(b[1]), b[2:]
	case 1:
		return uint64(b[1]) | uint64(b[2])<<8, b[3:]
	case 2:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16, b[4:]
	case 3:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24, b[5:]
	case 4:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24 | uint64(b[5])<<32, b[6:]
	case 5:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24 | uint64(b[5])<<32 | uint64(b[6])<<40, b[7:]
	case 6:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24 | uint64(b[5])<<32 | uint64(b[6])<<40 | uint64(b[7])<<48, b[8:]
	default:
		return uint64(b[1]) | uint64(b[2])<<8 | uint64(b[3])<<16 | uint64(b[4])<<24 | uint64(b[5])<<32 | uint64(b[6])<<40 | uint64(b[7])<<48 | uint64(b[8])<<56, b[9:]
	}
}
