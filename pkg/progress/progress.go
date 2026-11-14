package progress

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Writer is an io.Writer that renders a live progress bar.
type Writer struct {
	file    *os.File
	total   int64
	current int64
	start   time.Time
	mu      sync.Mutex
	last    time.Time
	lastCurrent int64 // ⭐ Speed တွက်ဖို့ နောက်ဆုံး ရောက်ခဲ့တဲ့ နေရာ
}

// New creates a progress writer.
// current is the byte offset already written (resume support).
func New(file *os.File, total, current int64) *Writer {
	now := time.Now()
	return &Writer{
		file:        file,
		total:       total,
		current:     current,
		start:       now,
		last:        now,
		lastCurrent: current, // ⭐ စဖွင့်ချိန်မှာ ရှိပြီးသား ပမာဏ
	}
}

func (w *Writer) Write(p []byte) (int, error) {
	n, err := w.file.Write(p)
	if err != nil {
		return n, err
	}

	w.mu.Lock()
	w.current += int64(n)
	now := time.Now()

	if now.Sub(w.last) >= 500*time.Millisecond || (w.total > 0 && w.current == w.total) {
		elapsed := now.Sub(w.last).Seconds()
		// ⭐ အရင်က ဆွဲထားတာကို မတွက်ဘဲ လက်ရှိ 500ms အတွင်း ဆွဲချလိုက်တဲ့ ပမာဏကိုပဲ တွက်မယ် ⭐
		deltaBytes := w.current - w.lastCurrent
		
		if elapsed <= 0 {
			elapsed = 0.001
		}
		speed := float64(deltaBytes) / elapsed

		w.last = now
		w.lastCurrent = w.current // ⭐ နောက်တစ်ခါ တွက်ဖို့ Update လုပ်မယ်

		var percent float64
		if w.total > 0 {
			percent = float64(w.current) / float64(w.total) * 100
			if percent > 100 {
				percent = 100
			}
		}

		var eta string
		// ETA တွက်တဲ့အခါ Speed က 0 ဖြစ်နေရင် Error မတက်အောင် ကာကွယ်မယ်
		if w.total > 0 && speed > 0 {
			etaSeconds := int(float64(w.total-w.current) / speed)
			minutes := etaSeconds / 60
			seconds := etaSeconds % 60
			if minutes > 0 {
				eta = fmt.Sprintf("%dm %ds", minutes, seconds)
			} else {
				eta = fmt.Sprintf("%ds", seconds)
			}
		} else {
			eta = "--"
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

		if w.total > 0 {
			fmt.Printf("\r[%s] %s %.1f%% | %s / %s | Speed: %s/s | ETA: %s",
				filepath.Base(w.file.Name()), bar, percent,
				FormatSize(w.current), FormatSize(w.total), FormatSize(int64(speed)), eta)
		} else {
			fmt.Printf("\r[%s] Downloading: %s | Speed: %s/s",
				filepath.Base(w.file.Name()), FormatSize(w.current), FormatSize(int64(speed)))
		}
	}
	w.mu.Unlock()
	return n, nil
}

// FormatSize renders a byte count as a human-readable string.
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