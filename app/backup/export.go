package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gotd/td/tg"
)

// ExportedMessage represents a single message in the backup.
type ExportedMessage struct {
	ID        int       `json:"id"`
	Date      time.Time `json:"date"`
	Text      string    `json:"text"`
	MediaType string    `json:"media_type,omitempty"`
	FileName  string    `json:"file_name,omitempty"`
	FileSize  int64     `json:"file_size,omitempty"`
}

// ExportChat exports all messages from a channel to a JSON file.
func ExportChat(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, outputDir string, limit int) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}

	peer := &tg.InputPeerChannel{
		ChannelID:  targetChannel.ChannelID,
		AccessHash: targetChannel.AccessHash,
	}

	fileName := filepath.Join(outputDir, fmt.Sprintf("export_%d_%s.json", targetChannel.ChannelID, time.Now().Format("20060102_150405")))
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("cannot create export file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")

	var messages []ExportedMessage
	offsetID := 0

	fmt.Printf("📤 Exporting messages...\n")

	for {
		if limit > 0 && len(messages) >= limit {
			break
		}

		reqLimit := 100
		if limit > 0 && limit-len(messages) < 100 {
			reqLimit = limit - len(messages)
		}

		res, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     peer,
			OffsetID: offsetID,
			Limit:    reqLimit,
		})
		if err != nil {
			return fmt.Errorf("error fetching messages: %w", err)
		}

		var msgs []*tg.Message
		if m, ok := res.(*tg.MessagesChannelMessages); ok {
			for _, msgClass := range m.Messages {
				if msg, ok := msgClass.(*tg.Message); ok {
					msgs = append(msgs, msg)
				}
			}
		}

		if len(msgs) == 0 {
			break
		}

		for _, msg := range msgs {
			exported := ExportedMessage{
				ID:   msg.ID,
				Date: time.Unix(int64(msg.Date), 0),
				Text: msg.Message,
			}

			if msg.Media != nil {
				switch media := msg.Media.(type) {
				case *tg.MessageMediaDocument:
					if doc, ok := media.Document.AsNotEmpty(); ok {
						exported.MediaType = "document"
						exported.FileSize = doc.Size
						for _, attr := range doc.Attributes {
							if fn, ok := attr.(*tg.DocumentAttributeFilename); ok {
								exported.FileName = fn.FileName
							}
						}
					}
				case *tg.MessageMediaPhoto:
					exported.MediaType = "photo"
				}
			}
			messages = append(messages, exported)
			fmt.Printf("\r📥 Exported %d messages...", len(messages))
		}

		offsetID = msgs[len(msgs)-1].ID
		time.Sleep(200 * time.Millisecond)
	}

	if err := encoder.Encode(messages); err != nil {
		return fmt.Errorf("failed to write JSON: %w", err)
	}

	fmt.Printf("\n✅ Exported %d messages to %s\n", len(messages), fileName)
	return nil
}