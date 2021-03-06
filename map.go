package msgpack

import (
	"errors"
	"fmt"
	"reflect"

	"gopkg.in/vmihailenco/msgpack.v2/codes"
)

var ErrArrMap = errors.New("array map")

func (e *Encoder) encodeMapLen(l int) error {
	switch {
	case l < 16:
		if err := e.w.WriteByte(codes.FixedMapLow | byte(l)); err != nil {
			return err
		}
	case l < 65536:
		if err := e.write2(codes.Map16, uint64(l)); err != nil {
			return err
		}
	default:
		if err := e.write4(codes.Map32, uint64(l)); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeMapStringString(m map[string]string) error {
	if err := e.encodeMapLen(len(m)); err != nil {
		return err
	}
	for mk, mv := range m {
		if err := e.EncodeString(mk); err != nil {
			return err
		}
		if err := e.EncodeString(mv); err != nil {
			return err
		}
	}
	return nil
}

func (e *Encoder) encodeMap(value reflect.Value) error {
	if err := e.encodeMapLen(value.Len()); err != nil {
		return err
	}
	for _, key := range value.MapKeys() {
		if err := e.EncodeValue(key); err != nil {
			return err
		}
		if err := e.EncodeValue(value.MapIndex(key)); err != nil {
			return err
		}
	}
	return nil
}

func decodeMap(d *Decoder) (interface{}, error) {
	n, err := d.DecodeMapLen()
	if err != nil {
		return nil, err
	}

	m := make(map[interface{}]interface{}, n)
	for i := 0; i < n; i++ {
		mk, err := d.DecodeInterface()
		if err != nil {
			return nil, err
		}
		mv, err := d.DecodeInterface()
		if err != nil {
			return nil, err
		}
		m[mk] = mv
	}
	return m, nil
}

func (d *Decoder) DecodeMapLen() (int, error) {
	c, err := d.r.ReadByte()
	if err != nil {
		return 0, err
	}
	if c == codes.Nil {
		return -1, nil
	}

	/*friendwu add*/
	if c == codes.FixedArrayLow {
		return 0, nil
	}

	if c > codes.FixedArrayLow && c <= codes.FixedArrayHigh {
		return int(c & codes.FixedArrayMask), ErrArrMap
	}
	switch c {
	case codes.Array16:
		n, _ := d.uint16()
		return int(n), ErrArrMap
	case codes.Array32:
		n, _ := d.uint32()
		return int(n), ErrArrMap
	}
	/*friendwu add end*/

	if c >= codes.FixedMapLow && c <= codes.FixedMapHigh {
		return int(c & codes.FixedMapMask), nil
	}
	switch c {
	case codes.Map16:
		n, err := d.uint16()
		return int(n), err
	case codes.Map32:
		n, err := d.uint32()
		return int(n), err
	}
	return 0, fmt.Errorf("msgpack: invalid code %x decoding map length", c)
}

func (d *Decoder) decodeIntoMapStringString(mp *map[string]string) error {
	n, err := d.DecodeMapLen()
	if err != nil {
		return err
	}
	if n == -1 {
		return nil
	}

	// TODO(vmihailenco): simpler way?
	m := *mp
	if m == nil {
		*mp = make(map[string]string, n)
		m = *mp
	}

	for i := 0; i < n; i++ {
		mk, err := d.DecodeString()
		if err != nil {
			return err
		}
		mv, err := d.DecodeString()
		if err != nil {
			return err
		}
		m[mk] = mv
	}

	return nil
}

func (d *Decoder) DecodeMap() (interface{}, error) {
	return d.DecodeMapFunc(d)
}

func (d *Decoder) mapValue(v reflect.Value) error {

	n, err := d.DecodeMapLen()

	if err != nil && err != ErrArrMap {
		return err
	}
	if n == -1 {
		return nil
	}

	typ := v.Type()
	if v.IsNil() {
		v.Set(reflect.MakeMap(typ))
	}
	keyType := typ.Key()
	valueType := typ.Elem()

	/*
		friendwu add.
		hack for lua-cmsgpack
		if map binary is an array
		we can convert it into an map[int]xx
	*/
	if err == ErrArrMap {
		if keyType.Kind() != reflect.Int {
			return errors.New("array map's must be map[int]xx" + keyType.Kind().String())
		}

		for i := 0; i < n; i++ {
			//lua array index starts from 1.
			mk := reflect.ValueOf(i + 1)
			mv := reflect.New(valueType).Elem()

			if err := d.DecodeValue(mv); err != nil {
				return err
			}

			v.SetMapIndex(mk, mv)
		}

		return nil
	}
	/*friendwu add end*/

	for i := 0; i < n; i++ {
		mk := reflect.New(keyType).Elem()
		if err := d.DecodeValue(mk); err != nil {
			return err
		}

		mv := reflect.New(valueType).Elem()
		if err := d.DecodeValue(mv); err != nil {
			return err
		}

		v.SetMapIndex(mk, mv)
	}

	return nil
}

func (e *Encoder) encodeStruct(strct reflect.Value) error {
	fields := structs.Fields(strct.Type())

	var length int
	for _, f := range fields {
		if !f.Omit(strct) {
			length++
		}
	}

	if err := e.encodeMapLen(length); err != nil {
		return err
	}

	for name, f := range fields {
		if f.Omit(strct) {
			continue
		}
		if err := e.EncodeString(name); err != nil {
			return err
		}
		if err := f.EncodeValue(e, strct); err != nil {
			return err
		}
	}

	return nil
}

func (d *Decoder) structValue(strct reflect.Value) error {
	n, err := d.DecodeMapLen()
	if err != nil {
		return err
	}

	fields := structs.Fields(strct.Type())

	for i := 0; i < n; i++ {
		name, err := d.DecodeString()
		if err != nil {
			return err
		}

		if f := fields[name]; f != nil {
			if err := f.DecodeValue(d, strct); err != nil {
				return err
			}
		} else {
			_, err := d.DecodeInterface()
			if err != nil {
				return err
			}
		}
	}

	return nil
}
