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
	"net/http"
	"reflect"
	"strconv"
	"strings"
)

var (
	_ marshaler = (*pointerMarshaler)(nil)
	_ marshaler = (*structMarshaler)(nil)
	_ marshaler = (*stringMarshaler)(nil)
	_ marshaler = (*intMarshaler)(nil)
	_ marshaler = (*methodMarshaler)(nil)
	_ marshaler = (*sliceMarshaler)(nil)
)

var (
	errMissingValue = errors.New("missing required value")
)

var (
	valueMarshalerReflectType = reflect.TypeOf((*ValueMarshaler)(nil)).Elem()
)

var (
	DefaultMarshaler Marshaler

	HeaderMarshaler = Marshaler{
		config: MarshalConfig{
			KeyLookupFunc: http.CanonicalHeaderKey,
		},
	}
)

type ValueMarshaler interface {
	MarshalValue() ([]string, error)
}

type marshaler interface {
	marshal(src reflect.Value, v map[string][]string) error
}

type pointerMarshaler struct {
	key      string
	required bool
	elem     marshaler
}

func (m *pointerMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	if !src.IsNil() {
		return m.elem.marshal(src, v)
	}

	if m.required {
		if m.key == "" {
			return errMissingValue
		}

		return fmt.Errorf("key %s: %w", m.key, errMissingValue)
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

type keyMarshaler struct {
	key       string
	required  bool
	omitEmpty bool
}

func newKeyMarshaler(cfg marshalConfig) keyMarshaler {
	return keyMarshaler{
		key:       cfg.name(),
		required:  cfg.Required,
		omitEmpty: cfg.OmitEmpty,
	}
}

type stringMarshaler struct {
	keyMarshaler
}

func (m *stringMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	val := src.String()

	if val == "" {
		if m.required {
			return fmt.Errorf("key %s: %w", m.key, errMissingValue)
		}

		if m.omitEmpty {
			return nil
		}
	}

	v[m.key] = append(v[m.key][:0], val)

	return nil
}

type intMarshaler struct {
	keyMarshaler
}

func (m *intMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	val := src.Int()

	if val == 0 {
		if m.required {
			return fmt.Errorf("key %s: %w", m.key, errMissingValue)
		}

		if m.omitEmpty {
			return nil
		}
	}

	v[m.key] = append(v[m.key][:0], strconv.FormatInt(val, 10))

	return nil
}

type methodMarshaler struct {
	keyMarshaler
	ptrReceiver bool
}

func (m *methodMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	if m.ptrReceiver {
		if !src.CanAddr() {
			return errors.New("unable to call MarshalValue to an unadressable value")
		}

		src = src.Addr()
	}

	val, err := src.Interface().(ValueMarshaler).MarshalValue()
	if err != nil {
		return err
	}

	if len(val) == 0 {
		if m.required {
			return fmt.Errorf("key %s: %w", m.key, errMissingValue)
		}

		if m.omitEmpty {
			return nil
		}
	}

	v[m.key] = append(v[m.key][:0], val...)

	return nil
}

type sliceMarshaler struct {
	keyMarshaler
	intElem bool
}

func (m *sliceMarshaler) marshal(src reflect.Value, v map[string][]string) error {
	n := src.Len()

	if n == 0 {
		if m.required {
			return fmt.Errorf("key %s: %w", m.key, errMissingValue)
		}

		if m.omitEmpty {
			return nil
		}
	}

	out := v[m.key][:0]

	for i := 0; i < n; i++ {
		var val string

		if m.intElem {
			val = strconv.FormatInt(src.Index(i).Int(), 10)
		} else {
			val = src.Index(i).String()
		}

		out = append(out, val)
	}

	v[m.key] = out

	return nil
}

type MarshalConfig struct {
	Delimiter     string
	KeyLookupFunc func(s string) string
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

func (c *marshalConfig) applyOption(opt string) error {
	switch opt {
	case "required":
		c.Required = true
	case "omitempty":
		c.OmitEmpty = true
	case "":
		// Allow empty option.
	default:
		return fmt.Errorf("unknown option %s", opt)
	}

	return nil
}

func (c *marshalConfig) name() string {
	key := strings.Join(c.Name, c.delimiter())

	if c.KeyLookupFunc != nil {
		key = c.KeyLookupFunc(key)
	}

	return key
}

func newSliceMarshaler(cfg marshalConfig, typ reflect.Type) (marshaler, error) {
	elem := typ.Elem()

	switch elem.Kind() {
	case reflect.String:
		return &sliceMarshaler{keyMarshaler: newKeyMarshaler(cfg)}, nil

	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return &sliceMarshaler{
			keyMarshaler: newKeyMarshaler(cfg),
			intElem:      true,
		}, nil
	}

	return nil, fmt.Errorf("cannot marshal from slice of %s", elem.Kind().String())
}

func newValueMarshaler(cfg marshalConfig, typ reflect.Type) (marshaler, error) {
	var valReceiver bool

	switch {
	case typ.Implements(valueMarshalerReflectType):
		valReceiver = true

		fallthrough

	case reflect.PointerTo(typ).Implements(valueMarshalerReflectType):
		return &methodMarshaler{
			keyMarshaler: newKeyMarshaler(cfg),
			ptrReceiver:  !valReceiver,
		}, nil
	}

	switch typ.Kind() {
	case reflect.Pointer:
		mv, err := newValueMarshaler(cfg, typ.Elem())
		if err != nil {
			return nil, err
		}

		return &pointerMarshaler{
			key:      cfg.name(),
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

	case reflect.Slice:
		return newSliceMarshaler(cfg, typ)

	case reflect.String:
		return &stringMarshaler{keyMarshaler: newKeyMarshaler(cfg)}, nil

	case reflect.Int, reflect.Int64, reflect.Int32, reflect.Int16, reflect.Int8:
		return &intMarshaler{keyMarshaler: newKeyMarshaler(cfg)}, nil
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
		if err := fieldCfg.applyOption(tag[i]); err != nil {
			return fieldMarshaler{}, err
		}
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

type Marshaler struct {
	cache  cache[reflect.Type, marshaler]
	config MarshalConfig
}

func (m *Marshaler) Marshal(src any, v map[string][]string) error {
	if v == nil {
		return errors.New("cannot marshal into a nil map")
	}

	val := reflect.ValueOf(src)

	vm, err := m.cache.Get(val.Type(), func(key reflect.Type) (marshaler, error) {
		return newMarshaler(marshalConfig{MarshalConfig: m.config}, key)
	})
	if err != nil {
		return err
	}

	return vm.marshal(val, v)
}

func NewMarshaler(cfg MarshalConfig) *Marshaler {
	return &Marshaler{
		config: cfg,
	}
}

func Marshal(src any, v map[string][]string) error {
	return DefaultMarshaler.Marshal(src, v)
}

func MarshalHeader(src any, v http.Header) error {
	return HeaderMarshaler.Marshal(src, v)
}
