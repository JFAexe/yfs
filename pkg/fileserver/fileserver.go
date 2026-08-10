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

const viewerTemplate = `<!DOCTYPE html>
<html>
<head>
  <meta charset="utf-8">
  <title>~{{ .Name }}</title>
  <style>
    :root { font-family: monospace; font-size: 62.5%; background: #181818; color: #e8e8e8 }
    body { font-size: 1.6rem; margin: 4rem; }
    span { margin: 0 0.6rem }
    table { border-collapse: collapse; width: 100%; }
    th { font-size: 1.8em; word-break: break-all; text-align: left; padding: 0 0 1.6rem; }
    td { font-size: 1.4em; white-space: nowrap; padding: 0.35rem 0; }
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

type entry struct {
	URL   string
	Name  string
	IsDir bool
}

type crumb struct {
	URL     string
	Name    string
	Disable bool
}

type dir struct {
	Name    string
	Crumbs  []crumb
	Entries []entry
}

type FileServer struct {
	root http.FileSystem
	tmpl *template.Template
}

func New(root fs.FS) *FileServer {
	return &FileServer{
		root: http.FS(root),
		tmpl: template.Must(template.New("viewer").Parse(viewerTemplate)),
	}
}

func (s *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)

		return
	}

	target := path.Clean("/" + r.URL.Path)

	f, err := s.root.Open(target)
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

		http.Redirect(w, r, u.String(), http.StatusMovedPermanently)

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

	entries := make([]entry, 0, len(files))

	for _, e := range files {
		entry := entry{
			URL:  (&url.URL{Path: e.Name()}).String(),
			Name: e.Name(),
		}

		if e.IsDir() {
			entry.IsDir = true
			entry.URL += "/"
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

	crumbs := []crumb{{
		URL:  "/",
		Name: "~",
	}}

	if target != "/" {
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
				URL:  (&url.URL{Path: current}).String(),
				Name: part,
			})
		}

		target += "/"
	}

	crumbs[len(crumbs)-1].Disable = true

	var buf bytes.Buffer

	if err := s.tmpl.Execute(&buf, dir{
		Name:    target,
		Crumbs:  crumbs,
		Entries: entries,
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
