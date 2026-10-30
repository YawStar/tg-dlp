package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/tg"

	"tg-dlp/core/client"
	"tg-dlp/core/session"
)

// RunLoginCmd implements "login" — forces a fresh session.
func RunLoginCmd(appID int, appHash, sessionPath string) error {
	fmt.Println("🔄 Deleting old session for new login...")
	os.Remove(sessionPath)
	session.ClearPasswordCache(sessionPath)

	cli, err := client.GetClient(appID, appHash, sessionPath, false, "")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() { <-sigChan; cancel() }()

	return cli.Run(ctx, func(ctx context.Context) error {
		fmt.Println("\n🔑 Login required.")
		var phone string
		fmt.Print("📞 Enter phone number (+959...): ")
		fmt.Scanln(&phone)

		flow := auth.NewFlow(
			auth.Constant(phone, "", auth.CodeAuthenticatorFunc(func(ctx context.Context, sentCode *tg.AuthSentCode) (string, error) {
				var code string
				fmt.Print("🔢 Enter code: ")
				fmt.Scanln(&code)
				return code, nil
			})),
			auth.SendCodeOptions{},
		)
		if err := flow.Run(ctx, cli.Auth()); err != nil {
			return err
		}
		fmt.Println("\n✅ Successfully logged in and encrypted session saved!")
		return nil
	})
}

// RunLogoutCmd clears the cached session password.
func RunLogoutCmd(sessionPath string) error {
	fmt.Println("🔄 Clearing cached session password...")
	session.ClearPasswordCache(sessionPath)
	fmt.Println("✅ Session password cache cleared.")
	fmt.Println("   You will need to enter the password again next time.")
	return nil
}

// RunClearCacheCmd clears only the cache (session file is untouched).
func RunClearCacheCmd(sessionPath string) error {
	fmt.Println("🔄 Clearing cached session password (Session file is untouched)...")
	session.ClearPasswordCache(sessionPath)
	fmt.Println("✅ Cache cleared successfully.")
	fmt.Println("   You will need to enter the password again next time.")
	return nil
}