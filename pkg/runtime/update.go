package runtime

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const GitHubRepo = "YawStar/tg-dlp"

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	HTMLURL string `json:"html_url"`
	Assets  []struct {
		Name               string `json:"name"`
		BrowserDownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

// compareVersions က Version နှစ်ခုကို နှိုင်းယှဉ်ပေးပါတယ်
func compareVersions(latest, current string) int {
	lParts := strings.Split(latest, ".")
	cParts := strings.Split(current, ".")
	maxLen := len(lParts)
	if len(cParts) > maxLen {
		maxLen = len(cParts)
	}
	for i := 0; i < maxLen; i++ {
		var lNum, cNum int
		if i < len(lParts) {
			lNum, _ = strconv.Atoi(lParts[i])
		}
		if i < len(cParts) {
			cNum, _ = strconv.Atoi(cParts[i])
		}
		if lNum > cNum {
			return 1
		}
		if lNum < cNum {
			return -1
		}
	}
	return 0
}

// CheckForUpdates က GitHub မှာ Version အသစ် ရှိမရှိ စစ်ဆေးပေးပါတယ်
func CheckForUpdates() {
	currentVersion := Version
	if currentVersion == "" || currentVersion == "dev" {
		return
	}

	client := http.Client{Timeout: 15 * time.Second}
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", GitHubRepo)
	resp, err := client.Get(url)
	if err != nil {
		fmt.Println("⚠ Could not check for updates (Network error).")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		fmt.Println("⚠ Could not check for updates (No release found).")
		return
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return
	}

	latestVer := strings.TrimPrefix(release.TagName, "v")
	currentVer := strings.TrimPrefix(currentVersion, "v")

	if compareVersions(latestVer, currentVer) == 1 {
		yellow := "\033[33m"
		green := "\033[32m"
		reset := "\033[0m"

		fmt.Printf("\n%s==================================================%s\n", yellow, reset)
		fmt.Printf("%s✨ Update Available!%s\n", green, reset)
		fmt.Printf("   Current: %s%s%s\n", yellow, currentVer, reset)
		fmt.Printf("   Latest:  %s%s%s\n", green, latestVer, reset)
		fmt.Printf("   Download: %s\n", release.HTMLURL)
		fmt.Printf("%s==================================================%s\n\n", yellow, reset)

		fmt.Print("Do you want to download and install the update? [y/N]: ")
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.ToLower(strings.TrimSpace(input))

		if input == "y" || input == "yes" {
			downloadUpdate(release)
		} else {
			fmt.Println("Update skipped. You can update later using --update.")
		}
	} else {
		fmt.Printf("\n✅ You are using the latest version (%s).\n\n", currentVer)
	}
}

// downloadUpdate က Update ကို ဒေါင်းပြီး အစားထိုးပေးပါတယ်
func downloadUpdate(release GitHubRelease) {
	osName := runtime.GOOS
	arch := runtime.GOARCH

	var downloadURL string
	var expectedName string

	// ⭐ Goreleaser ရဲ့ ဖိုင်နာမည်ပုံစံအတိုင်း ရှာမယ် ⭐
	// Windows
	if osName == "windows" {
		if arch == "amd64" {
			expectedName = "windows_x64.zip"
		} else if arch == "386" {
			expectedName = "windows_x86.zip"
		} else if arch == "arm64" {
			expectedName = "windows_arm64.zip"
		}
	} else if osName == "linux" {
		// Linux
		if arch == "amd64" {
			expectedName = "linux_amd64.tar.gz"
		} else if arch == "arm64" {
			expectedName = "linux_arm64.tar.gz"
		} else if arch == "arm" {
			// ARM (armv5, armv6, armv7) အတွက် - GOARM ကို ဖတ်ပြီး ရှာမယ်
			goarm := os.Getenv("GOARM")
			if goarm != "" {
				expectedName = "linux_armv" + goarm + ".tar.gz"
			} else {
				// GOARM မရှိရင် default အနေနဲ့ armv7 လို့ ယူဆမယ်
				expectedName = "linux_armv7.tar.gz"
			}
		}
	} else if osName == "darwin" {
		// macOS
		if arch == "amd64" {
			expectedName = "macos_amd64.tar.gz"
		} else if arch == "arm64" {
			expectedName = "macos_arm64.tar.gz"
		}
	}

	for _, asset := range release.Assets {
		if strings.Contains(strings.ToLower(asset.Name), expectedName) {
			downloadURL = asset.BrowserDownloadURL
			break
		}
	}

	if downloadURL == "" {
		fmt.Printf("❌ Could not find a suitable archive for your system (%s/%s) in the latest release.\n", osName, arch)
		return
	}

	fmt.Printf("📥 Downloading %s...\n", expectedName)

	client := http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		fmt.Printf("❌ Download failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	exePath, err := os.Executable()
	if err != nil {
		fmt.Printf("❌ Could not find current executable path: %v\n", err)
		return
	}

	// Archive ဖိုင်ကို ခေတ္တသိမ်းမယ်
	var archivePath string
	if strings.HasSuffix(expectedName, ".zip") {
		archivePath = exePath + ".zip"
	} else {
		archivePath = exePath + ".tar.gz"
	}

	out, err := os.Create(archivePath)
	if err != nil {
		fmt.Printf("❌ Could not create archive file: %v\n", err)
		return
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		fmt.Printf("❌ Failed to save archive: %v\n", err)
		return
	}

	fmt.Println("📦 Extracting update...")
	newExePath := exePath + ".new"

	// ⭐ OS အလိုက် ဖြည့်တဲ့ နည်းလမ်း ကွဲပြားမယ် ⭐
	if strings.HasSuffix(archivePath, ".zip") {
		err = extractZip(archivePath, newExePath)
	} else if strings.HasSuffix(archivePath, ".tar.gz") {
		err = extractTarGz(archivePath, newExePath)
	} else {
		err = fmt.Errorf("unsupported archive format")
	}

	if err != nil {
		fmt.Printf("❌ Extraction failed: %v\n", err)
		os.Remove(archivePath)
		return
	}
	os.Remove(archivePath)

	fmt.Println("✅ Update downloaded and extracted!")

	// ⭐ ဖိုင်အစားထိုးခြင်း ⭐
	if osName == "windows" {
		oldExePath := exePath + ".old"
		
		err := os.Rename(exePath, oldExePath)
		if err != nil {
			fmt.Printf("❌ Could not rename current executable: %v\n", err)
			fmt.Println("   Please close other instances of tg-dlp or run as Administrator.")
			os.Remove(newExePath) 
			return
		}

		err = os.Rename(newExePath, exePath)
		if err != nil {
			fmt.Printf("❌ Could not rename new executable: %v\n", err)
			os.Rename(oldExePath, exePath) 
			return
		}

		green := "\033[32m"
		reset := "\033[0m"
		fmt.Printf("\n%s✅ Update applied successfully!%s\n", green, reset)
		fmt.Println("   Please run your command again.")
		
		os.Exit(0)

	} else {
		// Linux / macOS အတွက် (run နေတဲ့ဖိုင်ကို တိုက်ရိုက် အစားထိုးလို့ရတယ်)
		err = os.Rename(newExePath, exePath)
		if err != nil {
			fmt.Printf("❌ Could not replace executable: %v\n", err)
			fmt.Println("   Try running with sudo or manually replace the file.")
		} else {
			os.Chmod(exePath, 0755)
			fmt.Println("✅ Update applied successfully! Please restart the program.")
		}
	}
}

// extractZip က ZIP ဖိုင်ထဲက .exe ကို ဖြည့်ထုတ်ပေးပါတယ်
func extractZip(zipPath, dest string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	targetExeName := "tg-dlp.exe"
	if runtime.GOOS != "windows" {
		targetExeName = "tg-dlp"
	}

	// တိကျတဲ့ နာမည်ကို ရှာမယ်
	for _, f := range r.File {
		if f.Name == targetExeName || strings.HasSuffix(f.Name, "/"+targetExeName) {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			return err
		}
	}

	// မတွေ့ဘူးဆိုရင် .exe နဲ့ ဆုံးတဲ့ ပထမဆုံးဖိုင်ကို ယူမယ်
	for _, f := range r.File {
		if strings.HasSuffix(strings.ToLower(f.Name), ".exe") && runtime.GOOS == "windows" {
			rc, err := f.Open()
			if err != nil {
				return err
			}
			defer rc.Close()

			outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, rc)
			return err
		}
	}
	return fmt.Errorf("no executable found inside the zip")
}

// extractTarGz က .tar.gz ဖိုင်ထဲက executable ကို ဖြည့်ထုတ်ပေးပါတယ် (Linux/macOS)
func extractTarGz(tarGzPath, dest string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()

	gzr, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gzr.Close()

	tr := tar.NewReader(gzr)

	targetExeName := "tg-dlp"

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		if header.Name == targetExeName || strings.HasSuffix(header.Name, "/"+targetExeName) {
			outFile, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, os.FileMode(header.Mode))
			if err != nil {
				return err
			}
			defer outFile.Close()

			_, err = io.Copy(outFile, tr)
			return err
		}
	}
	return fmt.Errorf("no executable found inside the tar.gz")
}
