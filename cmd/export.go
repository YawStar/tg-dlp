package cmd

import (
	"context"
	"flag"
	"fmt"

	"tg-dlp/app/backup"
	"tg-dlp/core/client"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// RunExportCmd implements the "export" command.
func RunExportCmd(args []string, appID int, appHash, sessionPath string) error {
	expCmd := flag.NewFlagSet("export", flag.ExitOnError)
	expCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp export [options]

Options:
  -u, --url string    Telegram channel link (Required)
  -d, --dir string    Output directory (Default: "exports")
  -l, --limit int     Maximum number of messages to export (Default: 0 = all)
  --proxy string      Manual proxy URL
  --no-proxy          Do not use system proxy
  --skip-flood        Skip waiting on FLOOD_WAIT
`)
	}

	urlPtr := expCmd.String("u", "", "Telegram channel link")
	expCmd.StringVar(urlPtr, "url", "", "Telegram channel link")
	dirPtr := expCmd.String("d", "exports", "Output directory")
	expCmd.StringVar(dirPtr, "dir", "exports", "Output directory")
	limitPtr := expCmd.Int("l", 0, "Maximum messages to export")
	proxyPtr := expCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := expCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := expCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")

	expCmd.Parse(args)

	// Debug Mode
	if DebugFlag(args) {
		runtime.DebugMode = true
		fmt.Println("🐛 Debug mode enabled")
	}

	// Set global flood wait flag
	runtime.SkipFlood = *skipFloodPtr

	if *urlPtr == "" {
		fmt.Println("❌ Please provide a channel URL using -u <url>.")
		expCmd.Usage()
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

		linkInfo, err := tgtg.ParseLink(*urlPtr)
		if err != nil {
			return fmt.Errorf("invalid link: %w", err)
		}

		targetChannel := linkInfoTarget(ctx, api, linkInfo)
		if targetChannel == nil {
			if linkInfo.IsUsername {
				return fmt.Errorf("cannot resolve public channel '@%s'. Make sure the username is correct", linkInfo.Username)
			}
			return fmt.Errorf("cannot resolve private channel ID %d. Make sure you have joined this channel and it exists", linkInfo.ChannelID)
		}

		return backup.ExportChat(ctx, api, targetChannel, *dirPtr, *limitPtr)
	})
}