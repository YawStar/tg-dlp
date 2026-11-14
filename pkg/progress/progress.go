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

// ⭐ downloader.go က ခေါ်သုံးဖို့ လက်ရှိ ပမာဏကို ပြန်ပေးမယ် ⭐
func (w *Writer) Current() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.current
}

// ⭐ downloader.go က ခေါ်သုံးဖို့ စုစုပေါင်း ပမာဏကို ပြန်ပေးမယ် ⭐
func (w *Writer) Total() int64 {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.total
}

// ⭐ downloader.go က ဖိုင်အခြေအနေကို စစ်ဆေးဖို့ ⭐
func (w *Writer) File() *os.File {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file
}

func (w *Writer) Write(p []byte) (int, error) {
	w.mu.Lock()
	// ⭐ ဖိုင်အရွယ်အစား ပြည့်သွားပြီလား အရင်စစ်မယ် ⭐
	if w.total > 0 && w.current >= w.total {
		w.mu.Unlock()
		return len(p), nil // ပြည့်သွားရင် ဘာမှ မရေးတော့ဘူး
	}

	// ⭐ ကျန်တဲ့ နေရာကို တွက်မယ် ⭐
	writeLen := len(p)
	if w.total > 0 {
		remaining := w.total - w.current
		if int64(writeLen) > remaining {
			writeLen = int(remaining) // ကျန်တဲ့ နေရာအထိပဲ ရေးမယ်
		}
	}

	// ⭐ တွက်ထားတဲ့ အရှည်အတိုင်းပဲ ရေးမယ် ⭐
	n, err := w.file.Write(p[:writeLen])
	w.mu.Unlock()

	if err != nil {
		return n, err
	}

	w.mu.Lock()
	w.current += int64(n)
	now := time.Now()

	// Progress Bar ကို 100ms တစ်ခါ Update လုပ်မယ်
	if now.Sub(w.last) >= 100*time.Millisecond || (w.total > 0 && w.current >= w.total) {
		
		// ⭐ ၂ စက္ကန့်ထက် ပိုကြာသွားရင် (Data မဝင်တော့ရင်) Speed ကို 0 ပြမယ် ⭐
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

		var line string
		if w.total > 0 {
			line = fmt.Sprintf("[%s] %s %.1f%% | %s / %s | Speed: %s/s | ETA: %s",
				filepath.Base(w.file.Name()), bar, percent,
				FormatSize(w.current), FormatSize(w.total), FormatSize(int64(w.lastSpeed)), w.lastETA)
		} else {
			line = fmt.Sprintf("[%s] Downloading: %s | Speed: %s/s",
				filepath.Base(w.file.Name()), FormatSize(w.current), FormatSize(int64(w.lastSpeed)))
		}

		width, _, err := term.GetSize(int(os.Stdout.Fd()))
		if err != nil || width <= 0 {
			width = 80
		}

		fmt.Printf("\033[2K\r%-*s", width-1, line)
		_ = os.Stdout.Sync()
	}
	w.mu.Unlock()
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
