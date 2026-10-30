package cmd

import (
	"context"
	"flag"

	"tg-dlp/app/info"
	"tg-dlp/core/client"
	"tg-dlp/pkg/runtime"
)

// RunWhoamiCmd implements "whoami".
func RunWhoamiCmd(args []string, appID int, appHash, sessionPath string) error {
	whoamiCmd := flag.NewFlagSet("whoami", flag.ExitOnError)
	savePtr := whoamiCmd.Bool("save", false, "Save your account information to a text file")
	proxyPtr := whoamiCmd.String("proxy", "", "Manual proxy URL")
	noProxyPtr := whoamiCmd.Bool("no-proxy", false, "Do not use system proxy")
	skipFloodPtr := whoamiCmd.Bool("skip-flood", false, "Skip waiting on FLOOD_WAIT")
	whoamiCmd.Parse(args)

	runtime.SkipFlood = *skipFloodPtr

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
		return info.ShowSelfInfo(ctx, client.NewAPI(cli), true, *savePtr, "downloads")
	})
}