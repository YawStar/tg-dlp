package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"tg-dlp/core/session"
	"tg-dlp/pkg/runtime"
)

// Default API credentials (replace with your own).
const (
	DefaultAPIID   = 0
	DefaultAPIHash = ""
)

// Run is the main dispatch. Call from main().
func Run() {
	// Global Debug Check
	for _, arg := range os.Args {
		if arg == "--debug" || arg == "-debug" || arg == "--verbose" || arg == "-verbose" {
			runtime.DebugMode = true
			fmt.Println("🐛 Debug mode enabled")
			break
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("❌ Cannot find user home directory:", err)
		return
	}
	configDir := filepath.Join(home, ".tg-dlp")
	os.MkdirAll(configDir, 0700)

	configPath := filepath.Join(configDir, "config.enc")
	sessionPath := filepath.Join(configDir, "session.enc")

	if len(os.Args) < 2 {
		PrintHelp()
		return
	}
	command := strings.ToLower(os.Args[1])

	switch command {
	case "help", "-h", "--help":
		PrintHelp()
		return
	case "version", "-v", "--version":
		PrintVersion()
		return
	case "logout":
		if err := RunLogoutCmd(sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		return
	case "clear-cache", "clearcache":
		if err := RunClearCacheCmd(sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		return
	}

	appID, appHash := loadConfig(configPath)

	switch command {
	case "download", "dl":
		if err := RunDownloadCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "upload", "up":
		if err := RunUploadCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "delete", "del", "rm":
		if err := RunDeleteCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "export":
		if err := RunExportCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "backup":
		if err := RunBackupCmd(os.Args[2:]); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "restore":
		if err := RunRestoreCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "purge", "clear":
		if err := RunPurgeCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "info":
		if err := RunInfoCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "whoami":
		if err := RunWhoamiCmd(os.Args[2:], appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	case "login":
		if err := RunLoginCmd(appID, appHash, sessionPath); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	default:
		fmt.Printf("❌ Unknown command: '%s'\n\n", command)
		PrintHelp()
	}
}

// loadConfig reads/creates the encrypted API credentials file.
func loadConfig(configPath string) (int, string) {
	appID := DefaultAPIID
	appHash := DefaultAPIHash
	const configPassword = "tg-dlp"

	cyan := "\033[36m"
	green := "\033[32m"
	yellow := "\033[33m"
	red := "\033[31m"
	reset := "\033[0m"

	if data, err := os.ReadFile(configPath); err == nil {
		decData, err := session.DecryptData(data, configPassword)
		if err != nil {
			fmt.Printf("\n%s❌ Invalid config password or corrupted file.%s\n", red, reset)
			os.Exit(1)
		}
		parts := strings.Split(string(decData), "|")
		if len(parts) == 2 {
			appID, _ = strconv.Atoi(parts[0])
			appHash = parts[1]
		}
		return appID, appHash
	}

	fmt.Printf("\n%s⚠️  Telegram API credentials missing.%s\n", yellow, reset)
	fmt.Printf("%s   (Get them from https://my.telegram.org)%s\n\n", cyan, reset)
	reader := bufio.NewReader(os.Stdin)

	fmt.Printf("%s🆔 Enter API ID:%s ", cyan, reset)
	idStr, _ := reader.ReadString('\n')
	idStr = strings.TrimSpace(idStr)
	id, err := strconv.Atoi(idStr)
	if err != nil {
		fmt.Printf("%s❌ API ID must be a number.%s\n", red, reset)
		os.Exit(1)
	}
	appID = id

	fmt.Printf("%s🔑 Enter API Hash:%s ", cyan, reset)
	hashStr, _ := reader.ReadString('\n')
	appHash = strings.TrimSpace(hashStr)

	encData, err := session.EncryptData(fmt.Appendf(nil, "%d|%s", appID, appHash), configPassword)
	if err == nil {
		_ = os.WriteFile(configPath, encData, 0600)
		fmt.Printf("\n%s✅ Encrypted config saved to %s%s\n", green, configPath, reset)
	}
	return appID, appHash
}