package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gotd/td/tg"

	"tg-dlp/core/client"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// RunPurgeCmd implements "purge"/"clear".
func RunPurgeCmd(args []string, appID int, appHash, sessionPath string) error {
	purgeCmd := flag.NewFlagSet("purge", flag.ExitOnError)
	purgeCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp purge [options]

Options:
  -c, --chat string    Target channel (username, ID, or link) - Required
  -i, --include string Only delete specific file extensions (e.g., jpg,png)
  -e, --exclude string Exclude specific file extensions (e.g., mp4,flv)
  --text               Delete text messages only (no media)
  --media              Delete media messages only
  -y, --yes            Skip confirmation prompt
  --skip-flood         Skip waiting on FLOOD_WAIT
  --proxy string       Manual proxy URL (e.g., http://127.0.0.1:8080)
  --no-proxy           Do not use system proxy
`)
	}

	chatPtr := purgeCmd.String("c", "", "Target channel")
	purgeCmd.StringVar(chatPtr, "chat", "", "Target channel")
	includePtr := purgeCmd.String("i", "", "Only delete files with these extensions")
	purgeCmd.StringVar(includePtr, "include", "", "Only delete files with these extensions")
	excludePtr := purgeCmd.String("e", "", "Exclude files with these extensions")
	purgeCmd.StringVar(excludePtr, "exclude", "", "Exclude files with these extensions")
	textPtr := purgeCmd.Bool("text", false, "Delete text messages only")
	mediaPtr := purgeCmd.Bool("media", false, "Delete media messages only")
	yesPtr := purgeCmd.Bool("y", false, "Skip confirmation")
	purgeCmd.BoolVar(yesPtr, "yes", false, "Skip confirmation")
	proxyPtr := purgeCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := purgeCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := purgeCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")

	purgeCmd.Parse(args)

	runtime.SkipFlood = *skipFloodPtr

	if *chatPtr == "" {
		fmt.Println("❌ Please provide a chat using -c <chat>.")
		purgeCmd.Usage()
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
	go func() { <-sigChan; fmt.Println("\n🛑 Stopping..."); cancel() }()

	return cli.Run(ctx, func(ctx context.Context) error {
		if err := client.CheckAuth(ctx, cli); err != nil {
			return err
		}
		api := tg.NewClient(cli)

		targetChannel, err := resolvePurgeChannel(ctx, api, *chatPtr)
		if err != nil {
			return err
		}

		includeExts := tgtg.ParseExtensions(*includePtr)
		excludeExts := tgtg.ParseExtensions(*excludePtr)

		if !*yesPtr {
			count, err := countPurgeableMessages(ctx, api, targetChannel, *textPtr, *mediaPtr, includeExts, excludeExts)
			if err != nil {
				return err
			}
			fmt.Printf("⚠️  Found approximately %d messages to delete from channel %d.\n", count, targetChannel.ChannelID)
			fmt.Print("Type 'yes' to confirm deletion: ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "yes" {
				fmt.Println("❌ Cancelled.")
				return nil
			}
		}

		return executePurge(ctx, api, targetChannel, *textPtr, *mediaPtr, includeExts, excludeExts)
	})
}

func resolvePurgeChannel(ctx context.Context, api *tg.Client, chatStr string) (*tg.InputChannel, error) {
	s := strings.TrimPrefix(chatStr, "@")
	if strings.Contains(s, "t.me/c/") {
		parts := strings.Split(s, "/")
		for i, p := range parts {
			if p == "c" && i+1 < len(parts) {
				s = parts[i+1]
				break
			}
		}
	}
	if ch, err := tgtg.ResolveUsername(ctx, api, s); err == nil {
		return ch, nil
	}
	if id, err := strconv.ParseInt(s, 10, 64); err == nil {
		if ch, err := tgtg.FetchChannelByID(ctx, api, id); err == nil {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("cannot resolve channel '%s'", chatStr)
}

func shouldDelete(msg *tg.Message, textOnly, mediaOnly bool, includeExts, excludeExts map[string]bool) bool {
	if textOnly && msg.Media != nil {
		return false
	}
	if mediaOnly && msg.Media == nil {
		return false
	}
	if msg.Media != nil && (len(includeExts) > 0 || len(excludeExts) > 0) {
		var ext string
		if media, ok := msg.Media.(*tg.MessageMediaDocument); ok {
			if doc, ok := media.Document.AsNotEmpty(); ok {
				ext = strings.TrimPrefix(tgtg.GetFileExt(doc), ".")
			}
		}
		if len(includeExts) > 0 && !includeExts[ext] {
			return false
		}
		if len(excludeExts) > 0 && excludeExts[ext] {
			return false
		}
	}
	return true
}

func fetchHistoryPage(ctx context.Context, api *tg.Client, peer *tg.InputPeerChannel, offsetID, limit int) ([]*tg.Message, error) {
	for {
		messages, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer: peer, OffsetID: offsetID, Limit: limit,
		})
		if err != nil {
			if tgtg.HandleFloodWait(err) {
				continue
			}
			return nil, err
		}
		var msgs []*tg.Message
		if m, ok := messages.(*tg.MessagesChannelMessages); ok {
			for _, mc := range m.Messages {
				if msg, ok := mc.(*tg.Message); ok {
					msgs = append(msgs, msg)
				}
			}
		}
		return msgs, nil
	}
}

func countPurgeableMessages(ctx context.Context, api *tg.Client, target *tg.InputChannel, textOnly, mediaOnly bool, includeExts, excludeExts map[string]bool) (int, error) {
	fmt.Println("🔍 Scanning messages to count...")
	peer := &tg.InputPeerChannel{ChannelID: target.ChannelID, AccessHash: target.AccessHash}
	offsetID, limit, total := 0, 100, 0
	for {
		msgs, err := fetchHistoryPage(ctx, api, peer, offsetID, limit)
		if err != nil {
			return total, fmt.Errorf("error fetching messages: %w", err)
		}
		if len(msgs) == 0 {
			break
		}
		for _, msg := range msgs {
			if shouldDelete(msg, textOnly, mediaOnly, includeExts, excludeExts) {
				total++
			}
		}
		offsetID = msgs[len(msgs)-1].ID
		time.Sleep(100 * time.Millisecond)
	}
	return total, nil
}

func executePurge(ctx context.Context, api *tg.Client, target *tg.InputChannel, textOnly, mediaOnly bool, includeExts, excludeExts map[string]bool) error {
	fmt.Println("🔍 Starting purge...")
	peer := &tg.InputPeerChannel{ChannelID: target.ChannelID, AccessHash: target.AccessHash}
	offsetID, limit := 0, 100
	totalDeleted, totalSkipped, totalScanned := 0, 0, 0

	for {
		msgs, err := fetchHistoryPage(ctx, api, peer, offsetID, limit)
		if err != nil {
			return fmt.Errorf("error fetching messages: %w", err)
		}
		if len(msgs) == 0 {
			break
		}

		var toDelete []int
		for _, msg := range msgs {
			totalScanned++
			if shouldDelete(msg, textOnly, mediaOnly, includeExts, excludeExts) {
				toDelete = append(toDelete, msg.ID)
			} else {
				totalSkipped++
			}
		}

		if len(toDelete) > 0 {
			if _, err := api.ChannelsDeleteMessages(ctx, &tg.ChannelsDeleteMessagesRequest{
				Channel: target, ID: toDelete,
			}); err != nil {
				fmt.Printf("\n⚠ Error deleting batch: %v\n", err)
			} else {
				totalDeleted += len(toDelete)
			}
		}

		offsetID = msgs[len(msgs)-1].ID
		fmt.Printf("\r🗑️  Scanned: %d | Deleted: %d | Skipped: %d", totalScanned, totalDeleted, totalSkipped)
		time.Sleep(300 * time.Millisecond)
	}

	fmt.Printf("\n\n🎉 Purge finished!\n")
	fmt.Printf("📊 Total Scanned: %d\n", totalScanned)
	fmt.Printf("✅ Deleted: %d\n", totalDeleted)
	fmt.Printf("⏭️  Skipped: %d\n", totalSkipped)
	return nil
}