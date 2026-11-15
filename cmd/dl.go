package cmd

import (
	"context"
	"flag"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/telegram/takeout"
	"github.com/gotd/td/tg"

	"tg-dlp/core/client"
	"tg-dlp/core/downloader"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// parseSize parses human-readable rate strings like "500K", "2M", "1024" into bytes per second (int64)
func parseSize(s string) int64 {
	s = strings.TrimSpace(strings.ToUpper(s))
	if s == "" {
		return 0
	}

	multiplier := int64(1)
	if strings.HasSuffix(s, "K") {
		multiplier = 1024
		s = strings.TrimSuffix(s, "K")
	} else if strings.HasSuffix(s, "M") {
		multiplier = 1024 * 1024
		s = strings.TrimSuffix(s, "M")
	} else if strings.HasSuffix(s, "G") {
		multiplier = 1024 * 1024 * 1024
		s = strings.TrimSuffix(s, "G")
	}

	val, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}

	return val * multiplier
}

// RunDownloadCmd implements the "download"/"dl" command.
func RunDownloadCmd(args []string, appID int, appHash, sessionPath string) error {
	dlCmd := flag.NewFlagSet("download", flag.ExitOnError)
	dlCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp download [options]

Options:
  -u, --url string       Telegram link (Message or Channel)
  -d, --dir string       Output directory (Default: "downloads")
  -i, --filter string    Filter file types (e.g., mp4,mkv,mp3)
  -t, --template string  Filename template (e.g., "{id}_{ext}")
  -c, --concurrency int  Number of concurrent workers for downloading (Default: 3)
  -r, --limit-rate string Download speed limit (e.g., 500K, 1M)
  --takeout              Use Takeout Session to reduce rate limits
  --auto-join            Automatically join private channels
  --skip-same            Skip downloading if file already exists
  --overwrite            Overwrite existing files
  --skip-flood           Skip waiting on FLOOD_WAIT
  --dry-run              List what would be downloaded without downloading
  --proxy string         Manual proxy URL (e.g., http://127.0.0.1:8080)
  --no-proxy             Do not use system proxy
  -debug, --debug        Show detailed logs
`)
	}

	autoJoinPtr := dlCmd.Bool("auto-join", false, "Automatically join channels")
	debugPtr := dlCmd.Bool("debug", false, "Show detailed logs")
	takeoutPtr := dlCmd.Bool("takeout", false, "Use Takeout Session to reduce rate limits")
	skipSamePtr := dlCmd.Bool("skip-same", false, "Skip downloading if file already exists")
	overwritePtr := dlCmd.Bool("overwrite", false, "Overwrite existing files")
	skipFloodPtr := dlCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")
	dryRunPtr := dlCmd.Bool("dry-run", false, "List what would be downloaded without downloading")
	proxyPtr := dlCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := dlCmd.Bool("no-proxy", false, "Do not use system proxy")

	// Concurrency flag addition
	concurrencyPtr := dlCmd.Int("concurrency", 3, "Number of concurrent workers for downloading")
	dlCmd.IntVar(concurrencyPtr, "c", 3, "Number of concurrent workers for downloading")

	// Limit Rate flag addition
	limitRatePtr := dlCmd.String("limit-rate", "", "Download speed limit (e.g., 500K, 1M)")
	dlCmd.StringVar(limitRatePtr, "r", "", "Download speed limit (shorthand)")

	templatePtr := dlCmd.String("t", "", "Filename template")
	dlCmd.StringVar(templatePtr, "template", "", "Filename template")

	var links StringSlice
	dlCmd.Var(&links, "u", "Links to download")
	dlCmd.Var(&links, "url", "Links to download")

	outDirPtr := dlCmd.String("d", "downloads", "Output directory")
	dlCmd.StringVar(outDirPtr, "dir", "downloads", "Output directory")

	filterPtr := dlCmd.String("i", "", "File types to download")
	dlCmd.StringVar(filterPtr, "filter", "", "File types to download")

	dlCmd.Parse(args)

	runtime.DebugMode = *debugPtr
	runtime.SkipFlood = *skipFloodPtr

	if len(links) == 0 {
		fmt.Println("❌ No links provided. Use -u <link> to download.")
		dlCmd.Usage()
		return nil
	}

	filterExts := filterSetToSlice(tgtg.ParseExtensions(*filterPtr))

	cli, err := client.GetClient(appID, appHash, sessionPath, *noProxyPtr, *proxyPtr)
	if err != nil {
		return err
	}

	// Parse limit-rate string to bytes per second
	limitBytes := parseSize(*limitRatePtr)

	// Initialize downloader with custom concurrency and limit rate options
	dlOpts := downloader.Options{
		Concurrency: *concurrencyPtr,
		LimitRate:   limitBytes,
	}
	dl := downloader.NewWithConfig(dlOpts)

	return cli.Run(context.Background(), func(ctx context.Context) error {
		if err := client.CheckAuth(ctx, cli); err != nil {
			return err
		}

		if *takeoutPtr {
			fmt.Println("\n🔄 Starting Takeout Session...")
			cfg := takeout.Config{
				MessageChannels: true,
				Files:           true,
				FileMaxSize:     2 * 1024 * 1024 * 1024,
			}
			return takeout.Run(ctx, cli, cfg, func(ctx context.Context, to *takeout.Client) error {
				fmt.Println("✅ Takeout Session active. (Rate limit reduced)")
				api := tg.NewClient(to)
				return runDownloadLoop(ctx, api, links, *outDirPtr, filterExts, *autoJoinPtr, *skipSamePtr, *overwritePtr, *dryRunPtr, *templatePtr, dl)
			})
		}

		api := tg.NewClient(cli)
		return runDownloadLoop(ctx, api, links, *outDirPtr, filterExts, *autoJoinPtr, *skipSamePtr, *overwritePtr, *dryRunPtr, *templatePtr, dl)
	})
}

func runDownloadLoop(ctx context.Context, api *tg.Client, links []string, outDir string, filterExts []string, autoJoin, skipSame, overwrite, dryRun bool, templateStr string, dl *downloader.Downloader) error {
	downloadCtx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()

	for idx, link := range links {
		fmt.Printf("\n--- [%d/%d] %s ---\n", idx+1, len(links), link)

		linkInfo, err := tgtg.ParseLink(link)
		if err != nil {
			fmt.Printf("❌ Failed to parse link: %v\n", err)
			continue
		}

		var targetChannel *tg.InputChannel
		if linkInfo.IsUsername {
			fmt.Printf("🔍 Resolving username '%s'...\n", linkInfo.Username)
			targetChannel, err = tgtg.ResolveUsername(ctx, api, linkInfo.Username)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
				continue
			}
		} else {
			targetChannel, err = tgtg.FetchChannelByID(ctx, api, linkInfo.ChannelID)
			if err != nil {
				targetChannel = &tg.InputChannel{ChannelID: linkInfo.ChannelID, AccessHash: 0}
			}
		}

		if autoJoin {
			fmt.Println("🔗 Joining channel...")
			if err := tgtg.JoinChannel(ctx, api, targetChannel); err != nil {
				fmt.Printf("⚠ Could not join channel: %v\n", err)
			}
		}

		switch {
		case linkInfo.ThreadID > 0:
			if dryRun {
				fmt.Printf("🔍 [dry-run] Would download media from thread %d of channel %d\n", linkInfo.ThreadID, targetChannel.ChannelID)
				continue
			}
			if err := dl.ThreadMedia(downloadCtx, api, targetChannel, linkInfo.ThreadID, outDir, filterExts, skipSame, overwrite, templateStr); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}

		case linkInfo.IsChannelOnly:
			if dryRun {
				fmt.Printf("🔍 [dry-run] Would download all media from channel %d\n", targetChannel.ChannelID)
				continue
			}
			if err := dl.ChannelMedia(downloadCtx, api, targetChannel, outDir, filterExts, skipSame, overwrite, templateStr); err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}

		default:
			messages, err := api.ChannelsGetMessages(ctx, &tg.ChannelsGetMessagesRequest{
				Channel: targetChannel,
				ID:      []tg.InputMessageClass{&tg.InputMessageID{ID: linkInfo.MessageID}},
			})
			if err != nil {
				if strings.Contains(err.Error(), "CHANNEL_PRIVATE") || strings.Contains(err.Error(), "CHANNEL_INVALID") {
					fmt.Printf("❌ You are not a member of this channel. Use '--auto-join' to join.\n")
				} else {
					fmt.Printf("❌ Error fetching post: %v\n", err)
				}
				continue
			}
			var msg *tg.Message
			if m, ok := messages.(*tg.MessagesChannelMessages); ok && len(m.Messages) > 0 {
				if s, ok := m.Messages[0].(*tg.Message); ok {
					msg = s
				}
			}
			if msg == nil || msg.Media == nil {
				fmt.Println("❌ No media in this post.")
				continue
			}
			if dryRun {
				fmt.Printf("🔍 [dry-run] Would download media_%d from channel %d\n", linkInfo.MessageID, targetChannel.ChannelID)
				continue
			}
			downloaded, err := dl.SingleMedia(downloadCtx, api, msg, linkInfo.MessageID, outDir, filterExts, skipSame, overwrite, templateStr)
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			} else if downloaded {
				// Success
			}
		}
	}
	// fmt.Println("\n🎉 All tasks completed!")
	return nil
}
