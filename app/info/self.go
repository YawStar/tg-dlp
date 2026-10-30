package info

import (
	"context"
	"fmt"

	"github.com/gotd/td/tg"
)

// ShowSelfInfo prints (and optionally saves) the current user's info.
func ShowSelfInfo(ctx context.Context, api *tg.Client, showInfo, saveInfo bool, outputDir string) error {
	out, file, fileName, err := setupOutput(saveInfo, showInfo, outputDir, "my_account_info.txt")
	if err != nil {
		return err
	}
	if file != nil {
		defer file.Close()
	}

	if showInfo {
		fmt.Println("🔍 Fetching your account information...")
	}

	res, err := api.UsersGetFullUser(ctx, &tg.InputUserSelf{})
	if err != nil {
		return fmt.Errorf("error fetching self info: %w", err)
	}
	if len(res.Users) == 0 {
		return fmt.Errorf("no user info returned")
	}
	user, ok := res.Users[0].(*tg.User)
	if !ok {
		return fmt.Errorf("could not parse user info")
	}
	fullUser := res.FullUser

	fmt.Fprintln(out, "==================================================")
	fmt.Fprintln(out, "👤 Your Account Information")
	fmt.Fprintln(out, "==================================================")
	fmt.Fprintf(out, "📛 Name: %s %s\n", user.FirstName, user.LastName)
	fmt.Fprintf(out, "🆔 User ID: %d\n", user.ID)
	if user.Username != "" {
		fmt.Fprintf(out, "🔗 Username: @%s\n", user.Username)
	}
	if user.Phone != "" {
		fmt.Fprintf(out, "📞 Phone: +%s\n", user.Phone)
	}
	if fullUser.About != "" {
		fmt.Fprintf(out, "📝 Bio: %s\n", fullUser.About)
	}
	if fullUser.Birthday.Day != 0 || fullUser.Birthday.Month != 0 {
		fmt.Fprintf(out, "🎂 Birthday: %d/%d\n", fullUser.Birthday.Day, fullUser.Birthday.Month)
		if fullUser.Birthday.Year != 0 {
			fmt.Fprintf(out, "   Year: %d\n", fullUser.Birthday.Year)
		}
	}
	fmt.Fprintln(out, "==================================================")

	if saveInfo {
		fmt.Printf("\n✅ Self info saved to %s\n", fileName)
	}
	return nil
}