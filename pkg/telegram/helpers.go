package telegram

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/tg"
	"tg-dlp/pkg/runtime"
)

var floodWaitRe = regexp.MustCompile(`FLOOD_WAIT_(\d+)`)

var skipFloodShown = false

// HandleFloodWait checks err for FLOOD_WAIT, sleeps (or skips),
// and returns true if the caller should retry.
func HandleFloodWait(err error) bool {
	if err == nil || !strings.Contains(err.Error(), "FLOOD_WAIT") {
		skipFloodShown = false
		return false
	}

	if runtime.SkipFlood {
		if !skipFloodShown {
			fmt.Println("\n⏩ Skipping FLOOD_WAIT as requested...")
			skipFloodShown = true
		}
		return true
	}

	match := floodWaitRe.FindStringSubmatch(err.Error())
	waitTime := 30 * time.Second
	if len(match) > 1 {
		if seconds, parseErr := strconv.Atoi(match[1]); parseErr == nil {
			waitTime = time.Duration(seconds) * time.Second
		}
	}
	fmt.Printf("\n⚠ Telegram requested a wait. Sleeping for %v...\n", waitTime)
	time.Sleep(waitTime)
	skipFloodShown = false
	return true
}

// GetFileExt derives a file extension from a document's attributes or MIME type.
func GetFileExt(doc *tg.Document) string {
	for _, attr := range doc.Attributes {
		if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
			ext := filepath.Ext(fn.FileName)
			if ext != "" {
				return strings.ToLower(ext)
			}
		}
	}

	mime := strings.ToLower(doc.MimeType)
	switch {
	case strings.HasPrefix(mime, "video/"):
		if strings.Contains(mime, "matroska") || strings.Contains(mime, "mkv") {
			return ".mkv"
		}
		return ".mp4"
	case strings.HasPrefix(mime, "audio/"):
		if strings.Contains(mime, "ogg") {
			return ".ogg"
		}
		return ".mp3"
	case strings.HasPrefix(mime, "image/"):
		if strings.Contains(mime, "png") {
			return ".png"
		}
		return ".jpeg"
	default:
		return ".bin"
	}
}

// PickLargestPhotoSize returns the largest PhotoSizeClass and its byte size.
func PickLargestPhotoSize(sizes []tg.PhotoSizeClass) (tg.PhotoSizeClass, int) {
	maxSize := -1
	var largest tg.PhotoSizeClass
	for _, s := range sizes {
		var sz int
		switch v := s.(type) {
		case *tg.PhotoSize:
			sz = v.Size
		case *tg.PhotoSizeProgressive:
			for _, p := range v.Sizes {
				if p > sz {
					sz = p
				}
			}
		case *tg.PhotoCachedSize:
			sz = len(v.Bytes)
		}
		if sz > maxSize {
			maxSize = sz
			largest = s
		}
	}
	return largest, maxSize
}

// ParseExtensions turns "jpg, png,.gif" into a set of lowercase extensions.
func ParseExtensions(s string) map[string]bool {
	result := make(map[string]bool)
	if s == "" {
		return result
	}
	for _, ext := range strings.Split(s, ",") {
		ext = strings.TrimSpace(ext)
		ext = strings.ToLower(ext)
		ext = strings.TrimPrefix(ext, ".")
		if ext != "" {
			result[ext] = true
		}
	}
	return result
}