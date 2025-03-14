package bot

import (
	"fmt"

	"github.com/mattermost/mattermost/server/public/model"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
)

const (
	BotUserName    = "channel-archiver"
	BotDisplayName = "Channel Archiver Bot"
	BotDescription = "Created by the Retention Tools Plugin."
)

type Bot struct {
	client *pluginapi.Client
	botID  string
}

func New(client *pluginapi.Client) (*Bot, error) {
	mmBot := &model.Bot{
		Username:    BotUserName,
		DisplayName: BotDisplayName,
		Description: BotDescription,
	}

	botID, err := client.Bot.EnsureBot(mmBot)
	if err != nil {
		return nil, fmt.Errorf("unable to ensure bot: %w", err)
	}

	return &Bot{
		client: client,
		botID:  botID,
	}, nil
}

func (b *Bot) SendEphemeralPost(channelID string, userID string, msg string) error {
	post := &model.Post{
		UserId:    b.botID,
		ChannelId: channelID,
		Message:   msg,
	}
	b.client.Post.SendEphemeralPost(userID, post)
	return nil
}

func (b *Bot) SendDirectPost(userID string, msg string) error {
	channel, err := b.client.Channel.GetDirect(userID, b.botID)
	if err != nil {
		return fmt.Errorf("bot cannot send direct message: %w", err)
	}

	post := &model.Post{
		UserId:    b.botID,
		ChannelId: channel.Id,
		Message:   msg,
	}
	return b.client.Post.CreatePost(post)
}

func (b *Bot) SendPost(channelID string, msg string) error {
	post := &model.Post{
		UserId:    b.botID,
		ChannelId: channelID,
		Message:   msg,
	}
	return b.client.Post.CreatePost(post)
}
