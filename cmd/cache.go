package cmd

import (
	"fmt"
	"time"

	"tg-dlp/core/session"
)

// RunSetCacheCmd implements "set-cache" or "config-cache".
func RunSetCacheCmd(sessionPath string) error {
	fmt.Println("\n⏱  How long should I remember the session password?")
	fmt.Println("   1. 30 mins (Default)")
	fmt.Println("   2. 1 hour")
	fmt.Println("   3. 2 hours")
	fmt.Println("   4. 3 hours")
	fmt.Println("   5. Once (Do not cache)")
	fmt.Println("   0. Until session expires (Unexpired)")
	fmt.Print("\n👉 Choose an option: ")

	var choice string
	fmt.Scanln(&choice)

	var duration time.Duration
	switch choice {
	case "1", "":
		duration = 30 * time.Minute
	case "2":
		duration = 1 * time.Hour
	case "3":
		duration = 2 * time.Hour
	case "4":
		duration = 3 * time.Hour
	case "5":
		duration = 0
	case "0":
		duration = 100 * 365 * 24 * time.Hour
	default:
		fmt.Println("⚠ Invalid choice. Using default (30 mins).")
		duration = 30 * time.Minute
	}

	if duration > 0 {
		password := session.PromptPassword("🔑 Enter session password to apply new cache setting: ")
		storage := &session.EncryptedSessionStorage{Path: sessionPath, Password: password}
		if _, err := storage.LoadSession(nil); err != nil {
			return fmt.Errorf("invalid password")
		}
		
		session.SavePasswordToCache(sessionPath, password, duration)
		if duration == 100*365*24*time.Hour {
			fmt.Println("✅ Password will be remembered until the session file is deleted.")
		} else {
			fmt.Printf("✅ Password will be remembered for %v.\n", duration)
		}
	} else {
		session.ClearPasswordCache(sessionPath)
		fmt.Println("✅ Password will not be cached.")
	}
	return nil
}
