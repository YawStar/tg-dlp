package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/gotd/td/tg"
	"tg-dlp/pkg/runtime"
	tgtg "tg-dlp/pkg/telegram"
)

// StringSlice is a flag.Value that collects repeated string flags.
type StringSlice []string

func (s *StringSlice) String() string { return strings.Join(*s, ", ") }
func (s *StringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

// PrintVersion prints build metadata.
func PrintVersion() {
	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	reset := "\033[0m"

	fmt.Printf("%stg-dlp Version:%s %s%s%s\n", cyan, reset, green, runtime.Version, reset)
	fmt.Printf("%sCommit:%s         %s%s%s\n", cyan, reset, yellow, runtime.Commit, reset)
	fmt.Printf("%sBuild Time:%s     %s%s%s\n", cyan, reset, yellow, runtime.BuildTime, reset)
}

// PrintHelp prints the main help text.
func PrintHelp() {
	green := "\033[32m"
	cyan := "\033[36m"
	yellow := "\033[33m"
	reset := "\033[0m"

	fmt.Println(`
==================================================
  tg-dlp - YawStar Telegram Media Downloader
==================================================
Usage:
  tg-dlp <command> [options]

Commands:`)
	fmt.Printf("  %sdownload, dl%s    Download media from Telegram links\n", green, reset)
	fmt.Printf("  %supload, up%s      Upload files to Telegram\n", green, reset)
	fmt.Printf("  %sexport%s          Export channel messages to JSON\n", green, reset)
	fmt.Printf("  %sbackup%s          Compress exports into a ZIP file\n", green, reset)
	fmt.Printf("  %srestore%s         Restore messages from a JSON file\n", green, reset)
	fmt.Printf("  %sinfo%s            Show channel, post, or thread information\n", green, reset)
	fmt.Printf("  %swhoami%s          Show your own account information\n", green, reset)
	fmt.Printf("  %slogin%s           Login or re-login to your Telegram account\n", green, reset)
	fmt.Printf("  %slogout%s          Clear the cached session password\n", green, reset)
	fmt.Printf("  %sclear-cache%s     Clear ONLY the cached session password\n", green, reset)
	fmt.Printf("  %sdelete, rm%s       Delete a message using its URL\n", green, reset)
	fmt.Printf("  %spurge, clear%s     Delete all messages in a channel\n", green, reset)
	fmt.Printf("  %sversion%s         Show version information\n", green, reset)
	fmt.Printf("  %shelp%s            Show this help menu\n", green, reset)

	fmt.Println(`
Download Options:`)
	fmt.Printf("  %s-u, --url%s %sstring%s    Telegram link (Message or Channel)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-d, --dir%s %sstring%s    Output directory (Default: \"downloads\")\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-i, --filter%s %sstring%s Filter file types (e.g., mp4,mkv,mp3)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-t, --template%s %sstring%s  Filename template (e.g., \"{id}_{ext}\")\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s--takeout%s           Use Takeout Session to reduce rate limits\n", cyan, reset)
	fmt.Printf("  %s--auto-join%s         Automatically join private channels\n", cyan, reset)
	fmt.Printf("  %s--skip-same%s         Skip downloading if the file already exists\n", cyan, reset)
	fmt.Printf("  %s--overwrite%s         Overwrite existing files\n", cyan, reset)
	fmt.Printf("  %s--skip-flood%s        Skip waiting on FLOOD_WAIT\n", cyan, reset)
	fmt.Printf("  %s--dry-run%s           List what would be downloaded without downloading\n", cyan, reset)

	fmt.Println(`
Upload Options:`)
	fmt.Printf("  %s-p, --path%s %sstring%s    File or directory path to upload\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-c, --chat%s %sstring%s    Target chat (Default: Saved Messages)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-i, --include%s %sstring%s Only upload specific extensions\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-e, --exclude%s %sstring%s Exclude specific extensions\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s--photo%s              Force upload images as photos\n", cyan, reset)
	fmt.Printf("  %s--rm%s                 Delete local files after upload\n", cyan, reset)

	fmt.Println(`
Export Options:`)
	fmt.Printf("  %s-u, --url%s %sstring%s    Telegram channel link (Required)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-d, --dir%s %sstring%s    Output directory (Default: \"exports\")\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-l, --limit%s %sint%s      Maximum messages to export (Default: 0 = all)\n", cyan, reset, yellow, reset)

	fmt.Println(`
Backup Options:`)
	fmt.Printf("  %s-d, --dir%s %sstring%s    Directory to backup (Default: \"exports\")\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-o, --out%s %sstring%s    Output ZIP file name\n", cyan, reset, yellow, reset)

	fmt.Println(`
Restore Options:`)
	fmt.Printf("  %s-f, --file%s %sstring%s   JSON file to restore (Required)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-c, --chat%s %sstring%s   Target channel or \"me\" for Saved Messages\n", cyan, reset, yellow, reset)

	fmt.Println(`
Delete Options:`)
	fmt.Printf("  %s-u, --url%s %sstring%s    Telegram message link\n", cyan, reset, yellow, reset)

	fmt.Println(`
Purge Options:`)
	fmt.Printf("  %s-c, --chat%s %sstring%s    Target channel (Required)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-i, --include%s %sstring%s Only delete specific extensions\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s-e, --exclude%s %sstring%s Exclude specific extensions\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s--text%s               Delete text messages only\n", cyan, reset)
	fmt.Printf("  %s--media%s              Delete media messages only\n", cyan, reset)
	fmt.Printf("  %s-y, --yes%s            Skip confirmation prompt\n", cyan, reset)

	fmt.Println(`
Global Options (Proxy & Debug):`)
	fmt.Printf("  %s--proxy%s %sstring%s     Manual proxy URL (e.g., http://127.0.0.1:8080)\n", cyan, reset, yellow, reset)
	fmt.Printf("  %s--no-proxy%s          Do not use system proxy\n", cyan, reset)
	fmt.Printf("  %s--skip-flood%s        Skip waiting on FLOOD_WAIT (dl/up/purge/delete/info/whoami)\n", cyan, reset)
	fmt.Printf("  %s--debug, --verbose%s  Show detailed debug logs (all commands)\n", cyan, reset)

	fmt.Println(`
Examples:`)
	fmt.Printf("  tg-dlp %sdl%s %s-u%s https://t.me/channel/123\n", green, reset, cyan, reset)
	fmt.Printf("  tg-dlp %sdl%s %s-u%s https://t.me/channel/123 %s--proxy%s http://127.0.0.1:8080\n", green, reset, cyan, reset, cyan, reset)
	fmt.Printf("  tg-dlp %sdl%s %s-u%s https://t.me/channel/123 %s--dry-run%s\n", green, reset, cyan, reset, cyan, reset)
	fmt.Printf("  tg-dlp %sup%s %s-p%s ./video.mp4\n", green, reset, cyan, reset)
	fmt.Printf("  tg-dlp %sexport%s %s-u%s https://t.me/channel/123 -l 500\n", green, reset, cyan, reset)
	fmt.Printf("  tg-dlp %sdelete%s %s-u%s https://t.me/channel/123\n", green, reset, cyan, reset)
	fmt.Printf("  tg-dlp %spurge%s %s-c%s @mychannel %s-y%s\n", green, reset, cyan, reset, cyan, reset)
	fmt.Printf("  tg-dlp %swhoami%s\n", green, reset)
	fmt.Printf("  tg-dlp %slogout%s\n", green, reset)
	fmt.Printf("  tg-dlp %sversion%s\n", green, reset)
	fmt.Println("==================================================")
}

// filterSetToSlice converts a set back to a slice (order not important).
func filterSetToSlice(set map[string]bool) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// linkInfoTarget resolves a LinkInfo to *tg.InputChannel.
func linkInfoTarget(ctx context.Context, api *tg.Client, linkInfo *tgtg.LinkInfo) *tg.InputChannel {
	if linkInfo.IsUsername {
		if ch, err := tgtg.ResolveUsername(ctx, api, linkInfo.Username); err == nil {
			return ch
		}
		return nil
	}
	if ch, err := tgtg.FetchChannelByID(ctx, api, linkInfo.ChannelID); err == nil {
		return ch
	}
	return nil
}

// DebugFlag returns true if --debug or --verbose is set.
func DebugFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--debug" || arg == "-debug" || arg == "--verbose" || arg == "-verbose" {
			return true
		}
	}
	return false
}