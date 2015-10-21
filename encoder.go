package binn

import (
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

	case reflect.Int:
		*f = encodeInt
	case reflect.Int8:
		*f = encodeInt
	case reflect.Int16:
		*f = encodeInt
	case reflect.Int32:
		*f = encodeInt
	case reflect.Int64:
		*f = encodeInt

	case reflect.Uint:
		*f = encodeUint
	case reflect.Uint8:
		*f = encodeUint
	case reflect.Uint16:
		*f = encodeUint
	case reflect.Uint32:
		*f = encodeUint
	case reflect.Uint64:
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
			throwf("binn: cannot encode type: %s", t)
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
