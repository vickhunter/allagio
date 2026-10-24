package orp

import (
	"bytes"
	"io/fs"
	"testing"
)

func TestPublicBundleMatchesRepo(t *testing.T) {
	root, ok := protocolRoot()
	if !ok {
		t.Fatal("docs/protocol not found")
	}
	repo := mappedDocs{root: root}
	bundled, err := fs.Sub(publicBundle, "bundled")
	if err != nil {
		t.Fatal(err)
	}
	repoFiles := map[string][]byte{}
	collect := func(fsys fs.FS, into map[string][]byte) {
		t.Helper()
		for _, name := range []string{"SPEC.md", "GAPS.md"} {
			body, err := fs.ReadFile(fsys, name)
			if err != nil {
				t.Fatalf("%s: %v", name, err)
			}
			into[name] = body
		}
		for _, dir := range []string{"schemas", "examples"} {
			err := fs.WalkDir(fsys, dir, func(entryPath string, entry fs.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := fs.ReadFile(fsys, entryPath)
				if err != nil {
					return err
				}
				into[entryPath] = body
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	collect(repo, repoFiles)
	bundleFiles := map[string][]byte{}
	collect(bundled, bundleFiles)
	if len(repoFiles) != len(bundleFiles) {
		t.Fatalf("bundle has %d files and the repo has %d", len(bundleFiles), len(repoFiles))
	}
	for name, want := range repoFiles {
		got, ok := bundleFiles[name]
		if !ok {
			t.Fatalf("bundle missing %s", name)
		}
		if !bytes.Equal(want, got) {
			t.Fatalf("bundle drifted from %s", name)
		}
		if bytes.Contains(want, []byte("offerrails.com")) || bytes.Contains(bytes.ToLower(want), []byte("open source")) {
			t.Fatalf("served doc %s points at the unpicked domain or says open source", name)
		}
	}
}
