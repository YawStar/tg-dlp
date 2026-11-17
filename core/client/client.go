package client

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/tg"
	"golang.org/x/net/proxy"

	"tg-dlp/core/session"
	"tg-dlp/pkg/runtime"
)

// getSystemProxy returns the first non-empty proxy env var.
func getSystemProxy() string {
	vars := []string{"HTTPS_PROXY", "https_proxy", "HTTP_PROXY", "http_proxy", "ALL_PROXY", "all_proxy"}
	for _, v := range vars {
		if val := os.Getenv(v); val != "" {
			return val
		}
	}
	return ""
}

// askCacheDuration က Password ကို ဘယ်လောက်ကြာကြာ မှတ်ထားမလဲဆိုတာကို မေးပါတယ်။
func askCacheDuration() time.Duration {
	fmt.Println("\n⏱  How long should I remember this password?")
	fmt.Println("   1. 30 mins (Default)")
	fmt.Println("   2. 1 hour")
	fmt.Println("   3. 2 hours")
	fmt.Println("   4. 3 hours")
	fmt.Println("   5. Once (Do not cache)")
	fmt.Println("   0. Until session expires (Unexpired)")
	fmt.Print("\n👉 Choose an option: ")

	var choice string
	fmt.Scanln(&choice)

	switch choice {
	case "1", "":
		return 30 * time.Minute
	case "2":
		return 1 * time.Hour
	case "3":
		return 2 * time.Hour
	case "4":
		return 3 * time.Hour
	case "5":
		return 0 // Cache မလုပ်ဘူး
	case "0":
		return 100 * 365 * 24 * time.Hour // Unexpired အတွက် နှစ် 100 လောက် သတ်မှတ်မယ်
	default:
		fmt.Println("⚠ Invalid choice. Using default (30 mins).")
		return 30 * time.Minute
	}
}

// GetClient builds a telegram.Client with encrypted session storage.
func GetClient(appID int, appHash, sessionPath string, noProxy bool, proxyStr string) (*telegram.Client, error) {
	_, err := os.Stat(sessionPath)
	isNew := os.IsNotExist(err)

	var password string
	needPasswordPrompt := false

	if isNew {
		session.ClearPasswordCache(sessionPath)
		fmt.Print("\n🔐 No secure session found. Create a new one please.\n")
		fmt.Print("⚠️  Note: This is NOT your Telegram account password.\n")
		fmt.Print("     It is a new password to protect your local session file.\n\n")

		for {
			pass1 := session.PromptPassword("🔑 Set a new password (Min 4 characters): ")
			pass2 := session.PromptPassword("🔁 Confirm password: ")
			if pass1 != pass2 {
				fmt.Println("❌ Passwords do not match. Try again.")
				continue
			}
			if len(pass1) < 4 {
				fmt.Println("❌ Password must be at least 4 characters.")
				continue
			}
			password = pass1
			fmt.Println("✅ Session password set successfully.")
			break
		}

		duration := askCacheDuration()
		if duration > 0 {
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

	} else {
		cachedPass, found := session.LoadPasswordFromCache(sessionPath)
		if found {
			password = cachedPass
			storage := &session.EncryptedSessionStorage{Path: sessionPath, Password: password}
			if _, err := storage.LoadSession(context.Background()); err != nil {
				session.ClearPasswordCache(sessionPath)
				fmt.Print("⚠️  Cached password is invalid. Please enter manually.\n\n")
				needPasswordPrompt = true
			} else {
				runtime.DebugLog("🔓 Using cached password.")
			}
		} else {
			needPasswordPrompt = true
		}

		if needPasswordPrompt {
			// ⭐ Password ကို အရင်ရိုက်ခိုင်းမယ် ⭐
			password = session.PromptPassword("\n🔑 Enter session file password: ")
			
			// ⭐ Password မှန်မမှန် အရင်စစ်မယ် ⭐
			storage := &session.EncryptedSessionStorage{Path: sessionPath, Password: password}
			if _, err := storage.LoadSession(context.Background()); err != nil {
				session.ClearPasswordCache(sessionPath)
				return nil, errors.New("invalid password")
			}

			// ⭐ မှန်မှသာ မေးခွန်းမေးပြီး Cache သိမ်းမယ် ⭐
			duration := askCacheDuration()
			if duration > 0 {
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
		}
	}

	storage := &session.EncryptedSessionStorage{Path: sessionPath, Password: password}

	if !isNew {
		if _, err := storage.LoadSession(context.Background()); err != nil {
			if strings.Contains(err.Error(), "incorrect password") {
				session.ClearPasswordCache(sessionPath)
				return nil, errors.New("invalid password")
			}
			return nil, err
		}
	}

	opts := telegram.Options{SessionStorage: storage}

	var finalProxyStr string
	if !noProxy {
		if proxyStr != "" {
			finalProxyStr = proxyStr
		} else {
			finalProxyStr = getSystemProxy()
		}

		if finalProxyStr != "" {
			proxyURL, err := url.Parse(finalProxyStr)
			if err == nil {
				fmt.Printf("🌐 Using proxy: %s\n", finalProxyStr)
				if strings.HasPrefix(proxyURL.Scheme, "socks5") {
					dialer, err := proxy.SOCKS5("tcp", proxyURL.Host, nil, proxy.Direct)
					if err == nil {
						dc := dialer.(proxy.ContextDialer)
						opts.Resolver = dcs.Plain(dcs.PlainOptions{Dial: dc.DialContext})
					}
				} else {
					fmt.Printf("⚠ HTTP proxy is not fully supported by gotd/td. Trying anyway...\n")
				}
			}
		}
	}

	return telegram.NewClient(appID, appHash, opts), nil
}

// CheckAuth ensures the client is authorized, prompting for login if needed.
func CheckAuth(ctx context.Context, client *telegram.Client) error {
	status, err := client.Auth().Status(ctx)
	if err != nil {
		return err
	}
	if !status.Authorized {
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
		if err := flow.Run(ctx, client.Auth()); err != nil {
			return err
		}
	}
	runtime.DebugLog("✅ Connected to Telegram successfully!")
	return nil
}

// NewAPI wraps telegram.Client into tg.Client for direct API calls.
func NewAPI(cli *telegram.Client) *tg.Client {
	return tg.NewClient(cli)
}
