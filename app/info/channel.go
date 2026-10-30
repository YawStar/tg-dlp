package info

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gotd/td/tg"

	"tg-dlp/pkg/fsutil"
	tgtg "tg-dlp/pkg/telegram"
)

// ShowChannelInfo prints (and optionally saves) channel info.
func ShowChannelInfo(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, showInfo, saveInfo bool, outputDir string) error {
	// 1. Fetch channel first (need title for filename).
	ch, err := fetchChannel(ctx, api, targetChannel)
	if err != nil {
		return err
	}

	// 2. Prepare output — single assignment, no double.
	var out io.Writer = os.Stdout
	var file *os.File
	var fileName string
	if saveInfo {
		safeTitle := fsutil.SanitizeFilename(ch.Title)
		fileName = filepath.Join(outputDir, fmt.Sprintf("%s_info.txt", safeTitle))
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return fmt.Errorf("cannot create output dir: %w", err)
		}
		file, err = os.Create(fileName)
		if err != nil {
			return fmt.Errorf("cannot create info file: %w", err)
		}
		defer file.Close()
		if showInfo {
			out = io.MultiWriter(os.Stdout, file)
		} else {
			out = file
		}
	}

	// 3. Full channel info.
	fullRes, err := api.ChannelsGetFullChannel(ctx, targetChannel)
	if err != nil {
		return fmt.Errorf("error fetching full channel info: %w", err)
	}
	chFull, ok := fullRes.FullChat.(*tg.ChannelFull)
	if !ok {
		return fmt.Errorf("unsupported chat type")
	}

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "📌 Channel Name: %s\n", ch.Title)
	if ch.Username != "" {
		fmt.Fprintf(out, "🔗 Username: @%s\n", ch.Username)
	}
	if chFull.About != "" {
		fmt.Fprintf(out, "📝 About: %s\n", chFull.About)
	}
	fmt.Fprintf(out, "👥 Subscribers: %d\n", chFull.ParticipantsCount)
	fmt.Fprintf(out, "🆔 Channel ID: %d\n", ch.ID)
	fmt.Fprintln(out, "==================================================")

	// 4. Counters.
	fmt.Fprintln(out, "🔍 Fetching media counters from server...")
	peer := &tg.InputPeerChannel{ChannelID: targetChannel.ChannelID, AccessHash: targetChannel.AccessHash}
	filters := []tg.MessagesFilterClass{
		&tg.InputMessagesFilterPhotos{},
		&tg.InputMessagesFilterVideo{},
		&tg.InputMessagesFilterDocument{},
		&tg.InputMessagesFilterURL{},
		&tg.InputMessagesFilterMusic{},
		&tg.InputMessagesFilterVoice{},
		&tg.InputMessagesFilterGif{},
	}
	counters, err := api.MessagesGetSearchCounters(ctx, &tg.MessagesGetSearchCountersRequest{Peer: peer, Filters: filters})
	if err != nil {
		fmt.Fprintf(out, "⚠ Could not fetch counters from server (%v). Falling back to manual counting...\n", err)
		manualChannelCount(ctx, api, peer, out, showInfo)
	} else {
		fmt.Fprintln(out, "==================================================")
		fmt.Fprintln(out, "📊 Media Summary (from Server)")
		fmt.Fprintln(out, "==================================================")
		for _, c := range counters {
			switch c.Filter.(type) {
			case *tg.InputMessagesFilterPhotos:
				fmt.Fprintf(out, "🖼️ Photos: %d\n", c.Count)
			case *tg.InputMessagesFilterVideo:
				fmt.Fprintf(out, "🎬 Videos: %d\n", c.Count)
			case *tg.InputMessagesFilterDocument:
				fmt.Fprintf(out, "📁 Files: %d\n", c.Count)
			case *tg.InputMessagesFilterURL:
				fmt.Fprintf(out, "🔗 Links: %d\n", c.Count)
			case *tg.InputMessagesFilterMusic:
				fmt.Fprintf(out, "🎵 Audio: %d\n", c.Count)
			case *tg.InputMessagesFilterVoice:
				fmt.Fprintf(out, "🎤 Voice: %d\n", c.Count)
			case *tg.InputMessagesFilterGif:
				fmt.Fprintf(out, "😀 GIFs: %d\n", c.Count)
			}
		}
		fmt.Fprintln(out, "==================================================")
	}

	if saveInfo {
		fmt.Printf("\n✅ Info saved to %s\n", fileName)
	}
	return nil
}

func fetchChannel(ctx context.Context, api *tg.Client, target *tg.InputChannel) (*tg.Channel, error) {
	res, err := api.ChannelsGetChannels(ctx, []tg.InputChannelClass{target})
	if err != nil {
		return nil, fmt.Errorf("error fetching basic channel info: %w", err)
	}
	switch chs := res.(type) {
	case *tg.MessagesChats:
		for _, chat := range chs.Chats {
			if c, ok := chat.(*tg.Channel); ok && c.ID == target.ChannelID {
				return c, nil
			}
		}
	case *tg.MessagesChatsSlice:
		for _, chat := range chs.Chats {
			if c, ok := chat.(*tg.Channel); ok && c.ID == target.ChannelID {
				return c, nil
			}
		}
	}
	return nil, fmt.Errorf("channel info not found")
}

func manualChannelCount(ctx context.Context, api *tg.Client, peer *tg.InputPeerChannel, out io.Writer, showInfo bool) {
	offsetID, limit := 0, 100
	videoCount, imageCount, audioCount, voiceCount, gifCount, fileCount, linkCount, totalMessages := 0, 0, 0, 0, 0, 0, 0, 0

	for {
		messages, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer: peer, OffsetID: offsetID, Limit: limit,
		})
		if err != nil {
			if tgtg.HandleFloodWait(err) {
				continue
			}
			fmt.Fprintf(out, "⚠ Error during manual count: %v\n", err)
			break
		}
		var msgs []*tg.Message
		if m, ok := messages.(*tg.MessagesChannelMessages); ok {
			for _, mc := range m.Messages {
				if msg, ok := mc.(*tg.Message); ok {
					msgs = append(msgs, msg)
				}
			}
		}
		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			totalMessages++
			if msg.Media != nil {
				switch media := msg.Media.(type) {
				case *tg.MessageMediaDocument:
					if doc, ok := media.Document.AsNotEmpty(); ok {
						ext := tgtg.GetFileExt(doc)
						switch {
						case ext == ".mp4" || ext == ".mkv" || ext == ".avi":
							videoCount++
						case ext == ".mp3" || ext == ".flac":
							audioCount++
						case ext == ".ogg":
							isVoice := false
							for _, attr := range doc.Attributes {
								if _, ok := attr.(*tg.DocumentAttributeAudio); ok {
									isVoice = true
									break
								}
							}
							if isVoice {
								voiceCount++
							} else {
								audioCount++
							}
						case ext == ".gif":
							gifCount++
						case ext == ".jpg" || ext == ".jpeg" || ext == ".png":
							imageCount++
						default:
							fileCount++
						}
					}
				case *tg.MessageMediaPhoto:
					imageCount++
				}
			}
			for _, entity := range msg.Entities {
				if _, ok := entity.(*tg.MessageEntityURL); ok {
					linkCount++
					break
				}
				if _, ok := entity.(*tg.MessageEntityTextURL); ok {
					linkCount++
					break
				}
			}
		}

		offsetID = msgs[len(msgs)-1].ID
		if showInfo {
			fmt.Printf("\r🔄 Scanned %d messages...", totalMessages)
		}
		time.Sleep(200 * time.Millisecond)
	}

	if showInfo {
		fmt.Println()
	}
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out, "📊 Media Summary (Manual Count)")
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "🖼️ Photos: %d\n", imageCount)
	fmt.Fprintf(out, "🎬 Videos: %d\n", videoCount)
	fmt.Fprintf(out, "🎵 Audio: %d\n", audioCount)
	fmt.Fprintf(out, "🎤 Voice: %d\n", voiceCount)
	fmt.Fprintf(out, "😀 GIFs: %d\n", gifCount)
	fmt.Fprintf(out, "📁 Files: %d\n", fileCount)
	fmt.Fprintf(out, "🔗 Links: %d\n", linkCount)
	fmt.Fprintf(out, "✉️ Total messages: %d\n", totalMessages)
	fmt.Fprintln(out, "==================================================")
}

// silence unused-import guard (kept for clarity)
var _ = strings.TrimSpace