package fsutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSanitizeFilename(t *testing.T) {
	cases := map[string]string{
		"hello/world": "hello_world",
		"a:b*c?":      "a_b_c_",
		"..":          "unnamed",
		"  .txt  ":    "txt",
	}
	for in, want := range cases {
		if got := SanitizeFilename(in); got != want {
			t.Errorf("SanitizeFilename(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestGetUniqueFilePath(t *testing.T) {
	dir := t.TempDir()

	// no collision
	p1 := GetUniqueFilePath(dir, "file.txt")
	if filepath.Base(p1) != "file.txt" {
		t.Errorf("got %q, want file.txt", filepath.Base(p1))
	}

	// create the file, then check collision
	if err := os.WriteFile(p1, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	p2 := GetUniqueFilePath(dir, "file.txt")
	if filepath.Base(p2) != "file_1.txt" {
		t.Errorf("got %q, want file_1.txt", filepath.Base(p2))
	}
}