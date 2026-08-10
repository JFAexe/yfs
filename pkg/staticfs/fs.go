package staticfs

import (
	"fmt"
	"io/fs"
	"path"
	"slices"
	"strings"
	"time"
)

var (
	ErrIsDir  = fmt.Errorf("%w: path is a directory", fs.ErrInvalid)
	ErrIsFile = fmt.Errorf("%w: path is a file", fs.ErrInvalid)
)

var (
	_ fs.FS         = (*FS)(nil)
	_ fs.ReadFileFS = (*FS)(nil)
	_ fs.ReadDirFS  = (*FS)(nil)
	_ fs.StatFS     = (*FS)(nil)
)

type File struct {
	Path string
	Data []byte
}

type Option = func(f *FS)

func WithOverwrite(allow bool) Option {
	return func(f *FS) {
		f.overwrite = allow
	}
}

type FS struct {
	entries map[string]*Entry
	cache   map[string][]fs.DirEntry

	overwrite bool
}

func New(files []File, options ...Option) (*FS, error) {
	sfs := new(FS)

	for _, option := range options {
		option(sfs)
	}

	if err := sfs.init(files); err != nil {
		return nil, fmt.Errorf("failed to init fs: %w", err)
	}

	return sfs, nil
}

func (s *FS) Open(name string) (fs.File, error) {
	if name = path.Clean(name); !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "open",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	if entry, ok := s.entries[name]; ok {
		if !entry.isDir {
			return &FileHandle{
				path:  name,
				entry: entry,
			}, nil
		}

		return &DirHandle{
			path: name,
			fs:   s,
		}, nil
	}

	return nil, &fs.PathError{
		Op:   "open",
		Path: name,
		Err:  fs.ErrNotExist,
	}
}

func (s *FS) ReadFile(name string) ([]byte, error) {
	if name = path.Clean(name); !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "readfile",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	entry, ok := s.entries[name]
	if !ok {
		return nil, &fs.PathError{
			Op:   "readfile",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}

	if entry.isDir {
		return nil, &fs.PathError{
			Op:   "readfile",
			Path: name,
			Err:  ErrIsDir,
		}
	}

	data := make([]byte, len(entry.data))

	copy(data, entry.data)

	return data, nil
}

func (s *FS) ReadDir(name string) ([]fs.DirEntry, error) {
	if name = path.Clean(name); !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	entry, ok := s.entries[name]
	if !ok {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: name,
			Err:  fs.ErrNotExist,
		}
	}

	if !entry.isDir {
		return nil, &fs.PathError{
			Op:   "readdir",
			Path: name,
			Err:  ErrIsFile,
		}
	}

	var (
		cache = s.cache[name]
		out   = make([]fs.DirEntry, len(cache))
	)

	copy(out, cache)

	return out, nil
}

func (s *FS) Stat(name string) (fs.FileInfo, error) {
	if name = path.Clean(name); !fs.ValidPath(name) {
		return nil, &fs.PathError{
			Op:   "stat",
			Path: name,
			Err:  fs.ErrInvalid,
		}
	}

	if entry, ok := s.entries[name]; ok {
		return &EntryInfo{
			entry: entry,
		}, nil
	}

	return nil, &fs.PathError{
		Op:   "stat",
		Path: name,
		Err:  fs.ErrNotExist,
	}
}

func (s *FS) init(files []File) error {
	now := time.Now()

	s.entries = map[string]*Entry{
		".": {
			name:  ".",
			isDir: true,
			time:  now,
		},
	}

	for _, file := range files {
		clean := path.Clean(file.Path)

		if clean == "." || !fs.ValidPath(clean) {
			return fmt.Errorf("invalid file path %#q: %w", file.Path, fs.ErrInvalid)
		}

		if e, exists := s.entries[clean]; exists {
			if !s.overwrite {
				if e.isDir {
					return fmt.Errorf("path conflict: cannot add %#q as a file because it is already a directory", clean)
				}

				return fmt.Errorf("duplicate file path: %#q", clean)
			}

			if e.isDir {
				prefix := clean + "/"

				for p := range s.entries {
					if p == clean || strings.HasPrefix(p, prefix) {
						delete(s.entries, p)
					}
				}
			}
		}

		for dir := path.Dir(clean); dir != "."; dir = path.Dir(dir) {
			if e, exists := s.entries[dir]; exists {
				if e.isDir {
					continue
				}

				if !s.overwrite {
					return fmt.Errorf("path conflict: cannot add %#q because %#q is already a file", clean, dir)
				}
			}

			s.entries[dir] = &Entry{
				name:  path.Base(dir),
				isDir: true,
				time:  now,
			}
		}

		s.entries[clean] = &Entry{
			name: path.Base(clean),
			data: file.Data,
			size: int64(len(file.Data)),
			time: now,
		}
	}

	s.cache = make(map[string][]fs.DirEntry)

	for p, e := range s.entries {
		if p == "." {
			continue
		}

		dir := path.Dir(p)

		s.cache[dir] = append(s.cache[dir], e)
	}

	for _, entries := range s.cache {
		slices.SortFunc(entries, func(a, b fs.DirEntry) int { return strings.Compare(a.Name(), b.Name()) })
	}

	return nil
}
