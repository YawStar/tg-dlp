package backup

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

// RestoreChat reads a JSON file and sends messages to the target channel.
func RestoreChat(ctx context.Context, api *tg.Client, targetChannel *tg.InputChannel, jsonFile string) error {
	file, err := os.Open(jsonFile)
	if err != nil {
		return fmt.Errorf("cannot open JSON file: %w", err)
	}
	defer file.Close()

	var messages []ExportedMessage
	if err := json.NewDecoder(file).Decode(&messages); err != nil {
		return fmt.Errorf("cannot parse JSON: %w", err)
	}

	fmt.Printf("📤 Restoring %d messages...\n", len(messages))

	var peer tg.InputPeerClass
	if targetChannel == nil {
		peer = &tg.InputPeerSelf{} // Saved Messages
		fmt.Println("🎯 Target: Saved Messages")
	} else {
		peer = &tg.InputPeerChannel{
			ChannelID:  targetChannel.ChannelID,
			AccessHash: targetChannel.AccessHash,
		}
		fmt.Printf("🎯 Target: Channel ID %d\n", targetChannel.ChannelID)
	}

	sender := message.NewSender(api)
	count := 0

	for i, msg := range messages {
		// စာသားမရှိရင် Media အမျိုးအစားနဲ့ ဖိုင်နာမည်ကို သုံး
		textToSend := msg.Text
		if textToSend == "" {
			if msg.FileName != "" {
				textToSend = fmt.Sprintf("[%s] %s", msg.MediaType, msg.FileName)
			} else if msg.MediaType != "" {
				textToSend = fmt.Sprintf("[%s] (ID: %d)", msg.MediaType, msg.ID)
			} else {
				textToSend = fmt.Sprintf("(Empty Message ID: %d)", msg.ID)
			}
		}

		_, err := sender.To(peer).Text(ctx, textToSend)
		if err != nil {
			fmt.Printf("\n⚠ Failed to send message %d: %v\n", msg.ID, err)
			continue
		}

		count++
		fmt.Printf("\r✅ Restored %d/%d messages...", i+1, len(messages))
		time.Sleep(100 * time.Millisecond) // Avoid flood
	}

	fmt.Printf("\n🎉 Restore complete! Sent %d messages.\n", count)
	return nil
}