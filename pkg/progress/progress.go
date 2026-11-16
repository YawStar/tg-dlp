package progress

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/term"
)

type Writer struct {
	file        *os.File
	total       int64
	current     int64
	start       time.Time
	mu          sync.Mutex
	last        time.Time
	lastCurrent int64
	lastSpeed   float64
	lastETA     string
}

func New(file *os.File, total, current int64) *Writer {
	now := time.Now()
	return &Writer{
		file:        file,
		total:       total,
		current:     current,
		start:       now,
		last:        now,
		lastCurrent: current,
	}
}

func (w *Writer) Current() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

func (w *Writer) Total() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.total
}

func (w *Writer) File() *os.File {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file
}

// Helper to truncate long filenames for display
func truncateFilename(name string, maxLen int) string {
	if len(name) <= maxLen {
		return name
	}
	if maxLen < 3 {
		return name[:maxLen]
	}
	return name[:maxLen-3] + "..."
}

// ⭐ WriteAt ကို implement လုပ်ထားပါတယ် (Parallel Download အတွက်) ⭐
func (w *Writer) WriteAt(p []byte, off int64) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// ဖိုင်အရွယ်အစား ကျော်သွားရင် ဆက်မရေးတော့ဘူး
	if w.total > 0 && off >= w.total {
		return len(p), nil
	}

	// ဖိုင်အရွယ်အစား ကျော်မသွားအောင် ဖြတ်တောက်မယ်
	writeLen := len(p)
	if w.total > 0 {
		remaining := w.total - off
		if int64(writeLen) > remaining {
			writeLen = int(remaining)
		}
	}

	n, err := w.file.WriteAt(p[:writeLen], off)
	if err != nil {
		return n, err
	}

	w.current += int64(n)
	now := time.Now()

	// Progress Bar ကို 100ms တစ်ခါ Update လုပ်မယ်
	if now.Sub(w.last) >= 100*time.Millisecond || (w.total > 0 && w.current >= w.total) {
		if now.Sub(w.last) > 2*time.Second {
			w.lastSpeed = 0
			w.lastETA = "--"
		}

		elapsed := now.Sub(w.last).Seconds()
		deltaBytes := w.current - w.lastCurrent
		if elapsed > 0 {
			currentSpeed := float64(deltaBytes) / elapsed
			if currentSpeed > 0 {
				w.lastSpeed = currentSpeed
			}
		}

		if now.Sub(w.start) >= 1*time.Second {
			if w.total > 0 && w.lastSpeed > 0 {
				etaSeconds := int(float64(w.total-w.current) / w.lastSpeed)
				minutes := etaSeconds / 60
				seconds := etaSeconds % 60
				if minutes > 0 {
					w.lastETA = fmt.Sprintf("%dm %ds", minutes, seconds)
				} else {
					w.lastETA = fmt.Sprintf("%ds", seconds)
				}
			} else {
				w.lastETA = "--"
			}
			w.start = now
		}

		w.last = now
		w.lastCurrent = w.current

		var percent float64
		if w.total > 0 {
			percent = float64(w.current) / float64(w.total) * 100
			if percent > 100 {
				percent = 100
			}
		}

		barLength := 20
		filledLength := int(percent / 100 * float64(barLength))
		bar := ""
		for i := 0; i < barLength; i++ {
			if i < filledLength {
				bar += "\033[32m#\033[0m"
			} else {
				bar += "-"
			}
		}

		displayName := truncateFilename(filepath.Base(w.file.Name()), 25)

		var line string
		if w.total > 0 {
			// အရောင်များ ခွဲခြားသတ်မှတ်ထားခြင်း (ANSI Escape Codes)
			// Percentage: အစိမ်းရောင် (\033[32m)
			// File Size: အပြာနုရောင် (\033[36m)
			// Speed: ခရမ်းရောင်/ပန်းရောင် (\033[35m)
			// ETA: အဝါရောင် (\033[33m)
			percentStr := fmt.Sprintf("\033[32m%.1f%%\033[0m", percent)
			sizeStr := fmt.Sprintf("\033[36m%s / %s\033[0m", FormatSize(w.current), FormatSize(w.total))
			speedStr := fmt.Sprintf("\033[35m%s/s\033[0m", FormatSize(int64(w.lastSpeed)))
			etaStr := fmt.Sprintf("\033[33m%s\033[0m", w.lastETA)

			line = fmt.Sprintf("[%s] %s %s | %s | Speed: %s | ETA: %s",
				displayName, bar, percentStr, sizeStr, speedStr, etaStr)
		} else {
			speedStr := fmt.Sprintf("\033[35m%s/s\033[0m", FormatSize(int64(w.lastSpeed)))
			line = fmt.Sprintf("[%s] Downloading: \033[36m%s\033[0m | Speed: %s",
				displayName, FormatSize(w.current), speedStr)
		}

		width, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || width <= 0 {
			width = 80
		}

		fmt.Printf("\033[2K\r%-*s", width-1, line)
		_ = os.Stdout.Sync()
	}

	return n, nil
}

func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}
