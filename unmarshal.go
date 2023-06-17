package dstruct

import (
	"errors"
	"fmt"
	"net/url"
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
	_ unmarshaler = (*stringSliceUnmarshaler)(nil)
	_ unmarshaler = (*intSliceUnmarshaler)(nil)
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
	delimiter string
	path      []string
	value     []string
}

func (uc *unmarshalContext) getDelimiter() string {
	if uc.delimiter != "" {
		return uc.delimiter
	}

	return "."
}

type unmarshaler interface {
	unmarshal(ctx unmarshalContext, v url.Values, dst reflect.Value) error
}

type pointerUnmarshaler struct {
	elemTyp reflect.Type
	elem    unmarshaler
}

func (u *pointerUnmarshaler) unmarshal(ctx unmarshalContext, v url.Values, dst reflect.Value) error {
	if dst.IsNil() {
		dst.Set(reflect.New(u.elemTyp))
	}

	return u.elem.unmarshal(ctx, v, dst.Elem())
}

type fieldUnmarshalConfig struct {
	name     string
	required bool
	nested   bool
	index    int
	field    unmarshaler
}

func (c *fieldUnmarshalConfig) applyOption(opt string) {
	switch opt {
	case "required":
		c.required = true
	}
}

type structUnmarshaler struct {
	fields []fieldUnmarshalConfig
}

func getValue(v url.Values, key string) ([]string, bool) {
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
	field fieldUnmarshalConfig,
	v url.Values,
	dst reflect.Value,
) error {
	fieldCtx := unmarshalContext{
		delimiter: ctx.delimiter,
		path:      ctx.path,
	}

	if field.name != "" {
		fieldCtx.path = append(fieldCtx.path, field.name)
	}

	if !field.nested {
		key := strings.Join(fieldCtx.path, ctx.getDelimiter())

		var ok bool
		fieldCtx.value, ok = getValue(v, key)

		if !ok {
			if field.required {
				return fmt.Errorf(`value not found for required key "%s"`, key)
			}

			dst.Field(field.index).SetZero()

			return nil
		}
	}

	return field.field.unmarshal(fieldCtx, v, dst.Field(field.index))
}

func ensureSize[T any](in []T) []T {
	if cap(in) > len(in) {
		return in
	}

	nin := make([]T, len(in), (cap(in)+1)*2)
	copy(nin, in)

	return nin
}

func (u *structUnmarshaler) unmarshal(ctx unmarshalContext, v url.Values, dst reflect.Value) error {
	// Without this, there will be an excessive amount of slice allocation when
	// len(ctx.path) == cap(ctx.path). This also ensures that the ctx.path
	// slice is going to be reused for most of the time.
	ctx.path = ensureSize(ctx.path)

	for _, field := range u.fields {
		if err := u.unmarshalField(ctx, field, v, dst); err != nil {
			return err
		}
	}

	return nil
}

type stringUnmarshaler struct{}

func (u *stringUnmarshaler) unmarshal(ctx unmarshalContext, _ url.Values, dst reflect.Value) error {
	dst.SetString(ctx.value[0])

	return nil
}

type intUnmarshaler struct {
	bitSize int
}

func (u *intUnmarshaler) unmarshal(ctx unmarshalContext, _ url.Values, dst reflect.Value) error {
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

func (u *methodUnmarshaler) unmarshal(ctx unmarshalContext, _ url.Values, dst reflect.Value) error {
	if u.newFn != nil {
		u.newFn(dst)
	}

	if u.ptrReceiver {
		dst = dst.Addr()
	}

	return dst.Interface().(ValueUnmarshaler).UnmarshalValue(ctx.value)
}

type stringSliceUnmarshaler struct{}

func (u *stringSliceUnmarshaler) unmarshal(ctx unmarshalContext, _ url.Values, dst reflect.Value) error {
	dst.Set(reflect.ValueOf(ctx.value))

	return nil
}

type intSliceUnmarshaler struct {
	typ     reflect.Type
	bitSize int
}

func (u *intSliceUnmarshaler) unmarshal(ctx unmarshalContext, _ url.Values, dst reflect.Value) error {
	if dst.Cap() < len(ctx.value) {
		dst.Set(reflect.MakeSlice(u.typ, len(ctx.value), len(ctx.value)))
	} else if dst.Len() != len(ctx.value) {
		dst.SetLen(len(ctx.value))
	}

	for i := 0; i < len(ctx.value); i++ {
		val, err := strconv.ParseInt(ctx.value[i], 10, u.bitSize)
		if err != nil {
			return fmt.Errorf("int slice index #%d: %w", i, err)
		}

		dst.Index(i).SetInt(val)
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
		return &stringSliceUnmarshaler{}, nil
	}

	if bitSize := getIntSize(elem.Kind()); bitSize > 0 {
		return &intSliceUnmarshaler{
			typ:     typ,
			bitSize: bitSize,
		}, nil
	}

	return nil, fmt.Errorf("cannot unmarshal into slice of %s", elem.Kind().String())
}

func newFieldUnmarshaler(typ reflect.Type) (unm unmarshaler, nested bool, err error) {
	var ptrReceiver bool

	switch {
	case reflect.PointerTo(typ).Implements(valueUnmarshalerReflectType):
		ptrReceiver = true

		fallthrough

	case typ.Implements(valueUnmarshalerReflectType):
		return &methodUnmarshaler{
			newFn:       buildNewFunc(typ),
			ptrReceiver: ptrReceiver,
		}, false, nil
	}

	switch typ.Kind() {
	case reflect.Pointer:
		unm, nested, err := newFieldUnmarshaler(typ.Elem())
		if err != nil {
			return nil, false, err
		}

		return &pointerUnmarshaler{
			elemTyp: typ.Elem(),
			elem:    unm,
		}, nested, nil

	case reflect.Struct:
		unm, err := newStructUnmarshaler(typ)

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

func newFieldUnmarshalConfig(field reflect.StructField) (fieldUnmarshalConfig, error) {
	tag := strings.Split(field.Tag.Get("map"), ",")
	name := tag[0]

	// Follow the encoding/json standard where a field can still be named "-"
	// by using a comma suffix.
	if name == "-" && len(tag) == 1 {
		return fieldUnmarshalConfig{}, errSkipField
	}

	conf := fieldUnmarshalConfig{
		name:  name,
		index: field.Index[len(field.Index)-1],
	}

	for i := 1; i < len(tag); i++ {
		conf.applyOption(tag[i])
	}

	var err error
	if conf.field, conf.nested, err = newFieldUnmarshaler(field.Type); err != nil {
		return fieldUnmarshalConfig{}, fmt.Errorf("struct field %s: %w", field.Name, err)
	}

	// Only anonymous struct can have their name empty. Otherwise, we need to
	// replace it with the field name.
	if conf.name == "" && (!conf.nested || !field.Anonymous) {
		conf.name = field.Name
	}

	return conf, nil
}

func newStructUnmarshaler(typ reflect.Type) (unmarshaler, error) {
	var fields []fieldUnmarshalConfig

	n := typ.NumField()
	for i := 0; i < n; i++ {
		field, err := newFieldUnmarshalConfig(typ.Field(i))

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

func newUnmarshaler(typ reflect.Type) (unmarshaler, error) {
	switch typ.Kind() {
	case reflect.Struct:
		return newStructUnmarshaler(typ)

	case reflect.Pointer:
		elem, err := newUnmarshaler(typ.Elem())
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
