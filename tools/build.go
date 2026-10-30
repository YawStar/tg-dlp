package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type Target struct {
	OS   string
	Arch string
	Ext  string
}

var targets = []Target{
	// 64-bit Targets
	{"windows", "amd64", ".exe"},
	{"windows", "arm64", ".exe"},
	{"linux", "amd64", ""},
	{"linux", "arm64", ""},
	{"darwin", "amd64", ""},
	{"darwin", "arm64", ""},

	// 32-bit Targets
	{"windows", "386", ".exe"},
	{"linux", "386", ""},
	{"linux", "arm", ""},
}

const (
	distDir  = "dist"
	appName  = "tg-dlp"
	mainFile = "main.go"
)

func main() {
	if err := build(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func build() error {
	// dist folder ဖန်တီး
	if err := os.MkdirAll(distDir, 0755); err != nil {
		return fmt.Errorf("dist folder ဖန်တီးမရပါ: %w", err)
	}

	// main.go ရှိ/မရှိ စစ်
	if _, err := os.Stat(mainFile); err != nil {
		return fmt.Errorf("'%s' not found — project root မှာ run ပါ", mainFile)
	}

	// Git commit, version, build time ကို auto ရယူ
	commit := getGitCommit()
	version := getVersion()
	buildTime := time.Now().UTC().Format("2006-01-02T15:04:05Z")

	fmt.Printf("Version:    %s\n", version)
	fmt.Printf("Commit:     %s\n", commit)
	fmt.Printf("Build Time: %s\n\n", buildTime)

	// ldflags — version info ကို pkg/runtime package ထဲ ထိုးသွင်း
	ldflags := fmt.Sprintf(
		"-s -w -X tg-dlp/pkg/runtime.Version=%s -X tg-dlp/pkg/runtime.Commit=%s -X tg-dlp/pkg/runtime.BuildTime=%s",
		version, commit, buildTime,
	)

	// Target တစ်ခုချင်း build
	for _, t := range targets {
		// ဖိုင်နာမည် ပုံစံသတ်မှတ်ခြင်း (Windows အတွက် x64/x86၊ Linux/Mac အတွက် amd64/i386)
		archName := t.Arch
		if t.OS == "windows" {
			if t.Arch == "amd64" {
				archName = "x64"
			} else if t.Arch == "386" {
				archName = "x86"
			}
		} else {
			if t.Arch == "amd64" {
				archName = "amd64"
			} else if t.Arch == "386" {
				archName = "i386"
			}
		}

		osName := t.OS
		if t.OS == "darwin" {
			osName = "macOS"
		}

		outputName := fmt.Sprintf("%s_%s_%s%s", appName, osName, archName, t.Ext)
		outputPath, err := filepath.Abs(filepath.Join(distDir, outputName))
		if err != nil {
			return err
		}

		fmt.Printf("Building for %s/%s...\n", t.OS, t.Arch)

		cmd := exec.Command("go", "build",
			"-ldflags", ldflags,
			"-o", outputPath,
			".",
		)
		cmd.Env = append(os.Environ(),
			"GOOS="+t.OS,
			"GOARCH="+t.Arch,
			"CGO_ENABLED=0",
		)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr

		if err := cmd.Run(); err != nil {
			return fmt.Errorf("building for %s/%s failed: %w", t.OS, t.Arch, err)
		}

		fmt.Printf("  ✅ %s\n", outputName)
	}

	fmt.Printf("\nBuild complete! All binaries are saved in the '%s' folder.\n", distDir)

	return nil
}

// Git commit hash ကို ရယူ (မရရင် "unknown")
func getGitCommit() string {
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// Git tag/version ကို ရယူ (မရရင် "dev")
func getVersion() string {
	cmd := exec.Command("git", "describe", "--tags", "--always")
	out, err := cmd.Output()
	if err != nil {
		return "dev"
	}
	return strings.TrimSpace(string(out))
}