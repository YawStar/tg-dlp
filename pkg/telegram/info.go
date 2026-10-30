package telegram

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/tg"
	"tg-dlp/pkg/fsutil"
	"tg-dlp/pkg/runtime"
)

// ShowChannelInfo prints (and optionally saves) channel info.
func ShowChannelInfo(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, showInfo, saveInfo bool, outputDir string) error {
	out, file, fileName, err := setupOutput(saveInfo, showInfo, outputDir, "")
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}
	defer func() {
		if saveInfo && fileName != "" {
			fmt.Printf("\n✅ Info saved to %s\n", fileName)
		}
	}()

	ch, err := fetchChannel(ctx, api, targetChannel)
	if err != nil {
		return err
	}
	if saveInfo && fileName == "" {
		// Re-create with proper filename now that we know the title.
		_ = file
	}
	// If we need save but used empty filename, recreate.
	if saveInfo && fileName == "" {
		safeTitle := fsutil.SanitizeFilename(ch.Title)
		path := filepath.Join(outputDir, fmt.Sprintf("%s_info.txt", safeTitle))
		f, err := os.Create(path)
		if err != nil {
			return fmt.Errorf("cannot create info file: %w", err)
		}
		file = f
		fileName = path
		if showInfo {
			out = io.MultiWriter(os.Stdout, file)
		} else {
			out = file
		}
		defer file.Close()
	}

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
		messages, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{Peer: peer, OffsetID: offsetID, Limit: limit})
		if err != nil {
			if HandleFloodWait(err) {
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
						ext := GetFileExt(doc)
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

// ShowPostInfo prints (and optionally saves) one post's info.
func ShowPostInfo(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, msgID int, showInfo, saveInfo bool, outputDir string) error {
	out, file, fileName, err := setupOutput(saveInfo, showInfo, outputDir, fmt.Sprintf("post_%d_info.txt", msgID))
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}

	messages, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
		Channel: targetChannel,
		ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: msgID}},
	})
	if err != nil {
		return fmt.Errorf("error fetching message: %w", err)
	}
	var msg *tg.Message
	if m, ok := messages.(*tg.MessagesChannelMessages); ok && len(m.Messages) > 0 {
		if msgStruct, ok := m.Messages[0].(*tg.Message); ok {
			msg = msgStruct
		}
	}
	if msg == nil {
		return fmt.Errorf("message not found")
	}

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "🆔 Message ID: %d\n", msg.ID)
	fmt.Fprintf(out, "📅 Date: %s\n", time.Unix(int64(msg.Date), 0).Format("2006-01-02 15:04:05"))
	fmt.Fprintf(out, "📝 Text: %s\n", msg.Message)

	if msg.Media != nil {
		switch media := msg.Media.(type) {
		case *tg.MessageMediaDocument:
			if doc, ok := media.Document.AsNotEmpty(); ok {
				ext := GetFileExt(doc)
				fmt.Fprintf(out, "📁 File Type: %s\n", ext)
				fmt.Fprintf(out, "📦 File Size: %.2f MB\n", float64(doc.Size)/(1024*1024))
				for _, attr := range doc.Attributes {
					if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
						fmt.Fprintf(out, "📄 File Name: %s\n", fn.FileName)
					}
				}
			}
		case *tg.MessageMediaPhoto:
			photo, _ := media.Photo.AsNotEmpty()
			_, size := PickLargestPhotoSize(photo.Sizes)
			fmt.Fprintf(out, "🖼️ Image Type: JPEG\n")
			fmt.Fprintf(out, "📦 Image Size: %.2f KB\n", float64(size)/1024)
		default:
			fmt.Fprintf(out, "📎 Media Type: %T\n", msg.Media)
		}
	} else {
		fmt.Fprintln(out, "❌ No media in this post.")
	}
	fmt.Fprintln(out, "==================================================")

	if saveInfo {
		fmt.Printf("\n✅ Post info saved to %s\n", fileName)
	}
	return nil
}

// ShowThreadInfo prints (and optionally saves) a forum thread's info.
func ShowThreadInfo(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, threadID int, showInfo, saveInfo bool, outputDir string) error {
	out, file, fileName, err := setupOutput(saveInfo, showInfo, outputDir, fmt.Sprintf("thread_%d_info.txt", threadID))
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}

	peer := &tg.InputPeerChannel{ChannelID: targetChannel.ChannelID, AccessHash: targetChannel.AccessHash}
	replies, err := api.MessagesGetReplies(ctx, &tg.MessagesGetRepliesRequest{
		Peer: peer, MsgID: threadID, OffsetID: 0, Limit: 100,
	})
	if err != nil {
		return fmt.Errorf("error fetching thread: %w", err)
	}
	var msgs []*tg.Message
	var totalReplies int
	if m, ok := replies.(*tg.MessagesChannelMessages); ok {
		totalReplies = m.Count
		for _, mc := range m.Messages {
			if msg, ok := mc.(*tg.Message); ok {
				msgs = append(msgs, msg)
			}
		}
	}

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "💬 Thread ID: %d\n", threadID)
	fmt.Fprintf(out, "✉️ Total Replies: %d\n", totalReplies)
	fmt.Fprintln(out, "==================================================")
	mediaCount := 0
	for _, msg := range msgs {
		if msg.Media != nil {
			mediaCount++
		}
	}
	fmt.Fprintf(out, "📁 Media found in this page (100 msgs): %d\n", mediaCount)
	fmt.Fprintln(out, "==================================================")

	if saveInfo {
		fmt.Printf("\n✅ Thread info saved to %s\n", fileName)
	}
	return nil
}

// ShowSelfInfo prints (and optionally saves) the current user's info.
func ShowSelfInfo(ctx context.Context, api *tg.Client, showInfo, saveInfo bool, outputDir string) error {
	out, file, fileName, err := setupOutput(saveInfo, showInfo, outputDir, "my_account_info.txt")
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}

	if showInfo {
		fmt.Println("🔍 Fetching your account information...")
	}

	res, err := api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return fmt.Errorf("error fetching self info: %w", err)
	}
	if len(res.Users) == 0 {
		return fmt.Errorf("no user info returned")
	}
	user, ok := res.Users[0].(*tg.User)
	if !ok {
		return fmt.Errorf("could not parse user info")
	}
	fullUser := res.FullUser

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out, "👤 Your Account Information")
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "📛 Name: %s %s\n", user.FirstName, user.LastName)
	fmt.Fprintf(out, "🆔 User ID: %d\n", user.ID)
	if user.Username != "" {
		fmt.Fprintf(out, "🔗 Username: @%s\n", user.Username)
	}
	if user.Phone != "" {
		fmt.Fprintf(out, "📞 Phone: +%s\n", user.Phone)
	}
	if fullUser.About != "" {
		fmt.Fprintf(out, "📝 Bio: %s\n", fullUser.About)
	}
	if fullUser.Birthday.Day != 0 || fullUser.Birthday.Month != 0 {
		fmt.Fprintf(out, "🎂 Birthday: %d/%d\n", fullUser.Birthday.Day, fullUser.Birthday.Month)
		if fullUser.Birthday.Year != 0 {
			fmt.Fprintf(out, "   Year: %d\n", fullUser.Birthday.Year)
		}
	}
	fmt.Fprintln(out, "==================================================")

	if saveInfo {
		fmt.Printf("\n✅ Self info saved to %s\n", fileName)
	}
	return nil
}

// setupOutput prepares output writer + file.
// If filename is empty, the caller must create the file later.
func setupOutput(saveInfo, showInfo bool, outputDir, filename string) (io.Writer, *os.File, string, error) {
	if !saveInfo {
		return os.Stdout, nil, "", nil
	}
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, nil, "", err
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

var _ = runtime.DebugLog // keep import if unused otherwise