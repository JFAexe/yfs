package mime

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime"
	"reflect"
	"strings"
)

var ErrBadExtension = errors.New("bad file extension")

type DecoderOption func(d *Decoder)

func WithDecoderNormalize(val bool) DecoderOption {
	return func(d *Decoder) {
		d.normalize = val
	}
}

func WithDecoderAddDot(val bool) DecoderOption {
	return func(d *Decoder) {
		d.addDot = val
	}
}

type Decoder struct {
	r         io.Reader
	normalize bool
	addDot    bool
}

func NewDecoder(r io.Reader, options ...DecoderOption) *Decoder {
	d := &Decoder{
		r:         r,
		normalize: true,
		addDot:    true,
	}

	for _, option := range options {
		option(d)
	}

	return d
}

func (d *Decoder) Decode(v any) error {
	if v == nil {
		return fmt.Errorf("cannot decode into nil value")
	}

	rv := reflect.ValueOf(v)

	if rv.Kind() != reflect.Pointer {
		return fmt.Errorf("decode requires a pointer, got %T", v)
	}

	if rv.IsNil() {
		rv.Set(reflect.New(rv.Type().Elem()))
	}

	var (
		re = rv.Elem()
		rk = re.Kind()
	)

	if rk != reflect.Interface && rk != reflect.Map {
		return fmt.Errorf("only *map[string]string or *any is supported, got %T", v)
	}

	if rk == reflect.Map && (re.Type().Key().Kind() != reflect.String || re.Type().Elem().Kind() != reflect.String) {
		return fmt.Errorf("only types like map[string]string are supported, got %s", re.Type())
	}

	out, err := d.decode()
	if err != nil {
		return fmt.Errorf("failed to decode data: %w", err)
	}

	switch rk {
	case reflect.Interface:
		re.Set(reflect.ValueOf(out))
	case reflect.Map:
		if re.IsNil() {
			re.Set(reflect.MakeMap(re.Type()))
		}

		for k, val := range out {
			re.SetMapIndex(reflect.ValueOf(k), reflect.ValueOf(val))
		}
	}

	return nil
}

func (d *Decoder) decode() (Map, error) {
	var (
		out     = make(Map)
		scanner = bufio.NewScanner(d.r)
	)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if len(line) == 0 || strings.HasPrefix(line, "#") {
			continue
		}

		fields := strings.Fields(line)

		if len(fields) < 2 {
			continue
		}

		t := fields[0]

		if d.normalize {
			parsed, _, err := mime.ParseMediaType(t)
			if err != nil {
				return nil, err
			}

			t = parsed
		}

		for _, ext := range fields[1:] {
			e, err := ParseExtension(ext)
			if err != nil {
				return nil, err
			}

			out[e] = t
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return out, nil
}

func Unmarshal(data []byte, v any, options ...DecoderOption) error {
	if err := NewDecoder(bytes.NewReader(data), options...).Decode(v); err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	return nil
}

func ParseMap(mimes Map) (m Map, err error) {
	m = make(Map, len(mimes))

	for e, t := range mimes {
		if e, err = ParseExtension(e); err != nil {
			return nil, err
		}

		if t, _, err = mime.ParseMediaType(t); err != nil {
			return nil, err
		}

		m[e] = t
	}

	return m, nil
}

func ParseExtension(e string) (string, error) {
	if e = strings.TrimSpace(e); e == "" {
		return "", ErrBadExtension
	}

	if !strings.HasPrefix(e, ".") {
		e = "." + e
	}

	return strings.ToLower(e), nil
}
