package fileserver

import (
	"bytes"
	"errors"
	"html/template"
	"io/fs"
	"net/http"
	"net/url"
	"path"
	"slices"
	"strings"
	"time"
)

const (
	defaultRootPath = "$"
	defaultTemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>{{ .Name }}</title>
  <style>
    :root { font-family: monospace; font-size: 62.5%; background: #181818; color: #e8e8e8 }
    body { margin: 0 3rem 3rem; font-size: 1.6rem; word-break: break-all; }
    table { border-collapse: collapse; width: 100%; }
    th { padding: 3rem 0 1.4rem 0; font-size: 2em; background: #181818; position: sticky; top: 0; z-index: 1; text-align: left; white-space: nowrap; }
    th span { margin: 0 0.6rem }
    td { padding: 0.3rem 0; font-size: 1.4em; }
    a, a:visited { text-decoration: none; color: inherit }
    a:hover { text-decoration: underline }
  </style>
</head>
<body>
  <table>
    <tr><th>{{ range $i, $c := .Crumbs }}{{ if $c.Disable }}{{ $c.Name }}{{ else }}<a href="{{ $c.URL }}">{{ $c.Name }}</a>{{ end }}<span>/</span>{{ end }}</th></tr>
    {{ range .Entries }}<tr><td><a href="{{ .URL }}">{{ .Name }}</a></td></tr>{{ end }}
  </table>
</body>
</html>`
)

type dir struct {
	Name    string
	Crumbs  []crumb
	Entries []entry
}

type crumb struct {
	URL     string
	Name    string
	Disable bool
}

type entry struct {
	URL   string
	Name  string
	IsDir bool
}

type FileServerOption func(s *FileServer)

func WithRootPath(r string) FileServerOption {
	return func(s *FileServer) {
		if r = strings.TrimSpace(r); r == "" {
			r = defaultRootPath
		}

		s.rootPath = r
	}
}

type FileServer struct {
	filesystem http.FileSystem
	template   *template.Template
	rootPath   string
}

func New(root fs.FS, options ...FileServerOption) *FileServer {
	s := &FileServer{
		filesystem: http.FS(root),
		template:   template.Must(template.New("viewer").Parse(defaultTemplate)),
		rootPath:   defaultRootPath,
	}

	for _, option := range options {
		option(s)
	}

	return s
}

func (s *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	f, err := s.filesystem.Open(r.URL.Path)
	if err != nil {
		s.error(w, err)

		return
	}
	defer f.Close() //nolint:errcheck

	info, err := f.Stat()
	if err != nil {
		s.error(w, err)

		return
	}

	if !info.IsDir() {
		http.ServeContent(w, r, info.Name(), info.ModTime(), f)

		return
	}

	if !strings.HasSuffix(r.URL.Path, "/") {
		u := *r.URL
		u.Path += "/"

		http.Redirect(w, r, u.String(), http.StatusPermanentRedirect)

		return
	}

	if m := info.ModTime(); !m.IsZero() {
		m = m.UTC().Truncate(time.Second)

		if ims := r.Header.Get("If-Modified-Since"); ims != "" {
			if t, err := http.ParseTime(ims); err == nil && !m.After(t) {
				w.WriteHeader(http.StatusNotModified)

				return
			}
		}

		w.Header().Set("Last-Modified", m.UTC().Format(http.TimeFormat))
	}

	w.Header().Set("Content-Type", "text/html; charset=UTF-8")

	if r.Method == http.MethodHead {
		return
	}

	files, err := f.Readdir(-1)
	if err != nil {
		s.error(w, err)

		return
	}

	var buf bytes.Buffer

	if err := s.template.Execute(&buf, dir{
		Name:    path.Join(s.rootPath, r.URL.Path) + "/",
		Crumbs:  breadcrumbs(s.rootPath, r.URL.Path),
		Entries: entries(files),
	}); err != nil {
		s.error(w, err)

		return
	}

	buf.WriteTo(w) //nolint:errcheck
}

func (s *FileServer) error(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, fs.ErrNotExist):
		http.Error(w, "404 page not found", http.StatusNotFound)
	case errors.Is(err, fs.ErrPermission):
		http.Error(w, "403 forbidden", http.StatusForbidden)
	default:
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
	}
}

func entries(files []fs.FileInfo) []entry {
	entries := make([]entry, 0, len(files))

	for _, info := range files {
		entry := entry{
			URL:   encodePath(info.Name()),
			Name:  info.Name(),
			IsDir: info.IsDir(),
		}

		if info.IsDir() {
			entry.Name += "/"
		}

		entries = append(entries, entry)
	}

	slices.SortStableFunc(entries, func(a, b entry) int {
		switch {
		case a.IsDir && !b.IsDir:
			return -1
		case !a.IsDir && b.IsDir:
			return 1
		}

		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}

		return strings.Compare(a.Name, b.Name)
	})

	return entries
}

func breadcrumbs(root, target string) []crumb {
	crumbs := []crumb{{
		URL:  "/",
		Name: root,
	}}

	defer func() { crumbs[len(crumbs)-1].Disable = true }()

	if target == "/" {
		return crumbs
	}

	var (
		current = "/"
		parts   = strings.Split(strings.Trim(target, "/"), "/")
	)

	for _, part := range parts {
		if part == "" {
			continue
		}

		current = path.Join(current, part) + "/"

		crumbs = append(crumbs, crumb{
			URL:  encodePath(current),
			Name: part,
		})
	}

	return crumbs
}

func encodePath(target string) string {
	return (&url.URL{Path: target}).String()
}
