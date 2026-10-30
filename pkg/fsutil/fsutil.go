package fsutil

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var unsafeFilenameChars = regexp.MustCompile(`[\\/:*?"<>|\x00-\x1F]`)

// SanitizeFilename removes characters that are illegal on common filesystems.
func SanitizeFilename(name string) string {
	cleaned := unsafeFilenameChars.ReplaceAllString(name, "_")
	cleaned = strings.Trim(cleaned, " .")
	if cleaned == "" {
		cleaned = "unnamed"
	}
	return cleaned
}

// GetUniqueFilePath returns a path that does not collide with existing
// files or .part files in dir.
func GetUniqueFilePath(dir, filename string) string {
	ext := filepath.Ext(filename)
	baseName := strings.TrimSuffix(filename, ext)
	path := filepath.Join(dir, filename)
	counter := 1

	for {
		_, err1 := os.Stat(path)
		_, err2 := os.Stat(path + ".part")

		if os.IsNotExist(err1) && os.IsNotExist(err2) {
			break
		}
		if os.IsNotExist(err1) && !os.IsNotExist(err2) {
			break
		}

		newName := fmt.Sprintf("%s_%d%s", baseName, counter, ext)
		path = filepath.Join(dir, newName)
		counter++
	}
	return path
}