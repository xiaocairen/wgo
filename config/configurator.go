package config

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"strconv"
	"strings"
)

type Configurator struct {
	data map[string]any
}

func New(file string) *Configurator {
	c := &Configurator{}
	res, err := ioutil.ReadFile(file)
	if err != nil {
		panic(err)
	}
	if len(res) == 0 {
		panic("app.json is empty")
	}
	if err := json.Unmarshal(res, &c.data); err != nil {
		panic(err)
	}

	return c
}

func (c *Configurator) Get(path string) (out any, err error) {
	paths := strings.Split(path, ".")
	var (
		m map[string]any
		n = len(paths)
		f bool
	)
	if n > 1 {
		if err = c.getMap(paths[:n-1], c.data, &m); err != nil {
			return
		}
	} else {
		m = c.data
	}

	if out, f = m[paths[n-1]]; !f {
		err = fmt.Errorf("not found key '%s'", path)
	}
	return
}

func (c *Configurator) GetStruct(path string, out any) error {
	paths := strings.Split(path, ".")
	m := make(map[string]any)
	if err := c.getMap(paths, c.data, &m); err != nil {
		return err
	}

	tmp, err := json.Marshal(m)
	if err != nil {
		return err
	}
	return json.Unmarshal(tmp, out)
}

func (c *Configurator) GetMap(path string, out *map[string]any) error {
	paths := strings.Split(path, ".")
	if err := c.getMap(paths, c.data, out); err != nil {
		return err
	}
	return nil
}

func (c *Configurator) GetBool(path string, out *bool) error {
	paths := strings.Split(path, ".")
	var (
		m   map[string]any
		n   = len(paths)
		err error
	)
	if n > 1 {
		if err = c.getMap(paths[:n-1], c.data, &m); err != nil {
			return err
		}
	} else {
		m = c.data
	}

	v, f := m[paths[n-1]]
	if !f {
		return fmt.Errorf("not found key '%s'", path)
	}

	b, ok := v.(bool)
	if !ok {
		return fmt.Errorf("the value of key '%s' is not bool", path)
	}

	*out = b
	return nil
}

func (c *Configurator) GetInt(path string, out *int) error {
	paths := strings.Split(path, ".")
	var (
		m   map[string]any
		n   = len(paths)
		err error
	)
	if n > 1 {
		if err = c.getMap(paths[:n-1], c.data, &m); err != nil {
			return err
		}
	} else {
		m = c.data
	}

	v, f := m[paths[n-1]]
	if !f {
		return fmt.Errorf("not found key '%s'", path)
	}

	val, ok := v.(float64)
	if !ok {
		return fmt.Errorf("the value of key '%s' is not int", path)
	}

	*out = int(val)
	return nil
}

func (c *Configurator) GetStr(path string, out *string) error {
	paths := strings.Split(path, ".")
	var (
		m   map[string]any
		n   = len(paths)
		err error
	)
	if n > 1 {
		if err = c.getMap(paths[:n-1], c.data, &m); err != nil {
			return err
		}
	} else {
		m = c.data
	}

	v, f := m[paths[n-1]]
	if !f {
		return fmt.Errorf("not found key '%s'", path)
	}

	val, ok := v.(string)
	if !ok {
		return fmt.Errorf("the value of key '%s' is not string", path)
	}

	*out = val
	return nil
}

func (c *Configurator) GetSlice(path string, out *[]any) error {
	paths := strings.Split(path, ".")
	var (
		m   map[string]any
		n   = len(paths)
		err error
	)
	if n > 1 {
		if err = c.getMap(paths[:n-1], c.data, &m); err != nil {
			return err
		}
	} else {
		m = c.data
	}

	v, f := m[paths[n-1]]
	if !f {
		return fmt.Errorf("not found key '%s'", path)
	}

	val, ok := v.([]any)
	if !ok {
		return fmt.Errorf("the value of key '%s' is not string", path)
	}

	*out = val
	return nil
}

func (c *Configurator) getMap(paths []string, in map[string]any, out *map[string]any) error {
	d, f := in[paths[0]]
	if !f {
		return fmt.Errorf("not found key '%s'", paths[0])
	}

	tmp, ok := d.(map[string]any)
	if !ok {
		return fmt.Errorf("the value of key '%s' is not map", paths[0])
	}

	if len(paths[1:]) > 0 {
		return c.getMap(paths[1:], tmp, out)
	}

	*out = tmp
	return nil
}

const (
	INT8_MIN   = -128
	INT8_MAX   = 127
	INT16_MIN  = -32768
	INT16_MAX  = 32767
	INT32_MIN  = -2147483648
	INT32_MAX  = 2147483647
	INT64_MIN  = -9223372036854775808
	INT64_MAX  = 9223372036854775807
	UINT8_MAX  = 255
	UINT16_MAX = 65535
	UINT32_MAX = 4294967295
	UINT64_MAX = 18446744073709551615
)

func (c *Configurator) fillStruct(outType reflect.Type, outValue reflect.Value, name string, value any) error {
	var field reflect.StructField
	if _, found := outType.FieldByName(name); !found {
		for i := 0; i < outType.NumField(); i++ {
			f := outType.Field(i)
			if name == f.Tag.Get("json") {
				field = f
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("not found '%s' in struct", name)
		}
	}

	org := outValue.FieldByName(field.Name)
	val := reflect.ValueOf(value)
	orgType := org.Type()
	valType := val.Type()
	if valType.AssignableTo(orgType) {
		org.Set(val)
	} else {
		valKind := valType.Kind()
		switch orgType.Kind() {
		case reflect.Bool:
			switch valKind {
			case reflect.Float64:
				if 0 == value {
					org.Set(reflect.ValueOf(false))
				} else {
					org.Set(reflect.ValueOf(true))
				}
			case reflect.String:
				s, _ := value.(string)
				s = strings.ToLower(strings.TrimSpace(s))
				if "0" == s || "" == s || "false" == s || "null" == s || "nil" == s {
					org.Set(reflect.ValueOf(false))
				} else {
					org.Set(reflect.ValueOf(true))
				}
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo bool '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo bool '%s'", name, field.Name)
			}

		case reflect.Int:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				org.Set(reflect.ValueOf(int(f)))
			case reflect.String:
				s, _ := value.(string)
				// strconv.ParseInt(s, 10, 64)
				if i, e := strconv.Atoi(s); e != nil {
					return fmt.Errorf("string value '%s' of %s can't assignableTo int '%s'", s, name, field.Name)
				} else {
					org.Set(reflect.ValueOf(i))
				}
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo int '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo int '%s'", name, field.Name)
			}

		case reflect.Int8:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < INT8_MIN || f > INT8_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo int8 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(int8(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < INT8_MIN || i > INT8_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo int8 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(int8(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo int8 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo int8 '%s'", name, field.Name)
			}

		case reflect.Int16:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < INT16_MIN || f > INT16_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo int16 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(int16(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < INT16_MIN || i > INT16_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo int16 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(int16(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo int16 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo int16 '%s'", name, field.Name)
			}

		case reflect.Int32:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < INT32_MIN || f > INT32_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo int32 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(int32(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < INT32_MIN || i > INT32_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo int32 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(int32(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo int32 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo int32 '%s'", name, field.Name)
			}

		case reflect.Int64:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				org.Set(reflect.ValueOf(int64(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil {
					return fmt.Errorf("string value '%s' of %s can't assignableTo int64 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(int64(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo int64 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo int64 '%s'", name, field.Name)
			}

		case reflect.Uint:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < 0 {
					return fmt.Errorf("value %t of '%s' can't assignableTo uint '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < 0 {
					return fmt.Errorf("string value '%s' of %s can't assignableTo uint '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo uint '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo uint '%s'", name, field.Name)
			}

		case reflect.Uint8:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < 0 || f > UINT8_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo uint8 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint8(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < 0 || i > UINT8_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo uint8 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint8(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo uint8 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo uint8 '%s'", name, field.Name)
			}

		case reflect.Uint16:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < 0 || f > UINT16_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo uint16 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint16(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < 0 || i > UINT16_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo uint16 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint16(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo uint16 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo uint16 '%s'", name, field.Name)
			}

		case reflect.Uint32:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < 0 || f > UINT32_MAX {
					return fmt.Errorf("value %t of '%s' can't assignableTo uint32 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint32(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < 0 || i > UINT32_MAX {
					return fmt.Errorf("string value '%s' of %s can't assignableTo uint32 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint32(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo uint32 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo uint32 '%s'", name, field.Name)
			}

		case reflect.Uint64:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				if f < 0 {
					return fmt.Errorf("value %t of '%s' can't assignableTo uint64 '%s'", f, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint64(f)))
			case reflect.String:
				s, _ := value.(string)
				i, e := strconv.Atoi(s)
				if e != nil || i < 0 {
					return fmt.Errorf("string value '%s' of %s can't assignableTo uint64 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(uint64(i)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo uint64 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo uint64 '%s'", name, field.Name)
			}

		case reflect.Float32:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				org.Set(reflect.ValueOf(float32(f)))
			case reflect.String:
				s, _ := value.(string)
				f, e := strconv.ParseFloat(s, 32)
				if e != nil {
					return fmt.Errorf("string value '%s' of %s can't assignableTo float32 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(float32(f)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo float32 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo float32 '%s'", name, field.Name)
			}

		case reflect.Float64:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf(1))
				} else {
					org.Set(reflect.ValueOf(0))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				org.Set(reflect.ValueOf(f))
			case reflect.String:
				s, _ := value.(string)
				f, e := strconv.ParseFloat(s, 64)
				if e != nil {
					return fmt.Errorf("string value '%s' of %s can't assignableTo float64 '%s'", s, name, field.Name)
				}
				org.Set(reflect.ValueOf(f))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo float64 '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo float64 '%s'", name, field.Name)
			}

		case reflect.String:
			switch valKind {
			case reflect.Bool:
				if true == value {
					org.Set(reflect.ValueOf("true"))
				} else {
					org.Set(reflect.ValueOf("false"))
				}
			case reflect.Float64:
				f, _ := value.(float64)
				org.Set(reflect.ValueOf(strconv.FormatFloat(f, 'f', -1, 64)))
			case reflect.Slice:
				return fmt.Errorf("slice '%s' can't assignableTo string '%s'", name, field.Name)
			case reflect.Map:
				return fmt.Errorf("map '%s' can't assignableTo string '%s'", name, field.Name)
			}

		case reflect.Array, reflect.Interface, reflect.Map, reflect.Slice, reflect.Struct:
			return fmt.Errorf("not support '%s', struct element '%s' is %s", orgType.Kind().String(), field.Name, orgType.Kind().String())
		}
	}

	return nil
}
