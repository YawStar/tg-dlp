package cmd

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"

	"tg-dlp/core/client"
	coreuploader "tg-dlp/core/uploader"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// RunUploadCmd implements the "upload"/"up" command.
func RunUploadCmd(args []string, appID int, appHash, sessionPath string) error {
	upCmd := flag.NewFlagSet("upload", flag.ExitOnError)
	upCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp upload [options]

Options:
  -p, --path string    File or directory path to upload (required)
  -c, --chat string    Target chat (username, ID, or link). Default: Saved Messages
  -i, --include string Only upload files with these extensions (e.g., jpg,png)
  -e, --exclude string Exclude files with these extensions (e.g., mp4,flv)
  --photo              Force upload images as photos
  --rm                 Delete local files after successful upload
  --skip-flood         Skip waiting on FLOOD_WAIT
  --proxy string       Manual proxy URL (e.g., http://127.0.0.1:8080)
  --no-proxy           Do not use system proxy
`)
	}

	var paths StringSlice
	upCmd.Var(&paths, "p", "File or directory path")
	upCmd.Var(&paths, "path", "File or directory path")

	chatPtr := upCmd.String("c", "", "Target chat (username, ID, or link)")
	upCmd.StringVar(chatPtr, "chat", "", "Target chat (username, ID, or link)")
	includePtr := upCmd.String("i", "", "Only upload files with these extensions")
	upCmd.StringVar(includePtr, "include", "", "Only upload files with these extensions")
	excludePtr := upCmd.String("e", "", "Exclude files with these extensions")
	upCmd.StringVar(excludePtr, "exclude", "", "Exclude files with these extensions")
	photoPtr := upCmd.Bool("photo", false, "Force upload images as photos")
	rmPtr := upCmd.Bool("rm", false, "Remove local files after upload")
	proxyPtr := upCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := upCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := upCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")

	upCmd.Parse(args)

	runtime.SkipFlood = *skipFloodPtr

	if len(paths) == 0 {
		fmt.Println("❌ No paths provided. Use -p <path> to upload.")
		upCmd.Usage()
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

		peer, err := resolveUploadPeer(ctx, api, *chatPtr)
		if err != nil {
			return err
		}

		up := uploader.NewUploader(api)
		sender := message.NewSender(api)
		includeExts := tgtg.ParseExtensions(*includePtr)
		excludeExts := tgtg.ParseExtensions(*excludePtr)

		for _, path := range paths {
			info, err := os.Stat(path)
			if err != nil {
				fmt.Printf("❌ Cannot access path '%s': %v\n", path, err)
				continue
			}
			if info.IsDir() {
				err = filepath.Walk(path, func(filePath string, fi os.FileInfo, err error) error {
					if err != nil {
						return err
					}
					if fi.IsDir() {
						return nil
					}
					return coreuploader.SingleFile(ctx, sender, up, peer, filePath, *photoPtr, *rmPtr, includeExts, excludeExts)
				})
				if err != nil {
					fmt.Printf("❌ Error walking directory '%s': %v\n", path, err)
				}
			} else {
				if err := coreuploader.SingleFile(ctx, sender, up, peer, path, *photoPtr, *rmPtr, includeExts, excludeExts); err != nil {
					fmt.Printf("❌ Error uploading '%s': %v\n", path, err)
				}
			}
		}
		fmt.Println("\n🎉 Upload tasks completed!")
		return nil
	})
}

func resolveUploadPeer(ctx context.Context, api *tg.Client, chatStr string) (tg.InputPeerClass, error) {
	if chatStr == "" {
		fmt.Println("📤 Uploading to Saved Messages...")
		return &tg.InputPeerSelf{}, nil
	}

	// Private Link (t.me/c/...) ဖြစ်နေရင် Channel ID ကို ဆွဲထုတ်မယ်
	if strings.Contains(chatStr, "t.me/c/") {
		parts := strings.Split(chatStr, "/")
		for i, p := range parts {
			if p == "c" && i+1 < len(parts) {
				idStr := parts[i+1]
				id, err := strconv.ParseInt(idStr, 10, 64)
				if err == nil {
					if id > 0 && id < 1000000000000 {
						id = int64(1000000000000) + id
					}

					// Message Link ကနေ Channel ကို ရှာမယ် (AccessHash ပါဝင်လာမယ်)
					peer := &tg.InputPeerChannel{ChannelID: id, AccessHash: 0}
					
					// History ကို ခေါ်ကြည့်မယ် (AccessHash ကို Auto ဖြည့်ပေး)
					_, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
						Peer:  peer,
						Limit: 1,
					})
					if err == nil {
						// အောင်မြင်ရင် AccessHash ကို ရှာတွေ့မယ်
						fmt.Printf("🎯 Target: Private Channel ID %d\n", id)
						return peer, nil
					}

					// မရရင် fallback အနေနဲ့ FetchChannelByID ကို ပြန်သုံးမယ်
					ch, err := tgtg.FetchChannelByID(ctx, api, id)
					if err != nil {
						return nil, fmt.Errorf("cannot resolve private channel ID %d: %w", id, err)
					}
					fmt.Printf("🎯 Target: Private Channel ID %d\n", id)
					return &tg.InputPeerChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash}, nil
				}
			}
		}
	}

	// ရိုးရိုး Username ဒါမှမဟုတ် Public Link ဖြစ်ရင်
	s := strings.TrimPrefix(chatStr, "@")
	if resolved, err := tgtg.ResolveUsername(ctx, api, s); err == nil {
		fmt.Printf("🎯 Target: @%s\n", s)
		return &tg.InputPeerChannel{ChannelID: resolved.ChannelID, AccessHash: resolved.AccessHash}, nil
	}

	// ဂဏန်းသက်သက် ID ဖြစ်ရင်
	if id, err := strconv.ParseInt(s, 10, 64); err == nil {
		if ch, err := tgtg.FetchChannelByID(ctx, api, id); err == nil {
			return &tg.InputPeerChannel{ChannelID: ch.ChannelID, AccessHash: ch.AccessHash}, nil
		}
		return &tg.InputPeerUser{UserID: id}, nil
	}
	return nil, fmt.Errorf("cannot resolve chat '%s'", chatStr)
}