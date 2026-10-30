package telegram

import "testing"

func TestParseLink(t *testing.T) {
	cases := []struct {
		name       string
		link       string
		wantUser   string
		wantMsg    int
		wantCh     int64
		wantChOnly bool
		wantThread int
	}{
		{"public post", "https://t.me/durov/42", "durov", 42, 0, false, 0},
		{"public channel", "https://t.me/durov", "durov", 0, 0, true, 0},
		{"private post", "https://t.me/c/123456789/42", "", 42, 123456789, false, 0},
		{"private channel", "https://t.me/c/123456789", "", 0, 123456789, true, 0},
		{"thread", "https://t.me/durov/42?thread=99", "durov", 42, 0, false, 99},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			info, err := ParseLink(tc.link)
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if info.Username != tc.wantUser {
				t.Errorf("username = %q, want %q", info.Username, tc.wantUser)
			}
			if info.MessageID != tc.wantMsg {
				t.Errorf("msg = %d, want %d", info.MessageID, tc.wantMsg)
			}
			if info.ChannelID != tc.wantCh {
				t.Errorf("ch = %d, want %d", info.ChannelID, tc.wantCh)
			}
			if info.IsChannelOnly != tc.wantChOnly {
				t.Errorf("chOnly = %v, want %v", info.IsChannelOnly, tc.wantChOnly)
			}
			if info.ThreadID != tc.wantThread {
				t.Errorf("thread = %d, want %d", info.ThreadID, tc.wantThread)
			}
		})
	}
}

func TestParseExtensions(t *testing.T) {
	got := ParseExtensions("MP4, .mkv ,  JPG")
	if !got["mp4"] || !got["mkv"] || !got["jpg"] {
		t.Errorf("ParseExtensions failed: %v", got)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 entries, got %d", len(got))
	}
}