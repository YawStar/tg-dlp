package cmd

import (
	"flag"
	"fmt"
	"os"
	"time"

	"tg-dlp/app/backup"
)

// RunBackupCmd implements the "backup" command.
func RunBackupCmd(args []string) error {
	bakCmd := flag.NewFlagSet("backup", flag.ExitOnError)
	bakCmd.Usage = func() {
		fmt.Print(`
Usage: tg-dlp backup [options]

Options:
  -d, --dir string    Directory to backup (Default: "exports")
  -o, --out string    Output zip file name (Default: backup_YYYYMMDD.zip)
`)
	}

	dirPtr := bakCmd.String("d", "exports", "Directory to backup")
	outPtr := bakCmd.String("o", "", "Output zip file name")

	bakCmd.Parse(args)

	if _, err := os.Stat(*dirPtr); os.IsNotExist(err) {
		fmt.Printf("❌ Directory '%s' does not exist.\n", *dirPtr)
		return nil
	}

	outName := *outPtr
	if outName == "" {
		outName = fmt.Sprintf("backup_%s.zip", time.Now().Format("20060102_150405"))
	}

	if err := backup.ZipDirectory(*dirPtr, outName); err != nil {
		return err
	}

	fmt.Printf("\n✅ Backup saved to %s\n", outName)
	return nil
}