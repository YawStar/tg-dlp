package downloader

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/tg"

	"tg-dlp/pkg/fsutil"
	"tg-dlp/pkg/progress"
	"tg-dlp/pkg/runtime"
	"tg-dlp/pkg/telegram"
)

// Options defines configuration parameters for the Downloader.
type Options struct {
	Concurrency int   // Concurrency (Workers)
	LimitRate   int64 // Bytes per second (0 = unlimited)
}

// Downloader wraps a gotd downloader for convenience.
type Downloader struct {
	Inner   *downloader.Downloader
	Options Options
}

// New creates a downloader with default options.
func New() *Downloader {
	return &Downloader{
		Inner: downloader.NewDownloader(),
		Options: Options{
			Concurrency: 3,
		},
	}
}

// NewWithConfig creates a downloader with custom configuration options.
func NewWithConfig(opts Options) *Downloader {
	if opts.Concurrency <= 0 {
		opts.Concurrency = 3
	}
	return &Downloader{
		Inner:   downloader.NewDownloader(),
		Options: opts,
	}
}

// Filename Template ကို ပြောင်းပေးတဲ့ Helper
func generateFilename(templateStr string, msgID int, ext string) string {
	if templateStr == "" {
		return fsutil.SanitizeFilename(fmt.Sprintf("media_%d%s", msgID, ext))
	}
	cleanExt := strings.TrimPrefix(ext, ".")
	name := strings.ReplaceAll(templateStr, "{id}", fmt.Sprintf("%d", msgID))
	name = strings.ReplaceAll(name, "{ext}", cleanExt)
	if !strings.HasSuffix(name, "."+cleanExt) {
		name = name + ext
	}
	return fsutil.SanitizeFilename(name)
}

// ---------- Output path helpers ----------
func resolveOutputPaths(outputDir, safeName string, overwrite, skipSame bool, expectedSize int64) (outputPath, partPath string, skip bool) {
	if overwrite {
		outputPath = filepath.Join(outputDir, safeName)
		partPath = outputPath + ".part"
		_ = os.Remove(outputPath)
		_ = os.Remove(partPath)
		return outputPath, partPath, false
	}
	if skipSame {
		outputPath = filepath.Join(outputDir, safeName)
		if info, err := os.Stat(outputPath); err == nil && info.Size() == expectedSize {
			return outputPath, outputPath + ".part", true
		}
		return outputPath, outputPath + ".part", false
	}
	outputPath = fsutil.GetUniqueFilePath(outputDir, safeName)
	return outputPath, outputPath + ".part", false
}

func openPartFile(partPath string, verbose bool) (*os.File, int64, error) {
	if info, err := os.Stat(partPath); err == nil {
		file, err := os.OpenFile(partPath, os.O_RDWR, 0644)
		if err != nil {
			return nil, 0, fmt.Errorf("cannot open part file: %w", err)
		}
		size := info.Size()
		if verbose {
			runtime.DebugLog("🔄 Resuming file: %s from %.2f MB", filepath.Base(partPath), float64(size)/(1024*1024))
		}
		return file, size, nil
	}
	file, err := os.Create(partPath)
	if err != nil {
		return nil, 0, fmt.Errorf("error creating file: %w", err)
	}
	return file, 0, nil
}

func BestThreads(size int64) int {
	if size < 1024*1024 {
		return 1
	}
	if size < 5*1024*1024 {
		return 4
	}
	if size < 20*1024*1024 {
		return 8
	}
	return 16
}

func streamWithRetry(ctx context.Context, api *tg.Client, dl *downloader.Downloader, loc tg.InputFileLocationClass, w *progress.Writer, offset int64, expectedSize int64) error {
	backoff := 2 * time.Second
	attempt := 1

	threads := BestThreads(expectedSize)

	for {
		if expectedSize > 0 {
			if info, err := w.File().Stat(); err == nil {
				if info.Size() >= expectedSize {
					return nil
				}
			}
		}

		_, err := dl.Download(api, loc).WithThreads(threads).Parallel(ctx, w)
		if err == nil {
			return nil
		}

		if telegram.HandleFloodWait(err) {
			continue
		}

		errStr := strings.ToLower(err.Error())
		isNetworkError := strings.Contains(errStr, "timeout") ||
			strings.Contains(errStr, "deadline exceeded") ||
			strings.Contains(errStr, "-503") ||
			strings.Contains(errStr, "-500") ||
			strings.Contains(errStr, "no workers running") ||
			strings.Contains(errStr, "connection reset") ||
			strings.Contains(errStr, "eof") ||
			strings.Contains(errStr, "broken pipe") ||
			strings.Contains(errStr, "connection refused") ||
			strings.Contains(errStr, "no route to host") ||
			strings.Contains(errStr, "i/o timeout") ||
			strings.Contains(errStr, "transport is closing")

		if isNetworkError {
			if expectedSize > 0 {
				if info, err := w.File().Stat(); err == nil {
					if info.Size() >= expectedSize {
						return nil
					}
				}
			}

			fmt.Printf("\n⚠ Network/Worker error at %s/%s. Retrying (attempt %d) in %v...\n",
				progress.FormatSize(w.Current()),
				progress.FormatSize(w.Total()),
				attempt, backoff)

			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(backoff):
			}
			if backoff < 30*time.Second {
				backoff *= 2
			}
			fmt.Print("\033[2K\r")
			attempt++
			continue
		}
		return err
	}
}

func extMatches(filterExts []string, ext string) bool {
	if len(filterExts) == 0 {
		return true
	}
	cleanExt := strings.TrimPrefix(ext, ".")
	for _, f := range filterExts {
		if strings.EqualFold(cleanExt, strings.TrimSpace(f)) {
			return true
		}
	}
	return false
}

func photoMatches(filterExts []string) bool {
	if len(filterExts) == 0 {
		return true
	}
	for _, f := range filterExts {
		f = strings.TrimSpace(f)
		if strings.EqualFold("jpg", f) || strings.EqualFold("jpeg", f) || strings.EqualFold("png", f) {
			return true
		}
	}
	return false
}

// ---------- SingleMedia ----------
func (d *Downloader) SingleMedia(ctx context.Context, api *tg.Client, msg *tg.Message, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) (bool, error) {
	if msg.Media == nil {
		return false, nil
	}
	switch media := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		return d.downloadDocument(ctx, api, media, msgID, outputDir, filterExts, skipSame, overwrite, templateStr, true)
	case *tg.MessageMediaPhoto:
		return d.downloadPhoto(ctx, api, media, msgID, outputDir, filterExts, skipSame, overwrite, templateStr, true)
	}
	return false, nil
}

func (d *Downloader) SingleMediaQuiet(ctx context.Context, api *tg.Client, msg *tg.Message, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) (bool, string, error) {
	if msg.Media == nil {
		return false, "no media", nil
	}
	switch media := msg.Media.(type) {
	case *tg.MessageMediaDocument:
		return d.downloadDocumentQuiet(ctx, api, media, msgID, outputDir, filterExts, skipSame, overwrite, templateStr)
	case *tg.MessageMediaPhoto:
		return d.downloadPhotoQuiet(ctx, api, media, msgID, outputDir, filterExts, skipSame, overwrite, templateStr)
	}
	return false, "unsupported media type", nil
}

// ---------- Document download ----------
func (d *Downloader) downloadDocument(ctx context.Context, api *tg.Client, media *tg.MessageMediaDocument, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string, verbose bool) (bool, error) {
	doc, ok := media.Document.AsNotEmpty()
	if !ok {
		return false, nil
	}
	ext := telegram.GetFileExt(doc)
	if !extMatches(filterExts, ext) {
		return false, nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return false, err
	}
	safeName := generateFilename(templateStr, msgID, ext)
	outputPath, partPath, skip := resolveOutputPaths(outputDir, safeName, overwrite, skipSame, doc.Size)
	if skip {
		return false, nil
	}
	file, offset, err := openPartFile(partPath, verbose)
	if err != nil {
		return false, err
	}
	defer file.Close()

	startTime := time.Now()
	loc := &tg.InputDocumentFileLocation{ID: doc.ID, AccessHash: doc.AccessHash, FileReference: doc.FileReference}
	pw := progress.New(file, doc.Size, offset)
	if err := streamWithRetry(ctx, api, d.Inner, loc, pw, offset, doc.Size); err != nil {
		return false, fmt.Errorf("download error: %w", err)
	}
	fmt.Println()

	file.Close()
	if err := os.Rename(partPath, outputPath); err != nil {
		return false, fmt.Errorf("error renaming file: %w", err)
	}
	if verbose {
		fmt.Printf("✅ Downloaded: %s (in %s)\n", filepath.Base(outputPath), time.Since(startTime).Round(time.Millisecond))
	}
	return true, nil
}

func (d *Downloader) downloadDocumentQuiet(ctx context.Context, api *tg.Client, media *tg.MessageMediaDocument, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) (bool, string, error) {
	doc, ok := media.Document.AsNotEmpty()
	if !ok {
		return false, "empty document", nil
	}
	ext := telegram.GetFileExt(doc)
	if !extMatches(filterExts, ext) {
		return false, fmt.Sprintf("file type %s not in filter", ext), nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return false, "", err
	}
	safeName := generateFilename(templateStr, msgID, ext)
	outputPath, partPath, skip := resolveOutputPaths(outputDir, safeName, overwrite, skipSame, doc.Size)
	if skip {
		return false, "same file & size", nil
	}
	file, offset, err := openPartFile(partPath, false)
	if err != nil {
		return false, "", err
	}
	defer file.Close()

	loc := &tg.InputDocumentFileLocation{ID: doc.ID, AccessHash: doc.AccessHash, FileReference: doc.FileReference}
	pw := progress.New(file, doc.Size, offset)
	if err := streamWithRetry(ctx, api, d.Inner, loc, pw, offset, doc.Size); err != nil {
		return false, "", fmt.Errorf("download error: %w", err)
	}
	file.Close()
	if err := os.Rename(partPath, outputPath); err != nil {
		return false, "", fmt.Errorf("error renaming file: %w", err)
	}
	return true, "", nil
}

// ---------- Photo download ----------
func (d *Downloader) downloadPhoto(ctx context.Context, api *tg.Client, media *tg.MessageMediaPhoto, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string, verbose bool) (bool, error) {
	photo, ok := media.Photo.AsNotEmpty()
	if !ok {
		return false, fmt.Errorf("cannot get photo")
	}
	if !photoMatches(filterExts) {
		return false, nil
	}
	largest, size := telegram.PickLargestPhotoSize(photo.Sizes)
	if largest == nil {
		return false, fmt.Errorf("photo size not found")
	}
	var loc tg.InputFileLocationClass
	switch v := largest.(type) {
	case *tg.PhotoSize:
		loc = &tg.InputPhotoFileLocation{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference, ThumbSize: v.Type}
	case *tg.PhotoSizeProgressive:
		loc = &tg.InputPhotoFileLocation{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference, ThumbSize: v.Type}
	default:
		return false, fmt.Errorf("unsupported photo type")
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return false, err
	}
	safeName := generateFilename(templateStr, msgID, ".jpg")
	outputPath, partPath, skip := resolveOutputPaths(outputDir, safeName, overwrite, skipSame, int64(size))
	if skip {
		return false, nil
	}
	file, offset, err := openPartFile(partPath, verbose)
	if err != nil {
		return false, err
	}
	defer file.Close()

	startTime := time.Now()
	pw := progress.New(file, int64(size), offset)
	if err := streamWithRetry(ctx, api, d.Inner, loc, pw, offset, int64(size)); err != nil {
		return false, fmt.Errorf("download error: %w", err)
	}
	fmt.Println()

	file.Close()
	if err := os.Rename(partPath, outputPath); err != nil {
		return false, fmt.Errorf("error renaming file: %w", err)
	}
	if verbose {
		fmt.Printf("✅ Downloaded: %s (in %s)\n", filepath.Base(outputPath), time.Since(startTime).Round(time.Millisecond))
	}
	return true, nil
}

func (d *Downloader) downloadPhotoQuiet(ctx context.Context, api *tg.Client, media *tg.MessageMediaPhoto, msgID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) (bool, string, error) {
	photo, ok := media.Photo.AsNotEmpty()
	if !ok {
		return false, "empty photo", nil
	}
	if !photoMatches(filterExts) {
		return false, "photo filter mismatch", nil
	}
	largest, size := telegram.PickLargestPhotoSize(photo.Sizes)
	if largest == nil {
		return false, "photo size not found", nil
	}
	var loc tg.InputFileLocationClass
	switch v := largest.(type) {
	case *tg.PhotoSize:
		loc = &tg.InputPhotoFileLocation{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference, ThumbSize: v.Type}
	case *tg.PhotoSizeProgressive:
		loc = &tg.InputPhotoFileLocation{ID: photo.ID, AccessHash: photo.AccessHash, FileReference: photo.FileReference, ThumbSize: v.Type}
	default:
		return false, "unsupported photo type", nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return false, "", err
	}
	safeName := generateFilename(templateStr, msgID, ".jpg")
	outputPath, partPath, skip := resolveOutputPaths(outputDir, safeName, overwrite, skipSame, int64(size))
	if skip {
		return false, "same photo & size", nil
	}
	file, offset, err := openPartFile(partPath, false)
	if err != nil {
		return false, "", err
	}
	defer file.Close()

	pw := progress.New(file, int64(size), offset)
	if err := streamWithRetry(ctx, api, d.Inner, loc, pw, offset, int64(size)); err != nil {
		return false, "", fmt.Errorf("download error: %w", err)
	}
	file.Close()
	if err := os.Rename(partPath, outputPath); err != nil {
		return false, "", fmt.Errorf("error renaming file: %w", err)
	}
	return true, "", nil
}

// ---------- Bulk download ----------
const CheckpointFileName = ".tg-dlp_checkpoint"

func (d *Downloader) ChannelMedia(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) error {
	fmt.Println("📥 Searching for media in channel...")
	peer := &tg.InputPeerChannel{ChannelID: targetChannel.ChannelID, AccessHash: targetChannel.AccessHash}

	filters := []tg.MessagesFilterClass{
		&tg.InputMessagesFilterPhotos{},
		&tg.InputMessagesFilterVideo{},
		&tg.InputMessagesFilterDocument{},
		&tg.InputMessagesFilterURL{},
	}
	totalMedia := 0
	if counters, err := api.MessagesGetSearchCounters(ctx, &tg.MessagesGetSearchCountersRequest{Peer: peer, Filters: filters}); err == nil {
		for _, c := range counters {
			totalMedia += c.Count
		}
		fmt.Printf("📊 Total media found: %d\n", totalMedia)
	}

	offsetID := 0
	totalScanned := 0
	checkpointFile := filepath.Join(outputDir, fmt.Sprintf("%s_%d", CheckpointFileName, targetChannel.ChannelID))
	if data, err := os.ReadFile(checkpointFile); err == nil {
		parts := strings.Split(strings.TrimSpace(string(data)), ",")
		if len(parts) == 2 {
			if savedID, err := parseAtoi(parts[0]); err == nil {
				offsetID = savedID
			}
			if savedScanned, err := parseAtoi(parts[1]); err == nil {
				totalScanned = savedScanned
			}
			fmt.Printf("🔄 Found checkpoint. Resuming from Message ID: %d (Scanned: %d/%d)\n", offsetID, totalScanned, totalMedia)
		}
	}

	limit := 100
	totalDownloaded, totalFailed := 0, 0

	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // ⭐ Bulk ဖြစ်လို့ File ၂ ခုပဲ တစ်ပြိုင်နက်ဆွဲမယ် ⭐
	var mu sync.Mutex

	for {
		messages, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, OffsetID: offsetID, Limit: limit})
		if err != nil {
			if telegram.HandleFloodWait(err) {
				continue
			}
			return fmt.Errorf("error fetching messages: %w", err)
		}
		var msgs []*tg.Message
		if m, ok := messages.(*tg.MessagesChannelMessages); ok {
			for _, msgClass := range m.Messages {
				if msg, ok := msgClass.(*tg.Message); ok && msg.Media != nil {
					msgs = append(msgs, msg)
				}
			}
		}
		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			totalScanned++
			var percent float64
			if totalMedia > 0 {
				percent = float64(totalScanned) / float64(totalMedia) * 100
				if percent > 100 {
					percent = 100
				}
			}
			fmt.Printf("\r[Progress: %d/%d | %.2f%%] ", totalScanned, totalMedia, percent)

			wg.Add(1)
			sem <- struct{}{}
			go func(msg *tg.Message) {
				defer wg.Done()
				defer func() { <-sem }()

				downloaded, skipReason, err := d.SingleMediaQuiet(ctx, api, msg, msg.ID, outputDir, filterExts, skipSame, overwrite, templateStr)
				if err != nil {
					fmt.Printf("\n⚠ Error: %v\n", err)
					mu.Lock()
					totalFailed++
					mu.Unlock()
				} else if downloaded {
					mu.Lock()
					totalDownloaded++
					mu.Unlock()
					fmt.Printf("\n✅ Downloaded: media_%d\n", msg.ID)
				} else {
					fmt.Printf("\n⏭ Skipped (media_%d): %s\n", msg.ID, skipReason)
				}
			}(msg)
		}

		lastMsgID := msgs[len(msgs)-1].ID
		err = os.WriteFile(checkpointFile, []byte(fmt.Sprintf("%d,%d", lastMsgID, totalScanned)), 0644)
		if err != nil {
			runtime.DebugLog("⚠ Failed to save checkpoint: %v", err)
		}
		offsetID = lastMsgID
		time.Sleep(300 * time.Millisecond)
	}

	wg.Wait()
	os.Remove(checkpointFile)

	fmt.Printf("\n🎉 Download finished!\n")
	fmt.Printf("📊 Total: %d\n", totalDownloaded+totalFailed)
	fmt.Printf("✅ Successful: %d\n", totalDownloaded)
	fmt.Printf("❌ Failed: %d\n", totalFailed)
	return nil
}

func (d *Downloader) ThreadMedia(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, threadID int, outputDir string, filterExts []string, skipSame, overwrite bool, templateStr string) error {
	fmt.Printf("💬 Fetching replies from thread (MsgID: %d)...\n", threadID)
	peer := &tg.InputPeerChannel{ChannelID: targetChannel.ChannelID, AccessHash: targetChannel.AccessHash}

	offsetID := 0
	limit := 100
	totalDownloaded, totalFailed, totalScanned := 0, 0, 0
	var wg sync.WaitGroup
	sem := make(chan struct{}, 10) // ⭐ Bulk ဖြစ်လို့ File ၂ ခုပဲ တစ်ပြိုင်နက်ဆွဲမယ် ⭐
	var mu sync.Mutex

	for {
		replies, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{Peer: peer, MsgID: threadID, OffsetID: offsetID, Limit: limit})
		if err != nil {
			if telegram.HandleFloodWait(err) {
				continue
			}
			return fmt.Errorf("error fetching replies: %w", err)
		}
		var msgs []*tg.Message
		var totalReplies int
		if m, ok := replies.(*tg.MessagesChannelMessages); ok {
			totalReplies = m.Count
			for _, msgClass := range m.Messages {
				if msg, ok := msgClass.(*tg.Message); ok && msg.Media != nil {
					msgs = append(msgs, msg)
				}
			}
		}
		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			totalScanned++
			var percent float64
			if totalReplies > 0 {
				percent = float64(totalScanned) / float64(totalReplies) * 100
				if percent > 100 {
					percent = 100
				}
			}
			fmt.Printf("\r[Thread Progress: %d/%d | %.2f%%] ", totalScanned, totalReplies, percent)

			wg.Add(1)
			sem <- struct{}{}
			go func(msg *tg.Message) {
				defer wg.Done()
				defer func() { <-sem }()

				downloaded, skipReason, err := d.SingleMediaQuiet(ctx, api, msg, msg.ID, outputDir, filterExts, skipSame, overwrite, templateStr)
				if err != nil {
					fmt.Printf("\n⚠ Error: %v\n", err)
					mu.Lock()
					totalFailed++
					mu.Unlock()
				} else if downloaded {
					mu.Lock()
					totalDownloaded++
					mu.Unlock()
					fmt.Printf("\n✅ Downloaded: media_%d\n", msg.ID)
				} else {
					fmt.Printf("\n⏭ Skipped (media_%d): %s\n", msg.ID, skipReason)
				}
			}(msg)
		}

		offsetID = msgs[len(msgs)-1].ID
		time.Sleep(300 * time.Millisecond)
	}

	wg.Wait()
	fmt.Printf("\n🎉 Thread Download finished!\n")
	fmt.Printf("📊 Total Media Found: %d\n", totalDownloaded+totalFailed)
	fmt.Printf("✅ Successful: %d\n", totalDownloaded)
	fmt.Printf("❌ Failed: %d\n", totalFailed)
	return nil
}

func parseAtoi(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("not a number")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}
