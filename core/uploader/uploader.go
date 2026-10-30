package uploader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
)

var imageExts = map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}

// SingleFile uploads one file to peer.
// Returns nil if the file is filtered out.
func SingleFile(
	ctx context.Context,
	sender *message.Sender,
	up *uploader.Uploader,
	peer tg.InputPeerClass,
	filePath string,
	asPhoto, removeAfter bool,
	includeExts, excludeExts map[string]bool,
) error {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))

	if len(includeExts) > 0 && !includeExts[ext] {
		fmt.Printf("⏭ Skipped (not in include list): %s\n", filepath.Base(filePath))
		return nil
	}
	if len(excludeExts) > 0 && excludeExts[ext] {
		fmt.Printf("⏭ Skipped (excluded): %s\n", filepath.Base(filePath))
		return nil
	}

	fmt.Printf("📤 Uploading: %s\n", filepath.Base(filePath))
	builder := sender.To(peer)

	if asPhoto || imageExts["."+ext] {
		_, err := builder.Upload(message.Upload(func(ctx context.Context, b message.Uploader) (tg.InputFileClass, error) {
			return b.FromPath(ctx, filePath)
		})).Photo(ctx)
		if err != nil {
			return fmt.Errorf("upload photo failed: %w", err)
		}
	} else {
		file, err := up.FromPath(ctx, filePath)
		if err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
		if _, err := builder.Media(ctx, message.UploadedDocument(file)); err != nil {
			return fmt.Errorf("send failed: %w", err)
		}
	}

	fmt.Printf("✅ Uploaded: %s\n", filepath.Base(filePath))

	if removeAfter {
		if err := os.Remove(filePath); err != nil {
			fmt.Printf("⚠ Could not remove local file: %v\n", err)
		} else {
			fmt.Printf("🗑️ Removed local file: %s\n", filepath.Base(filePath))
		}
	}
	return nil
}