package info

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

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