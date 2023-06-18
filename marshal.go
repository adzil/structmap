package dstruct

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	_ marshaler = (*pointerMarshaler)(nil)
	_ marshaler = (*structMarshaler)(nil)
	_ marshaler = (*stringMarshaler)(nil)
)

var (
	errMissingValue = errors.New("missing required value")
)

type ValueMarshaler interface {
	MarshalValue() ([]string, error)
}

type marshaler interface {
	marshal(src reflect.Value, v map[string][]string) error
}

type pointerMarshaler struct {
	required bool
	elem     marshaler
}

func (m *pointerMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	if !src.IsNil() {
		return m.elem.marshal(src, v)
	}

	if m.required {
		return errMissingValue
	}

	return nil
}

type fieldMarshaler struct {
	index     int
	marshaler marshaler
}

type structMarshaler struct {
	fields []fieldMarshaler
}

func (m *structMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	for _, field := range m.fields {
		if err := field.marshaler.marshal(src.Field(field.index), v); err != nil {
			return err
		}
	}

	return nil
}

type stringMarshaler struct {
	key       string
	required  bool
	omitempty bool
}

func (m *stringMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	val := src.String()

	if val == "" {
		if m.required {
			return errMissingValue
		}

		if m.omitempty {
			return nil
		}
	}

	v[m.key] = append(v[m.key][:0], val)

	return nil
}

type MarshalConfig struct {
	Delimiter string
}

func (cfg MarshalConfig) Marshal(src any, v map[string][]string) error {
	return defaultMarshaler.Marshal(cfg, src, v)
}

func (c MarshalConfig) delimiter() string {
	if c.Delimiter != "" {
		return c.Delimiter
	}

	return "."
}

type marshalConfig struct {
	MarshalConfig
	Name         []string
	NamelessAnon bool
	Required     bool
	OmitEmpty    bool
}

func (c *marshalConfig) applyOption(opt string) {
	switch opt {
	case "required":
		c.Required = true
	case "omitempty":
		c.OmitEmpty = true
	}
}

func newValueMarshaler(cfg marshalConfig, typ reflect.Type) (marshaler, error) {
	switch typ.Kind() {
	case reflect.Pointer:
		mv, err := newValueMarshaler(cfg, typ.Elem())
		if err != nil {
			return nil, err
		}

		return &pointerMarshaler{
			required: cfg.Required,
			elem:     mv,
		}, nil

	case reflect.Struct:
		if cfg.Required || cfg.OmitEmpty {
			return nil, errors.New("cannot set any option for struct")
		}

		if cfg.NamelessAnon {
			cfg.Name = cfg.Name[:len(cfg.Name)-1]
		}

		return newStructMarshaler(cfg, typ)

	case reflect.String:
		return &stringMarshaler{
			key:       strings.Join(cfg.Name, cfg.delimiter()),
			required:  cfg.Required,
			omitempty: cfg.OmitEmpty,
		}, nil
	}

	return nil, fmt.Errorf("cannot marshal from %s", typ.Kind().String())
}

func newFieldMarshaler(cfg marshalConfig, structFld reflect.StructField) (fieldMarshaler, error) {
	tag := strings.Split(structFld.Tag.Get("map"), ",")
	name := tag[0]

	// Follow the encoding/json standard where a field can still be named "-"
	// by using a comma suffix.
	if name == "-" && len(tag) == 1 {
		return fieldMarshaler{}, errSkipField
	}

	var namelessAnon bool
	if name == "" {
		if structFld.Anonymous {
			namelessAnon = true
		}

		name = structFld.Name
	}

	fieldCfg := marshalConfig{
		MarshalConfig: cfg.MarshalConfig,
		Name:          append(cfg.Name, name),
		NamelessAnon:  namelessAnon,
	}

	for i := 1; i < len(tag); i++ {
		fieldCfg.applyOption(tag[i])
	}

	if fieldCfg.Required && fieldCfg.OmitEmpty {
		return fieldMarshaler{}, errors.New("a field cannot be set as both required and omitempty")
	}

	vm, err := newValueMarshaler(fieldCfg, structFld.Type)
	if err != nil {
		return fieldMarshaler{}, fmt.Errorf("struct field %s: %w", structFld.Name, err)
	}

	return fieldMarshaler{
		index:     structFld.Index[len(structFld.Index)-1],
		marshaler: vm,
	}, nil
}

func newStructMarshaler(cfg marshalConfig, typ reflect.Type) (marshaler, error) {
	var fields []fieldMarshaler

	n := typ.NumField()
	for i := 0; i < n; i++ {
		field, err := newFieldMarshaler(cfg, typ.Field(i))

		if errors.Is(err, errSkipField) {
			continue
		}

		if err != nil {
			return nil, err
		}

		fields = append(fields, field)
	}

	return &structMarshaler{
		fields: fields,
	}, nil
}

func newMarshaler(cfg marshalConfig, typ reflect.Type) (marshaler, error) {
	switch typ.Kind() {
	case reflect.Struct:
		return newStructMarshaler(cfg, typ)

	case reflect.Pointer:
		elem, err := newMarshaler(cfg, typ.Elem())
		if err != nil {
			return nil, err
		}

		return &pointerMarshaler{
			required: true,
			elem:     elem,
		}, nil
	}

	return nil, fmt.Errorf("cannot marshal from %s", typ.Kind().String())
}

type marshalerCacheKey struct {
	typ    reflect.Type
	config MarshalConfig
}

type marshalerCache struct {
	cache cache[marshalerCacheKey, marshaler]
}

func (mc *marshalerCache) Marshal(cfg MarshalConfig, src any, v map[string][]string) error {
	if v == nil {
		return errors.New("cannot marshal into a nil map")
	}

	val := reflect.ValueOf(src)

	key := marshalerCacheKey{
		typ:    val.Type(),
		config: cfg,
	}

	vm, err := mc.cache.Get(key, func(k marshalerCacheKey) (marshaler, error) {
		return newMarshaler(marshalConfig{MarshalConfig: key.config}, k.typ)
	})
	if err != nil {
		return err
	}

	return vm.marshal(val, v)
}

var defaultMarshaler marshalerCache

func Marshal(src any, v map[string][]string) error {
	return defaultMarshaler.Marshal(MarshalConfig{}, src, v)
}
