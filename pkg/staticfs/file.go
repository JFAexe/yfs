package staticfs

import (
	"fmt"
	"io"
	"io/fs"
)

var (
	_ fs.File       = (*FileHandle)(nil)
	_ io.ReadSeeker = (*FileHandle)(nil)
)

type FileHandle struct {
	entry  *Entry
	path   string
	offset int64
	closed bool
}

func (f *FileHandle) Read(b []byte) (int, error) {
	if f.closed {
		return 0, &fs.PathError{
			Op:   "read",
			Path: f.path,
			Err:  fs.ErrClosed,
		}
	}

	if f.offset >= int64(len(f.entry.data)) {
		return 0, io.EOF
	}

	n := copy(b, f.entry.data[f.offset:])

	f.offset += int64(n)

	return n, nil
}

func (f *FileHandle) Seek(offset int64, whence int) (int64, error) {
	if f.closed {
		return 0, &fs.PathError{
			Op:   "seek",
			Path: f.path,
			Err:  fs.ErrClosed,
		}
	}

	var abs int64

	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs = f.offset + offset
	case io.SeekEnd:
		abs = int64(len(f.entry.data)) + offset
	default:
		return 0, &fs.PathError{
			Op:   "seek",
			Path: f.path,
			Err:  fmt.Errorf("invalid whence: %d", whence),
		}
	}

	if abs < 0 {
		return 0, &fs.PathError{
			Op:   "seek",
			Path: f.path,
			Err:  fmt.Errorf("negative seek position: %d", abs),
		}
	}

	f.offset = abs

	return abs, nil
}

func (f *FileHandle) Close() error {
	if f.closed {
		return &fs.PathError{
			Op:   "close",
			Path: f.path,
			Err:  fs.ErrClosed,
		}
	}

	f.closed = true

	return nil
}

func (f *FileHandle) Stat() (fs.FileInfo, error) {
	if f.closed {
		return nil, &fs.PathError{
			Op:   "stat",
			Path: f.path,
			Err:  fs.ErrClosed,
		}
	}

	return f.entry.Info()
}
