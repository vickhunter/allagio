package orp

import (
	"encoding/json"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// SchemaIDHost is the only host used in ORP schema $id values.
// It is a literal, not the public site default. ORP 0.3 files keep this
// host byte for byte. Change this one string the day a domain is bought,
// then update every $id together so they stay on one host.
const SchemaIDHost = "https://offer-rails.onrender.com"

// PublicHost is that same $id host. Served document links use
// config.ExpandPublicBase and do not copy this constant.
const PublicHost = SchemaIDHost

// ReadPublic returns one served document. name is SPEC.md, GAPS.md,
// schemas/<file>, or examples/<file>. A missing file returns false.
func ReadPublic(name string) ([]byte, bool) {
	name, ok := cleanPublic(name)
	if !ok {
		return nil, false
	}
	fsys, err := openPublic()
	if err != nil {
		return nil, false
	}
	body, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, false
	}
	return body, true
}

// ListPublic returns the files under schemas or examples, sorted.
func ListPublic(dir string) ([]string, bool) {
	if dir != "schemas" && dir != "examples" {
		return nil, false
	}
	fsys, err := openPublic()
	if err != nil {
		return nil, false
	}
	var out []string
	err = fs.WalkDir(fsys, dir, func(entryPath string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel := strings.TrimPrefix(entryPath, dir+"/")
		if rel == "" || strings.Contains(rel, "..") {
			return nil
		}
		out = append(out, rel)
		return nil
	})
	if err != nil {
		return nil, false
	}
	sort.Strings(out)
	return out, true
}

// PublicIndex is the JSON directory listing for schemas or examples.
func PublicIndex(dir string) ([]byte, bool) {
	files, ok := ListPublic(dir)
	if !ok {
		return nil, false
	}
	body, err := json.Marshal(map[string]any{"files": files})
	if err != nil {
		return nil, false
	}
	return body, true
}

func cleanPublic(name string) (string, bool) {
	name = strings.TrimPrefix(name, "/")
	if name == "" || strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return "", false
	}
	name = path.Clean(name)
	if name == "." || strings.HasPrefix(name, "../") || strings.Contains(name, "..") {
		return "", false
	}
	switch {
	case name == "SPEC.md" || name == "GAPS.md":
		return name, true
	case name == "schemas" || strings.HasPrefix(name, "schemas/"):
		return name, true
	case name == "examples" || strings.HasPrefix(name, "examples/"):
		return name, true
	default:
		return "", false
	}
}

func openPublic() (fs.FS, error) {
	if root, ok := protocolRoot(); ok {
		return mappedDocs{root: root}, nil
	}
	return fs.Sub(publicBundle, "bundled")
}

func protocolRoot() (string, bool) {
	dir, err := os.Getwd()
	if err != nil {
		return "", false
	}
	for {
		candidate := filepath.Join(dir, "docs", "protocol", "SPEC.md")
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return filepath.Join(dir, "docs", "protocol"), true
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", false
		}
		dir = parent
	}
}

// mappedDocs reads the repo layout through the served names.
type mappedDocs struct {
	root string
}

func (m mappedDocs) Open(name string) (fs.File, error) {
	full, err := m.path(name)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(full)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fs.ErrNotExist
		}
		return nil, err
	}
	return file, nil
}

func (m mappedDocs) path(name string) (string, error) {
	name, ok := cleanPublic(name)
	if !ok {
		return "", fs.ErrInvalid
	}
	switch {
	case name == "SPEC.md" || name == "GAPS.md":
		return filepath.Join(m.root, filepath.FromSlash(name)), nil
	case name == "schemas" || strings.HasPrefix(name, "schemas/"):
		rest := strings.TrimPrefix(strings.TrimPrefix(name, "schemas"), "/")
		if rest == "0.4" || strings.HasPrefix(rest, "0.4/") {
			inner := strings.TrimPrefix(strings.TrimPrefix(rest, "0.4"), "/")
			return filepath.Join(m.root, "schema", "0.4", filepath.FromSlash(inner)), nil
		}
		return filepath.Join(m.root, "schema", "0.3", filepath.FromSlash(rest)), nil
	case name == "examples" || strings.HasPrefix(name, "examples/"):
		rest := strings.TrimPrefix(strings.TrimPrefix(name, "examples"), "/")
		return filepath.Join(m.root, "schema", "examples", filepath.FromSlash(rest)), nil
	default:
		return "", fs.ErrNotExist
	}
}

func (m mappedDocs) ReadDir(name string) ([]fs.DirEntry, error) {
	name, ok := cleanPublic(name)
	if !ok {
		return nil, fs.ErrInvalid
	}
	if name == "schemas" {
		entries, err := os.ReadDir(filepath.Join(m.root, "schema", "0.3"))
		if err != nil {
			return nil, err
		}
		out := make([]fs.DirEntry, 0, len(entries)+1)
		seen := false
		for _, entry := range entries {
			if entry.Name() == "0.4" {
				seen = true
			}
			out = append(out, entry)
		}
		if !seen {
			out = append(out, staticDir{name: "0.4"})
		}
		sort.Slice(out, func(i, j int) bool { return out[i].Name() < out[j].Name() })
		return out, nil
	}
	full, err := m.path(name)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(full)
}

type staticDir struct{ name string }

func (d staticDir) Name() string      { return d.name }
func (d staticDir) IsDir() bool       { return true }
func (d staticDir) Type() fs.FileMode { return fs.ModeDir }
func (d staticDir) Info() (fs.FileInfo, error) {
	return staticInfo{name: d.name}, nil
}

type staticInfo struct{ name string }

func (s staticInfo) Name() string       { return s.name }
func (s staticInfo) Size() int64        { return 0 }
func (s staticInfo) Mode() fs.FileMode  { return fs.ModeDir }
func (s staticInfo) ModTime() time.Time { return time.Time{} }
func (s staticInfo) IsDir() bool        { return true }
func (s staticInfo) Sys() any           { return nil }
