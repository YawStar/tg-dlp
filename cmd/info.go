package cmd

import (
	"context"
	"flag"
	"fmt"

	"tg-dlp/app/info"
	"tg-dlp/core/client"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// RunInfoCmd implements "info".
func RunInfoCmd(args []string, appID int, appHash, sessionPath string) error {
	infoCmd := flag.NewFlagSet("info", flag.ExitOnError)
	var links StringSlice
	infoCmd.Var(&links, "u", "Telegram link")
	infoCmd.Var(&links, "url", "Telegram link")
	savePtr := infoCmd.Bool("save", false, "Save info to file")
	proxyPtr := infoCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := infoCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := infoCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")

	infoCmd.Parse(args)

	runtime.SkipFlood = *skipFloodPtr

	if len(links) == 0 {
		fmt.Println("❌ No links provided. Use -u <link> to view info.")
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

		for _, link := range links {
			linkInfo, err := tgtg.ParseLink(link)
			if err != nil {
				fmt.Printf("❌ Failed to parse link: %v\n", err)
				continue
			}
			targetChannel := linkInfoTarget(ctx, api, linkInfo)
			if targetChannel == nil {
				fmt.Printf("❌ Could not resolve channel for %s\n", link)
				continue
			}

			switch {
			case linkInfo.ThreadID > 0:
				err = info.ShowThreadInfo(ctx, api, targetChannel, linkInfo.ThreadID, true, *savePtr, "downloads")
			case linkInfo.IsChannelOnly:
				err = info.ShowChannelInfo(ctx, api, targetChannel, true, *savePtr, "downloads")
			default:
				err = info.ShowPostInfo(ctx, api, targetChannel, linkInfo.MessageID, true, *savePtr, "downloads")
			}
			if err != nil {
				fmt.Printf("❌ Error: %v\n", err)
			}
		}
		return nil
	})
}