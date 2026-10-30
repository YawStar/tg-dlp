package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/gotd/td/tg"
	"tg-dlp/pkg/runtime"
)

// LinkInfo is a parsed Telegram link.
type LinkInfo struct {
	ChannelID     int64
	AccessHash    int64
	Username      string
	MessageID     int
	ThreadID      int
	IsUsername    bool
	IsChannelOnly bool
}

// ParseLink parses public and private Telegram links.
func ParseLink(link string) (*LinkInfo, error) {
	threadID := 0
	if idx := strings.Index(link, "?"); idx != -1 {
		query := link[idx+1:]
		link = link[:idx]
		for _, param := range strings.Split(query, "&") {
			if strings.HasPrefix(param, "thread=") {
				if id, err := strconv.Atoi(strings.TrimPrefix(param, "thread=")); err == nil {
					threadID = id
				}
			}
		}
	}

	parts := strings.Split(strings.TrimSuffix(link, "/"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid link format")
	}

	info := &LinkInfo{ThreadID: threadID}

	if len(parts) >= 4 && parts[len(parts)-3] == "c" {
		channelID, err := strconv.ParseInt(parts[len(parts)-2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse channel ID: %w", err)
		}
		msgID, err := strconv.Atoi(parts[len(parts)-1])
		if err != nil {
			return nil, fmt.Errorf("cannot parse message ID: %w", err)
		}
		info.ChannelID = channelID
		info.MessageID = msgID
		return info, nil
	}

	if len(parts) >= 4 && parts[len(parts)-2] == "c" {
		channelID, err := strconv.ParseInt(parts[len(parts)-1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("cannot parse channel ID: %w", err)
		}
		info.ChannelID = channelID
		info.IsChannelOnly = true
		return info, nil
	}

	lastPart := parts[len(parts)-1]
	msgID, err := strconv.Atoi(lastPart)
	if err == nil {
		info.MessageID = msgID
		info.Username = parts[len(parts)-2]
	} else {
		info.IsChannelOnly = true
		info.Username = lastPart
	}
	info.IsUsername = true
	return info, nil
}

// ResolveUsername resolves @username to an InputChannel.
func ResolveUsername(ctx context.Context, api *tg.Client, username string) (*tg.InputChannel, error) {
	runtime.DebugLog("Calling ContactsResolveUsername: %s", username)
	req := &tg.ContactsResolveUsernameRequest{Username: username}

	res, err := api.ContactsResolveUsername(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("cannot find username '%s': %w", username, err)
	}

	for _, chat := range res.Chats {
		if c, ok := chat.(*tg.Channel); ok {
			runtime.DebugLog("Channel found - ID: %d, AccessHash: %d", c.ID, c.AccessHash)
			return &tg.InputChannel{
				ChannelID:  c.ID,
				AccessHash: c.AccessHash,
			}, nil
		}
	}
	return nil, fmt.Errorf("username '%s' is not a channel", username)
}

// JoinChannel joins a channel, ignoring already-participant errors.
func JoinChannel(ctx context.Context, api *tg.Client, inputChannel *tg.InputChannel) error {
	runtime.DebugLog("Calling ChannelsJoinChannel...")
	_, err := api.ChannelsJoinChannel(ctx, inputChannel)
	if err != nil {
		if strings.Contains(err.Error(), "USER_ALREADY_PARTICIPANT") {
			runtime.DebugLog("Already a participant in this channel.")
			return nil
		}
		return err
	}
	runtime.DebugLog("Successfully joined the channel.")
	return nil
}

// FetchChannelByID resolves a channel ID to an InputChannel by querying the server.
func FetchChannelByID(ctx context.Context, api *tg.Client, channelID int64) (*tg.InputChannel, error) {
	channelsRes, err := api.ChannelsGetChannels(ctx, []tg.InputChannelClass{
		&tg.InputChannel{ChannelID: channelID, AccessHash: 0},
	})
	if err != nil {
		return nil, err
	}
	switch chs := channelsRes.(type) {
	case *tg.MessagesChats:
		for _, chat := range chs.Chats {
			if c, ok := chat.(*tg.Channel); ok && c.ID == channelID {
				return &tg.InputChannel{ChannelID: c.ID, AccessHash: c.AccessHash}, nil
			}
		}
	case *tg.MessagesChatsSlice:
		for _, chat := range chs.Chats {
			if c, ok := chat.(*tg.Channel); ok && c.ID == channelID {
				return &tg.InputChannel{ChannelID: c.ID, AccessHash: c.AccessHash}, nil
			}
		}
	}
	return nil, fmt.Errorf("channel %d not found", channelID)
}