package info

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// setupOutput prepares an output writer + optional file.
//   - saveInfo=false → (stdout, nil, "", nil)
//   - saveInfo=true, filename="" → (stdout, nil, "", nil) — caller must re-create
//   - saveInfo=true, filename!="" → (writer, file, path, nil)
func setupOutput(saveInfo, showInfo bool, outputDir, filename string) (io.Writer, *os.File, string, error) {
	if !saveInfo {
		return os.Stdout, nil, "", nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, nil, "", fmt.Errorf("cannot create output dir: %w", err)
	}
	if filename == "" {
		return os.Stdout, nil, "", nil
	}
	path := filepath.Join(outputDir, filename)
	f, err := os.Create(path)
	if err != nil {
		return nil, nil, "", fmt.Errorf("cannot create info file: %w", err)
	}
	if showInfo {
		return io.MultiWriter(os.Stdout, f), f, path, nil
	}
	return f, f, path, nil
}