package channels

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-plugin-retention-tooling/server/bot"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/store"
	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/plugin/plugintest"
	"github.com/mattermost/mattermost/server/public/pluginapi"
)

var (
	staleTime = time.Now().AddDate(0, 0, -31)
	monthAgo  = model.GetMillisForTime(staleTime)
)

func TestArchiveStaleChannelsListMode(t *testing.T) {
	th := store.SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	mockAPI := &plugintest.API{}
	mockAPI.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	client := pluginapi.NewClient(mockAPI, nil)

	// Create test channels
	channels, err := th.CreateChannels(3, "test-channel", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// Set channels as stale by updating their UpdateAt timestamp
	for _, ch := range channels {
		store.SetTimestamps(t, th, "Posts", ch.Id, monthAgo, monthAgo, 0)
		store.SetTimestamps(t, th, "Channels", ch.Id, monthAgo, monthAgo, 0)
	}

	opts := ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:              30,
			IncludeChannelTypeOpen: true,
		},
		ListOnly:  true,
		BatchSize: 10,
	}

	results, err := ArchiveStaleChannels(context.Background(), th.Store, client, opts)
	require.NoError(t, err)
	assert.Equal(t, ReasonDone, results.ExitReason)
	assert.Len(t, results.ChannelsArchived, 3)
}

func TestArchiveStaleChannelsArchiveMode(t *testing.T) {
	th := store.SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	mockAPI := &plugintest.API{}
	mockAPI.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	client := pluginapi.NewClient(mockAPI, nil)

	// Create test channels
	channels, err := th.CreateChannels(3, "test-channel", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// Set channels as stale
	for _, ch := range channels {
		store.SetTimestamps(t, th, "Posts", ch.Id, monthAgo, monthAgo, 0)
		store.SetTimestamps(t, th, "Channels", ch.Id, monthAgo, monthAgo, 0)
	}

	// Mock the channel deletion
	for _, ch := range channels {
		mockAPI.On("DeleteChannel", ch.Id).Return(nil)
	}

	opts := ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:              30,
			IncludeChannelTypeOpen: true,
		},
		BatchSize: 10,
		ListOnly:  false,
	}

	results, err := ArchiveStaleChannels(context.Background(), th.Store, client, opts)
	require.NoError(t, err)
	assert.Equal(t, ReasonDone, results.ExitReason)
	assert.Len(t, results.ChannelsArchived, 3)

	mockAPI.AssertNumberOfCalls(t, "DeleteChannel", 3)
}

func TestArchiveStaleChannelsWithAdminChannelAndExclude(t *testing.T) {
	th, client, testBot, adminChannel, channels, mockAPI := setupStaleChannelsTest(t)
	defer th.TearDown()

	// Mock the channel deletion for all except excluded channel
	for i, ch := range channels {
		if i != 0 { // Skip first channel as it will be excluded
			mockAPI.On("DeleteChannel", ch.Id).Return(nil)
		}
	}

	opts := ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:              30,
			ExcludeChannels:        []string{channels[0].Id}, // Exclude first channel
			IncludeChannelTypeOpen: true,
			AdminChannel:           adminChannel[0].Id,
		},
		BatchSize: 10,
		Bot:       testBot,
		ListOnly:  false,
	}

	results, err := ArchiveStaleChannels(context.Background(), th.Store, client, opts)
	require.NoError(t, err)
	assert.Equal(t, ReasonDone, results.ExitReason)
	assert.Len(t, results.ChannelsArchived, 3) // Should be 3 since one is excluded

	// Verify admin channel and excluded channel are not in results
	for _, ch := range results.ChannelsArchived {
		assert.NotContains(t, ch, adminChannel[0].Id, "Admin channel should not be in results")
		assert.NotContains(t, ch, channels[0].Id, "Excluded channel should not be in results")
	}

	mockAPI.AssertNumberOfCalls(t, "DeleteChannel", 3)
}

func TestArchiveStaleChannelsListModeWithAdminChannelAndExclude(t *testing.T) {
	th, client, testBot, adminChannel, channels, _ := setupStaleChannelsTest(t)
	defer th.TearDown()

	opts := ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:              30,
			ExcludeChannels:        []string{channels[0].Id}, // Exclude first channel
			IncludeChannelTypeOpen: true,
			AdminChannel:           adminChannel[0].Id,
		},
		ListOnly:  true,
		BatchSize: 10,
		Bot:       testBot,
	}

	results, err := ArchiveStaleChannels(context.Background(), th.Store, client, opts)
	require.NoError(t, err)
	assert.Equal(t, ReasonDone, results.ExitReason)
	assert.Len(t, results.ChannelsArchived, 3) // Should be 3 since one is excluded

	// Verify admin channel and excluded channel are not in results
	for _, ch := range results.ChannelsArchived {
		assert.NotContains(t, ch, adminChannel[0].Id, "Admin channel should not be in results")
		assert.NotContains(t, ch, channels[0].Id, "Excluded channel should not be in results")
	}
}

func setupStaleChannelsTest(t *testing.T) (*store.TestHelper, *pluginapi.Client, *bot.Bot, []*model.Channel, []*model.Channel, *plugintest.API) {
	th := store.SetupHelper(t).SetupBasic(t)

	mockAPI := &plugintest.API{}
	mockString := mock.AnythingOfType("string")
	mockBytes := mock.AnythingOfType("[]uint8")
	mockPost := mock.AnythingOfType("*model.Post")
	mockBot := mock.AnythingOfType("*model.Bot")
	mockKVOptions := mock.AnythingOfType("model.PluginKVSetOptions")

	mockAPI.On("LogDebug", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return()
	mockAPI.On("UploadFile", mockBytes, mockString, mockString).Return(&model.FileInfo{Id: "test-file-id"}, nil)
	mockAPI.On("CreatePost", mockPost).Return(&model.Post{}, nil)
	mockAPI.On("GetBot", mockString).Return(&model.Bot{}, nil)
	mockAPI.On("EnsureBot", mockBot).Return("test-bot-id", nil)
	mockAPI.On("GetServerVersion").Return("9.6.0")
	mockAPI.On("KVSetWithOptions", mockString, mockBytes, mockKVOptions).Return(true, nil)
	mockAPI.On("KVGet", mockString).Return([]byte{}, nil)
	mockAPI.On("EnsureBotUser", mockBot).Return("test-bot-id", nil)

	client := pluginapi.NewClient(mockAPI, nil)
	testBot, err := bot.New(client)
	require.NoError(t, err)

	// Create test channels including admin channel
	adminChannel, err := th.CreateChannels(1, "admin-channel", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)
	channels, err := th.CreateChannels(4, "test-channel", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// Set channels as stale
	for _, ch := range channels {
		store.SetTimestamps(t, th, "Posts", ch.Id, monthAgo, monthAgo, 0)
		store.SetTimestamps(t, th, "Channels", ch.Id, monthAgo, monthAgo, 0)
	}

	// Set the admin channel to a month old as well, this channel should not be stale
	store.SetTimestamps(t, th, "Posts", adminChannel[0].Id, monthAgo, monthAgo, 0)
	store.SetTimestamps(t, th, "Channels", adminChannel[0].Id, monthAgo, monthAgo, 0)

	return th, client, testBot, adminChannel, channels, mockAPI
}
