// SPDX-License-Identifier: BUSL-1.1

package survey_test

// corpus_confinement_test.go proves the mechanism corpus (surveyresult_corpus_test.go)
// gives ampCorpus, svdCorpus and every corpus test built on them: os.OpenRoot
// confines every read to the operator-named directory, and fs.WalkDir/ReadFile
// only ever hand back names inside it. It needs no TRELLIS_AMP_CORPUS or
// TRELLIS_SVD_CORPUS corpus of its own and always runs.

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestCorpusWalksOnlyMatchingRootRelativeNames(t *testing.T) {
	dir := t.TempDir()
	write := func(rel string) {
		t.Helper()
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o700); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte("ok"), 0o600); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
	write("a.amp")
	write("sub/b.AMP") // case-insensitive extension match, and a level deep
	write("c.txt")     // wrong extension: must not come back

	t.Setenv("TRELLIS_AMP_CORPUS", dir)
	root, files := ampCorpus(t)

	want := map[string]bool{"a.amp": true, filepath.ToSlash("sub/b.AMP"): true}
	if len(files) != len(want) {
		t.Fatalf("corpus returned %v, want exactly %v", files, want)
	}
	for _, name := range files {
		if !want[name] {
			t.Errorf("corpus returned %q, which is not one of the .amp fixtures", name)
		}
		if data, err := fs.ReadFile(root, name); err != nil || string(data) != "ok" {
			t.Errorf("fs.ReadFile(root, %q) = %q, %v, want \"ok\", nil", name, data, err)
		}
	}
}

func TestCorpusRootRejectsEscapingNames(t *testing.T) {
	corpusDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(corpusDir, "inside.amp"), []byte("ok"), 0o600); err != nil {
		t.Fatalf("write fixture inside the corpus dir: %v", err)
	}

	outsideDir := t.TempDir()
	secretPath := filepath.Join(outsideDir, "secret-outside-corpus.txt")
	if err := os.WriteFile(secretPath, []byte("do not read me"), 0o600); err != nil {
		t.Fatalf("write fixture outside the corpus dir: %v", err)
	}

	r, err := os.OpenRoot(corpusDir)
	if err != nil {
		t.Fatalf("OpenRoot(%s): %v", corpusDir, err)
	}
	defer func() { _ = r.Close() }()
	root := r.FS()

	if data, err := fs.ReadFile(root, "inside.amp"); err != nil || string(data) != "ok" {
		t.Fatalf("expected the in-corpus file to read cleanly, got data=%q err=%v", data, err)
	}

	// A name built the way a hostile "corpus" entry could try to escape:
	// climbing out of the root with "..". Two independent guards refuse it:
	// fs.ReadFile validates the name against fs.ValidPath before it reaches
	// the OS, and even given a name that passed that check, os.Root's own
	// Open refuses anything resolving outside the directory it was opened on.
	escaping := filepath.ToSlash(filepath.Join("..", filepath.Base(outsideDir), filepath.Base(secretPath)))
	if _, err := fs.ReadFile(root, escaping); err == nil {
		t.Fatalf("fs.ReadFile(root, %q) followed a \"..\" name out of the corpus root — confinement broken", escaping)
	} else if !errors.Is(err, fs.ErrInvalid) {
		t.Errorf("fs.ReadFile(root, %q) failed with %v, want an fs.ErrInvalid-class rejection", escaping, err)
	}

	if _, err := r.Open(escaping); err == nil {
		t.Fatalf("os.Root.Open(%q) followed a \"..\" name out of the corpus root — confinement broken", escaping)
	}
}
