package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gotd/td/tg"

	"tg-dlp/core/client"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// RunDeleteCmd implements "delete"/"del"/"rm".
func RunDeleteCmd(args []string, appID int, appHash, sessionPath string) error {
	delCmd := flag.NewFlagSet("delete", flag.ExitOnError)
	delCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp delete [options]

Options:
  -u, --url string    Telegram message link (e.g., https://t.me/channel/123)
  --skip-flood        Skip waiting on FLOOD_WAIT
  --proxy string      Manual proxy URL (e.g., http://127.0.0.1:8080)
  --no-proxy          Do not use system proxy
`)
	}

	urlPtr := delCmd.String("u", "", "Telegram message link")
	delCmd.StringVar(urlPtr, "url", "", "Telegram message link")
	proxyPtr := delCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := delCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := delCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")

	delCmd.Parse(args)

	runtime.SkipFlood = *skipFloodPtr

	if *urlPtr == "" {
		fmt.Println("❌ Please provide a message URL using -u <url>.")
		delCmd.Usage()
		return nil
	}

	cli, err := client.GetClient(appID, appHash, sessionPath, *noProxyPtr, *proxyPtr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() { <-sigChan; cancel() }()

	return cli.Run(ctx, func(ctx context.Context) error {
		if err := client.CheckAuth(ctx, cli); err != nil {
			return err
		}
		api := tg.NewClient(cli)

		linkInfo, err := tgtg.ParseLink(*urlPtr)
		if err != nil {
			return fmt.Errorf("invalid link format: %w", err)
		}
		if linkInfo.MessageID == 0 {
			return fmt.Errorf("no message ID found in the URL")
		}

		fmt.Printf("🔍 Resolving chat for message ID: %d...\n", linkInfo.MessageID)
		var targetChannel *tg.InputChannel
		if linkInfo.IsUsername {
			targetChannel, err = tgtg.ResolveUsername(ctx, api, linkInfo.Username)
			if err != nil {
				return fmt.Errorf("cannot resolve username: %w", err)
			}
		} else {
			targetChannel, err = tgtg.FetchChannelByID(ctx, api, linkInfo.ChannelID)
			if err != nil {
				return fmt.Errorf("channel not found or you are not a member: %w", err)
			}
		}

		fmt.Printf("🗑️ Deleting message %d from channel...\n", linkInfo.MessageID)
		if _, err := api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
			Channel: targetChannel,
			ID:      []int{linkInfo.MessageID},
		}); err != nil {
			return fmt.Errorf("delete request failed: %w", err)
		}

		fmt.Println("🔍 Verifying deletion...")
		time.Sleep(2 * time.Second)

		messagesRes, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
			Channel: targetChannel,
			ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: linkInfo.MessageID}},
		})
		if err != nil {
			if strings.Contains(err.Error(), "MSG_ID_INVALID") || strings.Contains(err.Error(), "MESSAGE_ID_INVALID") {
				fmt.Println("✅ Message deleted successfully!")
				return nil
			}
			fmt.Printf("⚠ Could not verify deletion: %v\n", err)
			fmt.Println("✅ Delete request sent successfully (unverified)")
			return nil
		}

		if m, ok := messagesRes.(*tg.MessagesChannelMessages); ok {
			for _, mc := range m.Messages {
				if msg, ok := mc.(*tg.Message); ok && msg.ID == linkInfo.MessageID {
					return fmt.Errorf("❌ Message still exists after delete request (You might not have permission to delete it)")
				}
			}
		}
		fmt.Println("✅ Message deleted successfully!")
		return nil
	})
}