package backup

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// ZipDirectory compresses an entire directory into a single zip file.
func ZipDirectory(sourceDir, targetZip string) error {
	archive, err := os.Create(targetZip)
	if err != nil {
		return fmt.Errorf("cannot create zip file: %w", err)
	}
	defer archive.Close()

	zipWriter := zip.NewWriter(archive)
	defer zipWriter.Close()

	baseDir := filepath.Base(sourceDir)

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		zipPath := filepath.Join(baseDir, relPath)
		zipFile, err := zipWriter.Create(zipPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		if err != nil {
			return err
		}

		fmt.Printf("\r📦 Compressing: %s", relPath)
		return nil
	})
}