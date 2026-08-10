package mime

import (
	"bytes"
	"fmt"
	"io"
	"slices"
	"strings"
)

type EncoderOption = func(e *Encoder)

func WithEncoderStripDot(val bool) EncoderOption {
	return func(e *Encoder) {
		e.stripDot = val
	}
}

func WithEncoderSortExtensions(val bool) EncoderOption {
	return func(e *Encoder) {
		e.sortExt = val
	}
}

type Encoder struct {
	w        io.Writer
	stripDot bool
	sortExt  bool
}

func NewEncoder(w io.Writer, options ...EncoderOption) *Encoder {
	e := &Encoder{
		w:        w,
		stripDot: true,
		sortExt:  true,
	}

	for _, option := range options {
		option(e)
	}

	return e
}

func (e *Encoder) Encode(v any) error {
	m, ok := v.(Map)
	if !ok {
		return fmt.Errorf("encode requires Map or map[string]string, got %T", v)
	}

	grouped := make(map[string][]string)

	for ext, mt := range m {
		if e.stripDot {
			ext = strings.TrimPrefix(ext, ".")
		}

		grouped[mt] = append(grouped[mt], ext)
	}

	mimes := make([]string, 0, len(grouped))

	for mt := range grouped {
		mimes = append(mimes, mt)
	}

	slices.Sort(mimes)

	var buf bytes.Buffer

	for i, mt := range mimes {
		if i > 0 {
			fmt.Fprint(&buf, "\n")
		}

		exts := grouped[mt]

		if e.sortExt {
			slices.Sort(exts)
		}

		fmt.Fprint(&buf, mt)

		for _, ext := range exts {
			fmt.Fprint(&buf, " ", ext)
		}
	}

	if len(mimes) > 0 {
		fmt.Fprint(&buf, "\n")
	}

	if _, err := buf.WriteTo(e.w); err != nil {
		return fmt.Errorf("encoding error: %w", err)
	}

	return nil
}

func Marshal(value Map, options ...EncoderOption) ([]byte, error) {
	buf := new(bytes.Buffer)

	if err := NewEncoder(buf, options...).Encode(value); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}
