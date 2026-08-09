package staticfs

import (
	"io"
	"io/fs"
)

var (
	_ fs.File        = (*DirHandle)(nil)
	_ fs.ReadDirFile = (*DirHandle)(nil)
)

type DirHandle struct {
	fs     *FS
	path   string
	offset int64
	closed bool
}

func (d *DirHandle) Read(_ []byte) (int, error) {
	if d.closed {
		return 0, &fs.PathError{
			Op:   "read",
			Path: d.path,
			Err:  fs.ErrClosed,
		}
	}

	return 0, &fs.PathError{
		Op:   "read",
		Path: d.path,
		Err:  ErrIsDir,
	}
}

func (d *DirHandle) Close() error {
	if d.closed {
		return &fs.PathError{
			Op:   "close",
			Path: d.path,
			Err:  fs.ErrClosed,
		}
	}

	d.closed = true

	return nil
}

func (d *DirHandle) Stat() (fs.FileInfo, error) {
	if d.closed {
		return nil, &fs.PathError{
			Op:   "stat",
			Path: d.path,
			Err:  fs.ErrClosed,
		}
	}

	return d.fs.Stat(d.path)
}

func (d *DirHandle) ReadDir(n int) ([]fs.DirEntry, error) {
	if d.closed {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: d.path,
			Err:  fs.ErrClosed,
		}
	}

	var (
		cache = d.fs.cache[d.path]
		size  = int64(len(cache))
	)

	if d.offset >= size {
		if n > 0 {
			return nil, io.EOF
		}

		return []fs.DirEntry{}, nil
	}

	end := size

	if n > 0 {
		end = min(d.offset+int64(n), end)
	}

	var (
		sli = cache[d.offset:end]
		out = make([]fs.DirEntry, len(sli))
	)

	copy(out, sli)

	d.offset = end

	return out, nil
}
