package tool

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/rand"
	"os"
	"reflect"
	"strconv"
	"strings"
	"unsafe"
)

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

func Bytes2String(b []byte) string {
	return *(*string)(unsafe.Pointer(&b))
}

func String2Bytes(s string) []byte {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  0,
	}
	return *(*[]byte)(unsafe.Pointer(&bh))
}

func MD5(p []byte) string {
	h := md5.New()
	h.Write(p)
	return hex.EncodeToString(h.Sum(nil))
}

func RandomInt(min int, max int) int {
	return min + rand.Intn(max-min)
}

func Camel2Underline(camel string) string {
	var under = make([]rune, 128)
	var i = 0
	for k, c := range camel {
		if c >= 65 && c <= 90 {
			if k > 0 {
				under[i] = '_'
				i++
				under[i] = c + 32
			} else {
				under[i] = c + 32
			}
		} else {
			under[i] = c
		}
		i++
	}
	return string(under[:i])
}

func Underline2Camel(underline string) string {
	var camel = make([]rune, len(underline))
	var i = 0
	for k, c := range underline {
		if k == 0 {
			if c >= 97 && c <= 122 {
				camel[i] = c - 32
			} else if c != '_' {
				camel[i] = c
			}
			i++
		} else if c >= 97 && c <= 122 {
			if underline[k-1] == '_' {
				camel[i] = c - 32
			} else {
				camel[i] = c
			}
			i++
		} else if c != '_' {
			camel[i] = c
			i++
		}
	}

	return string(camel[:i])
}

func deepFields(rtype reflect.Type) (fields []reflect.StructField) {
	for i := 0; i < rtype.NumField(); i++ {
		f := rtype.Field(i)
		if f.Anonymous && f.Type.Kind() == reflect.Struct {
			fields = append(fields, deepFields(f.Type)...)
		} else {
			fields = append(fields, f)
		}
	}
	return
}

// in must be a ptr to struct, or will panic
func StructCopy(in interface{}) (out interface{}) {
	srcT := reflect.TypeOf(in).Elem()
	out = reflect.New(srcT).Interface()
	srcV := reflect.ValueOf(in).Elem()
	dstV := reflect.ValueOf(out).Elem()
	srcfields := deepFields(srcT)
	for _, field := range srcfields {
		if field.Anonymous {
			continue
		}

		dst := dstV.FieldByName(field.Name)
		src := srcV.FieldByName(field.Name)
		if !dst.CanSet() {
			continue
		}
		if src.Type().AssignableTo(dst.Type()) {
			dst.Set(src)
			continue
		}
		if dst.Kind() == reflect.Ptr && dst.Type().Elem() == src.Type() {
			dst.Set(reflect.New(src.Type()))
			dst.Elem().Set(src)
			continue
		}
		if src.Kind() == reflect.Ptr && !src.IsNil() && src.Type().Elem() == dst.Type() {
			dst.Set(src.Elem())
			continue
		}
	}
	return
}

func StructFill(in *map[string]interface{}, out interface{}) error {
	outt := reflect.TypeOf(out)
	outte := outt.Elem()
	if outt.Kind() != reflect.Ptr || outte.Kind() != reflect.Struct {
		return fmt.Errorf("out must be ptr to struct")
	}

	outv := reflect.ValueOf(out)
	if outv.IsNil() {
		return fmt.Errorf("out is nil")
	}

	outve := outv.Elem()
	for k, v := range *in {
		if err := fillStruct(outte, outve, k, v); err != nil {
			return err
		}
	}

	return nil
}

func fillStruct(outType reflect.Type, outValue reflect.Value, name string, value interface{}) error {
	var field reflect.StructField
	if _, found := outType.FieldByName(name); !found {
		for i := 0; i < outType.NumField(); i++ {
			f := outType.Field(i)
			if name == f.Tag.Get("mdb") || name == f.Tag.Get("yaml") || name == f.Tag.Get("json") {
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

/*func StructCopy(in interface{}) (out interface{}) {
	srct := reflect.TypeOf(in)
	srcv := reflect.ValueOf(in)
	if srct.Kind() != reflect.Ptr || srct.Elem().Kind() == reflect.Ptr {
		panic("Fatal error:type of parameter must be Ptr")
	}
	if srcv.IsNil() {
		panic("Fatal error:value of parameter should not be nil")
	}

	out = reflect.New(reflect.TypeOf(in).Elem()).Interface()
	srcV := srcv.Elem()
	dstV := reflect.ValueOf(out).Elem()
	srcfields := deepFields(reflect.ValueOf(in).Elem().Type())
	for _, field := range srcfields {
		if field.Anonymous {
			continue
		}

		dst := dstV.FieldByName(field.Name)
		src := srcV.FieldByName(field.Name)
		if !dst.IsValid() {
			continue
		}
		if src.Type() == dst.Type() && dst.CanSet() {
			dst.Set(src)
			continue
		}
		if src.Kind() == reflect.Ptr && !src.IsNil() && src.Type().Elem() == dst.Type() {
			dst.Set(src.Elem())
			continue
		}
		if dst.Kind() == reflect.Ptr && dst.Type().Elem() == src.Type() {
			dst.Set(reflect.New(src.Type()))
			dst.Elem().Set(src)
			continue
		}
	}
	return
}*/

// The key argument should be the AES key,
// either 16, 24, or 32 bytes to select AES-128, AES-192, or AES-256.
func AesCBCEncrypt(rawData, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	if iv == nil {
		iv = key[:blockSize]
	}
	blockMode := cipher.NewCBCEncrypter(block, iv)
	rawData = pkcs7Padding(rawData, blockSize)
	encData := make([]byte, len(rawData))

	blockMode.CryptBlocks(encData, rawData)
	return encData, nil
}

func AesCBCDecrypt(encData, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	blockSize := block.BlockSize()
	if iv == nil {
		iv = key[:blockSize]
	}
	blockMode := cipher.NewCBCDecrypter(block, iv)
	origData := make([]byte, len(encData))
	blockMode.CryptBlocks(origData, encData)
	origData = pkcs7UnPadding(origData)
	return origData, nil
}

func Encrypt(rawData, key, iv []byte) (string, error) {
	data, err := AesCBCEncrypt(rawData, key, iv)
	if err != nil {
		return "", err
	}
	return base64.RawStdEncoding.EncodeToString(data), nil
}

func Decrypt(encData string, key, iv []byte) ([]byte, error) {
	tmpData, err := base64.RawStdEncoding.DecodeString(encData)
	if err != nil {
		return nil, err
	}

	decData, err := AesCBCDecrypt(tmpData, key, iv)
	if err != nil {
		return nil, err
	}
	return decData, nil
}

func zeroPadding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{0}, padding)
	return append(ciphertext, padtext...)
}

func zeroUnPadding(origData []byte) []byte {
	length := len(origData)
	if length == 0 {
		return origData
	}
	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func pkcs7Padding(ciphertext []byte, blockSize int) []byte {
	padding := blockSize - len(ciphertext)%blockSize
	padtext := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padtext...)
}

func pkcs7UnPadding(origData []byte) []byte {
	length := len(origData)
	if length == 0 {
		return origData
	}

	unpadding := int(origData[length-1])
	return origData[:(length - unpadding)]
}

func ImageFormatCheck(file string) (string, error) {
	f, e := os.OpenFile(file, os.O_RDONLY, 0644)
	if e != nil {
		return "", e
	}
	defer f.Close()

	buf := make([]byte, 8)
	if n, e := f.Read(buf); e != nil {
		return "", e
	} else if n != 8 {
		return "", fmt.Errorf("can't read image")
	}

	if buf[0] == 0xff && buf[1] == 0xd8 && buf[2] == 0xff {
		return "jpg", nil
	} else if buf[0] == 0x89 && buf[1] == 0x50 && buf[2] == 0x4e && buf[3] == 0x47 && buf[4] == 0x0d && buf[5] == 0x0a && buf[6] == 0x1a && buf[7] == 0x0a {
		return "png", nil
	} else if buf[0] == 0x47 && buf[1] == 0x49 && buf[2] == 0x46 && buf[3] == 0x38 && buf[4] == 0x39 && buf[5] == 0x61 {
		return "gif", nil
	} else if buf[0] == 0x42 && buf[1] == 0x4d {
		return "bmp", nil
	}

	return "", fmt.Errorf("unknown image format")
}
