package staticfs

import (
	"io/fs"
	"time"
)

var (
	_ fs.DirEntry = (*Entry)(nil)
	_ fs.FileInfo = (*EntryInfo)(nil)
)

type Entry struct {
	name  string
	size  int64
	isDir bool
	data  []byte
	time  time.Time
}

func (e *Entry) Name() string {
	return e.name
}

func (e *Entry) IsDir() bool {
	return e.isDir
}

func (e *Entry) Type() fs.FileMode {
	if e.isDir {
		return fs.ModeDir
	}

	return 0
}

func (e *Entry) Info() (fs.FileInfo, error) {
	return &EntryInfo{
		entry: e,
	}, nil
}

type EntryInfo struct {
	entry *Entry
}

func (i *EntryInfo) Name() string {
	return i.entry.name
}

func (i *EntryInfo) Size() int64 {
	return i.entry.size
}

func (i *EntryInfo) Mode() fs.FileMode {
	m := fs.FileMode(0444)

	if i.entry.isDir {
		m = 0555
	}

	return i.entry.Type() | m
}

func (i *EntryInfo) ModTime() time.Time {
	return i.entry.time
}

func (i *EntryInfo) IsDir() bool {
	return i.entry.isDir
}

func (i *EntryInfo) Sys() any {
	return nil
}
