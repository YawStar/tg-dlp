package cmd

import (
	"context"
	"flag"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"

	"tg-dlp/app/backup"
	"tg-dlp/core/client"
	tgtg "tg-dlp/pkg/telegram"
)

// RunRestoreCmd implements the "restore" command.
func RunRestoreCmd(args []string, appID int, appHash, sessionPath string) error {
	resCmd := flag.NewFlagSet("restore", flag.ExitOnError)
	resCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp restore [options]

Options:
  -f, --file string   JSON file to restore (Required)
  -c, --chat string   Target channel or "me" for Saved Messages
  --proxy string      Manual proxy URL
  --no-proxy          Do not use system proxy
`)
	}

	filePtr := resCmd.String("f", "", "JSON file to restore")
	resCmd.StringVar(filePtr, "file", "", "JSON file to restore")
	chatPtr := resCmd.String("c", "", "Target channel or 'me'")
	resCmd.StringVar(chatPtr, "chat", "", "Target channel or 'me'")
	proxyPtr := resCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := resCmd.Bool("no-proxy", false, "Do not use system proxy")

	resCmd.Parse(args)

	if *filePtr == "" {
		fmt.Println("❌ Please provide a JSON file using -f <file>.")
		resCmd.Usage()
		return nil
	}

	cli, err := client.GetClient(appID, appHash, sessionPath, *noProxyPtr, *proxyPtr)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	return cli.Run(ctx, func(ctx context.Context) error {
		if err := client.CheckAuth(ctx, cli); err != nil {
			return err
		}
		api := client.NewAPI(cli)

		var targetChat *tg.InputChannel

		// "me" ဒါမှမဟုတ် ဗလာဖြစ်ရင် Saved Messages ကို သုံးမယ် (targetChat ကို nil ထားခဲ့မယ်)
		if *chatPtr == "" || strings.ToLower(*chatPtr) == "me" || *chatPtr == "self" {
			fmt.Println("📤 Restoring to Saved Messages...")
		} else {
			chatStr := strings.TrimPrefix(*chatPtr, "@")
			// Private link ဖြစ်ရင် channel ID ကို ဆွဲထုတ်မယ်
			if strings.Contains(chatStr, "t.me/c/") {
				parts := strings.Split(chatStr, "/")
				for i, p := range parts {
					if p == "c" && i+1 < len(parts) {
						chatStr = parts[i+1]
						break
					}
				}
			}

			// ID နဲ့ ရှာမယ် (-100 ဒါမှမဟုတ် 100 နဲ့ စရင်)
			if strings.HasPrefix(chatStr, "-100") || strings.HasPrefix(chatStr, "100") {
				var id int64
				if _, err := fmt.Sscanf(chatStr, "%d", &id); err == nil {
					// ID ကို မှန်ကန်တဲ့ Format ပြောင်းမယ်
					if id > 0 && id < 1000000000000 {
						id = int64(1000000000000) + id
					}
					ch, err := tgtg.FetchChannelByID(ctx, api, id)
					if err != nil {
						return fmt.Errorf("cannot resolve channel ID %d: %w", id, err)
					}
					targetChat = ch
				}
			} else {
				// Username နဲ့ ရှာမယ်
				ch, err := tgtg.ResolveUsername(ctx, api, chatStr)
				if err != nil {
					return fmt.Errorf("cannot resolve target chat '%s': %w", *chatPtr, err)
				}
				targetChat = ch
			}
		}

		// `backup.RestoreChat` ကို ခေါ်မယ် (targetChat က nil ဖြစ်ရင် Saved Messages ကို သွားမယ်)
		return backup.RestoreChat(ctx, api, targetChat, *filePtr)
	})
}