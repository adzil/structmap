/*
Copyright 2023 Fadhli Dzil Ikram.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package structmap

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	_ unmarshaler = (*pointerUnmarshaler)(nil)
	_ unmarshaler = (*structUnmarshaler)(nil)
	_ unmarshaler = (*stringUnmarshaler)(nil)
	_ unmarshaler = (*intUnmarshaler)(nil)
	_ unmarshaler = (*methodUnmarshaler)(nil)
	_ unmarshaler = (*sliceUnmarshaler)(nil)
)

var (
	errSkipField = errors.New("skip field")
)

var (
	valueUnmarshalerReflectType = reflect.TypeOf((*ValueUnmarshaler)(nil)).Elem()
)

type ValueUnmarshaler interface {
	UnmarshalValue(v []string) error
}

type unmarshalContext struct {
	value []string
}

type unmarshaler interface {
	unmarshal(ctx unmarshalContext, v map[string][]string, dst reflect.Value) error
}

type pointerUnmarshaler struct {
	elemTyp reflect.Type
	elem    unmarshaler
}

func (u *pointerUnmarshaler) unmarshal(ctx unmarshalContext, v map[string][]string, dst reflect.Value) error {
	if dst.IsNil() {
		dst.Set(reflect.New(u.elemTyp))
	}

	return u.elem.unmarshal(ctx, v, dst.Elem())
}

type fieldUnmarshaler struct {
	name        string
	required    bool
	nested      bool
	index       int
	unmarshaler unmarshaler
}

func (c *fieldUnmarshaler) applyOption(opt string) error {
	switch opt {
	case "required":
		c.required = true
	case "omitempty":
		// This option is only valid for marhsaler so it will be ignored.
	case "":
		// Allow empty option.
	default:
		return fmt.Errorf("unknown option %s", opt)
	}

	return nil
}

type structUnmarshaler struct {
	fields []fieldUnmarshaler
}

func getValue(v map[string][]string, key string) ([]string, bool) {
	val, ok := v[key]
	if !ok {
		return nil, false
	}

	if len(val) == 0 {
		return nil, false
	}

	return val, true
}

func (u *structUnmarshaler) unmarshalField(
	ctx unmarshalContext,
	field fieldUnmarshaler,
	v map[string][]string,
	dst reflect.Value,
) error {
	if !field.nested {
		var ok bool
		ctx.value, ok = getValue(v, field.name)

		if !ok {
			if field.required {
				return fmt.Errorf(`value not found for required key "%s"`, field.name)
			}

			dst.Field(field.index).SetZero()

			return nil
		}
	}

	return field.unmarshaler.unmarshal(ctx, v, dst.Field(field.index))
}

func (u *structUnmarshaler) unmarshal(ctx unmarshalContext, v map[string][]string, dst reflect.Value) error {
	for _, field := range u.fields {
		if err := u.unmarshalField(ctx, field, v, dst); err != nil {
			return err
		}
	}

	return nil
}

type stringUnmarshaler struct{}

func (u *stringUnmarshaler) unmarshal(ctx unmarshalContext, _ map[string][]string, dst reflect.Value) error {
	dst.SetString(ctx.value[0])

	return nil
}

type intUnmarshaler struct {
	bitSize int
}

func (u *intUnmarshaler) unmarshal(ctx unmarshalContext, _ map[string][]string, dst reflect.Value) error {
	val, err := strconv.ParseInt(ctx.value[0], 10, u.bitSize)
	if err != nil {
		return err
	}

	dst.SetInt(val)

	return nil
}

type methodUnmarshaler struct {
	newFn       func(dst reflect.Value)
	ptrReceiver bool
}

func (u *methodUnmarshaler) unmarshal(ctx unmarshalContext, _ map[string][]string, dst reflect.Value) error {
	if u.newFn != nil {
		u.newFn(dst)
	}

	if u.ptrReceiver {
		// TODO: Make sure that this is guaranteed to be addressable or not.
		// The non-nil pointer check already in place at the top-level function
		// so it must always be addressable.
		dst = dst.Addr()
	}

	return dst.Interface().(ValueUnmarshaler).UnmarshalValue(ctx.value)
}

type sliceUnmarshaler struct {
	typ     reflect.Type
	bitSize int
}

func (u *sliceUnmarshaler) unmarshal(ctx unmarshalContext, _ map[string][]string, dst reflect.Value) error {
	if dst.Cap() < len(ctx.value) {
		dst.Set(reflect.MakeSlice(u.typ, len(ctx.value), len(ctx.value)))
	} else if dst.Len() != len(ctx.value) {
		dst.SetLen(len(ctx.value))
	}

	for i := 0; i < len(ctx.value); i++ {
		if u.bitSize > 0 {
			val, err := strconv.ParseInt(ctx.value[i], 10, u.bitSize)
			if err != nil {
				return fmt.Errorf("int slice index #%d: %w", i, err)
			}

			dst.Index(i).SetInt(val)
		} else {
			dst.Index(i).SetString(ctx.value[i])
		}
	}

	return nil
}

func buildNewFunc(typ reflect.Type) func(dst reflect.Value) {
	switch typ.Kind() {
	case reflect.Pointer:
		return func(dst reflect.Value) {
			if dst.IsNil() {
				dst.Set(reflect.New(typ.Elem()))
			}
		}

	case reflect.Map:
		return func(dst reflect.Value) {
			if dst.IsNil() {
				dst.Set(reflect.MakeMap(typ))
			}
		}
	}

	return nil
}

func getIntSize(kind reflect.Kind) int {
	switch kind {
	case reflect.Int:
		return strconv.IntSize
	case reflect.Int64:
		return 64
	case reflect.Int32:
		return 32
	case reflect.Int16:
		return 16
	case reflect.Int8:
		return 8
	}

	return -1
}

func newSliceUnmarshaler(typ reflect.Type) (unmarshaler, error) {
	elem := typ.Elem()

	if elem.Kind() == reflect.String {
		return &sliceUnmarshaler{typ: typ}, nil
	}

	if bitSize := getIntSize(elem.Kind()); bitSize > 0 {
		return &sliceUnmarshaler{
			typ:     typ,
			bitSize: bitSize,
		}, nil
	}

	return nil, fmt.Errorf("cannot unmarshal into slice of %s", elem.Kind().String())
}

func newValueUnmarshaler(cfg unmarshalConfig, typ reflect.Type) (unm unmarshaler, nested bool, err error) {
	var valReceiver bool

	switch {
	case typ.Implements(valueUnmarshalerReflectType):
		valReceiver = true

		fallthrough

	case reflect.PointerTo(typ).Implements(valueUnmarshalerReflectType):
		return &methodUnmarshaler{
			newFn:       buildNewFunc(typ),
			ptrReceiver: !valReceiver,
		}, false, nil
	}

	switch typ.Kind() {
	case reflect.Pointer:
		unm, nested, err := newValueUnmarshaler(cfg, typ.Elem())
		if err != nil {
			return nil, false, err
		}

		return &pointerUnmarshaler{
			elemTyp: typ.Elem(),
			elem:    unm,
		}, nested, nil

	case reflect.Struct:
		unm, err := newStructUnmarshaler(cfg, typ)

		return unm, true, err

	case reflect.String:
		return &stringUnmarshaler{}, false, nil

	case reflect.Slice:
		unm, err := newSliceUnmarshaler(typ)

		return unm, false, err
	}

	if intSize := getIntSize(typ.Kind()); intSize > 0 {
		return &intUnmarshaler{
			bitSize: intSize,
		}, false, nil
	}

	return nil, false, fmt.Errorf("cannot unmarshal into %s", typ.Kind().String())
}

func newFieldUnmarshaler(cfg unmarshalConfig, structFld reflect.StructField) (fieldUnmarshaler, error) {
	tag := strings.Split(structFld.Tag.Get("map"), ",")
	name := tag[0]

	// Follow the encoding/json standard where a field can still be named "-"
	// by using a comma suffix.
	if name == "-" && len(tag) == 1 {
		return fieldUnmarshaler{}, errSkipField
	}

	field := fieldUnmarshaler{
		index: structFld.Index[len(structFld.Index)-1],
	}

	for i := 1; i < len(tag); i++ {
		if err := field.applyOption(tag[i]); err != nil {
			return fieldUnmarshaler{}, err
		}
	}

	prefix := cfg.Prefix
	if name != "" {
		prefix = append(prefix, name)
	} else if !structFld.Anonymous {
		prefix = append(prefix, structFld.Name)
	}

	var err error
	if field.unmarshaler, field.nested, err = newValueUnmarshaler(unmarshalConfig{
		UnmarshalConfig: cfg.UnmarshalConfig,
		Prefix:          prefix,
	}, structFld.Type); err != nil {
		return fieldUnmarshaler{}, fmt.Errorf("struct field %s: %w", structFld.Name, err)
	}

	if field.nested {
		if field.required {
			return fieldUnmarshaler{}, errors.New("cannot set required option for struct")
		}

		return field, nil
	}

	if structFld.Anonymous && name == "" {
		prefix = append(prefix, structFld.Name)
	}

	field.name = strings.Join(prefix, cfg.delimiter())

	return field, nil
}

func newStructUnmarshaler(cfg unmarshalConfig, typ reflect.Type) (unmarshaler, error) {
	var fields []fieldUnmarshaler

	n := typ.NumField()
	for i := 0; i < n; i++ {
		field, err := newFieldUnmarshaler(cfg, typ.Field(i))

		if errors.Is(err, errSkipField) {
			continue
		}

		if err != nil {
			return nil, err
		}

		fields = append(fields, field)
	}

	return &structUnmarshaler{
		fields: fields,
	}, nil
}

type UnmarshalConfig struct {
	Delimiter string
}

func (cfg UnmarshalConfig) delimiter() string {
	if cfg.Delimiter != "" {
		return cfg.Delimiter
	}

	return "."
}

func (cfg UnmarshalConfig) Unmarshal(v map[string][]string, dst any) error {
	return defaultUnmarshaler.Unmarshal(cfg, v, dst)
}

type unmarshalConfig struct {
	UnmarshalConfig
	Prefix []string
}

func newUnmarshaler(cfg unmarshalConfig, typ reflect.Type) (unmarshaler, error) {
	switch typ.Kind() {
	case reflect.Struct:
		return newStructUnmarshaler(cfg, typ)

	case reflect.Pointer:
		elem, err := newUnmarshaler(cfg, typ.Elem())
		if err != nil {
			return nil, err
		}

		return &pointerUnmarshaler{
			elemTyp: typ.Elem(),
			elem:    elem,
		}, nil
	}

	return nil, fmt.Errorf("cannot unmarshal into %s", typ.Kind().String())
}

type unmarshalerCacheKey struct {
	typ    reflect.Type
	config UnmarshalConfig
}

type unmarshalerCache struct {
	cache cache[unmarshalerCacheKey, unmarshaler]
}

func (uc *unmarshalerCache) Unmarshal(cfg UnmarshalConfig, v map[string][]string, dst any) error {
	val := reflect.ValueOf(dst)

	if val.Kind() != reflect.Pointer || val.IsNil() {
		return errors.New("can only unmarshal into a non-nil pointer")
	}

	elem := val.Elem()
	key := unmarshalerCacheKey{
		typ:    elem.Type(),
		config: cfg,
	}

	vu, err := uc.cache.Get(key, func(key unmarshalerCacheKey) (unmarshaler, error) {
		return newUnmarshaler(unmarshalConfig{UnmarshalConfig: key.config}, key.typ)
	})
	if err != nil {
		return err
	}

	return vu.unmarshal(unmarshalContext{}, v, elem)
}

var defaultUnmarshaler unmarshalerCache

func Unmarshal(v map[string][]string, dst any) error {
	return defaultUnmarshaler.Unmarshal(UnmarshalConfig{}, v, dst)
}
