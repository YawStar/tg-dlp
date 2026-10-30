package runtime

import "fmt"

// Build metadata — set via -ldflags
var (
	Version   = "0.1.2"
	Commit    = "none"     // GoReleaser က Build လုပ်ချိန်တွင် အလိုအလျောက် ပြောင်းမယ်
	BuildTime = "unknown"  // GoReleaser က Build လုပ်ချိန်တွင် အလိုအလျောက် ပြောင်းမယ်
)

// Runtime flags — set by commands at startup
var (
	DebugMode = false
	SkipFlood = false
)

// DebugLog prints only when DebugMode is enabled.
func DebugLog(format string, a ...interface{}) {
	if DebugMode {
		fmt.Printf("[DEBUG] "+format+"\n", a...)
	}
}