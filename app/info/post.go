package info

import (
	"context"
	"fmt"
	"time"

	"github.com/gotd/td/tg"

	tgtg "tg-dlp/pkg/telegram"
)

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
		if s, ok := m.Messages[0].(*tg.Message); ok {
			msg = s
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
				ext := tgtg.GetFileExt(doc)
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
			_, size := tgtg.PickLargestPhotoSize(photo.Sizes)
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