package store

import (
	"testing"
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

var (
	yearAgo = model.GetMillisForTime(time.Now().AddDate(-1, 0, 0))
	weekAgo = model.GetMillisForTime(time.Now().AddDate(0, 0, -7))
)

func TestSQLStore_GetStaleChannels_Simplified(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	// Create a single channel
	channels, err := th.CreateChannels(1, "stale-test", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// Add posts and reactions to the channel
	posts, err := th.CreatePosts(10, th.User1.Id, channels[0].Id)
	require.NoError(t, err)

	reactions, err := th.CreateReactions(posts, th.User1.Id)
	require.NoError(t, err)
	assert.NotEmpty(t, reactions)

	// Adjust timestamps for the channel, posts, and reactions
	yearAgo := model.GetMillisForTime(time.Now().AddDate(-1, 0, 0))
	weekAgo := model.GetMillisForTime(time.Now().AddDate(0, 0, -7))

	setTimestamps(t, th, "Channels", channels[0].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[0].Id, weekAgo, weekAgo, 0)
	setTimestamps(t, th, "Reactions", channels[0].Id, yearAgo, yearAgo, yearAgo)

	// Fetch channels stale for 30 days or more
	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
	}
	staleChannels, more, err := th.Store.GetStaleChannels(opts, 0, 0)
	require.NoError(t, err)
	assert.False(t, more)

	// Verify that the channel is not considered stale
	assert.Empty(t, staleChannels)
}

func TestSQLStore_GetStaleChannels(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	const channelCount = 10
	const postCount = 10

	// create a bunch of channels
	channels, err := th.CreateChannels(channelCount, "stale-test", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	var posts []*model.Post
	var reactions []*model.Reaction

	// add some posts and reactions
	for _, channel := range channels {
		posts, err = th.CreatePosts(postCount, th.User1.Id, channel.Id)
		require.NoError(t, err)

		reactions, err = th.CreateReactions(posts, th.User1.Id)
		require.NoError(t, err)
		assert.NotEmpty(t, reactions)
	}

	// channel 0 - adjust all timestamps to 1 year old (stale)
	setTimestamps(t, th, "Channels", channels[0].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[0].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Reactions", channels[0].Id, yearAgo, yearAgo, 0)

	// channel 1 - posts and reactions deleted a year ago (stale)
	setTimestamps(t, th, "Channels", channels[1].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[1].Id, yearAgo, yearAgo, yearAgo)
	setTimestamps(t, th, "Reactions", channels[1].Id, yearAgo, yearAgo, yearAgo)

	// channels 2-4 - all timestamps current (not stale)

	// channel 5 - posts and reactions deleted a week ago (not stale)
	setTimestamps(t, th, "Channels", channels[5].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[5].Id, yearAgo, weekAgo, weekAgo)
	setTimestamps(t, th, "Reactions", channels[5].Id, yearAgo, weekAgo, weekAgo)

	// channel 6 - old channel timstamps, new posts (not stale)
	setTimestamps(t, th, "Channels", channels[6].Id, yearAgo, yearAgo, 0)

	// channel 7 - deleted channel (not stale)
	setTimestamps(t, th, "Channels", channels[7].Id, yearAgo, yearAgo, weekAgo)

	// channel 8 - adjust post timestamps to 1 year old, leave reactions (not stale)
	setTimestamps(t, th, "Channels", channels[8].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[8].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Reactions", channels[8].Id, weekAgo, weekAgo, weekAgo)

	// channel 9 - adjust all post/reaction timestamps to 1 week old (not stale)
	setTimestamps(t, th, "Channels", channels[9].Id, yearAgo, weekAgo, 0)
	setTimestamps(t, th, "Posts", channels[9].Id, yearAgo, weekAgo, 0)
	setTimestamps(t, th, "Reactions", channels[9].Id, weekAgo, weekAgo, 0)

	// fetch channels stale for 30 days or more
	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
	}
	staleChannels, more, err := th.Store.GetStaleChannels(opts, 0, 0)
	require.NoError(t, err)
	assert.False(t, more)
	assert.Len(t, staleChannels, 2)

	// only channels 0,1 are stale
	staleIDs := make([]string, 0, len(staleChannels))
	for _, ch := range staleChannels {
		staleIDs = append(staleIDs, ch.Id)
	}
	assert.ElementsMatch(t, staleIDs, []string{channels[0].Id, channels[1].Id})

	for i, ch := range channels {
		t.Log(i, ch.Id)
	}
}

func TestSQLStore_GetStaleChannelsEmptyChannel(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	const channelCount = 3

	channels, err := th.CreateChannels(channelCount, "empty-channel-test", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// make 0,2 stale
	setTimestamps(t, th, "Channels", channels[0].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Channels", channels[2].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[0].Id, yearAgo, yearAgo, 0)
	setTimestamps(t, th, "Posts", channels[2].Id, yearAgo, yearAgo, 0)

	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
	}
	staleChannels, more, err := th.Store.GetStaleChannels(opts, 0, 0)
	require.NoError(t, err)
	assert.False(t, more)

	assert.Len(t, staleChannels, 2)

	staleIDs := extractChannelIDs(staleChannels)
	assert.ElementsMatch(t, staleIDs, []string{channels[0].Id, channels[2].Id})
}

func TestSQLStore_GetStaleChannelsPagnation(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	const channelCount = 100

	channels, err := th.CreateChannels(channelCount, "pagnation-test", th.User1.Id, th.Team2.Id)
	require.NoError(t, err)

	// make 50 of them stale
	for i, ch := range channels {
		if i < 50 {
			setTimestamps(t, th, "Channels", ch.Id, yearAgo, yearAgo, 0)
		}
		// When you create the channel it creates a post, so we need to update the post timestamp
		setTimestamps(t, th, "Posts", ch.Id, yearAgo, yearAgo, 0)
	}

	const pageSize = 10
	var page = 0
	staleChannels := make([]*model.Channel, 0)
	loopCount := 0

	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
	}

	// fetch channels stale for 30 days or more
	for {
		fetchedChannels, more, err := th.Store.GetStaleChannels(opts, page, pageSize)
		require.NoError(t, err)
		page++
		loopCount++

		staleChannels = append(staleChannels, fetchedChannels...)

		if !more {
			break
		}
	}

	assert.Equal(t, loopCount, 5)
	assert.Len(t, staleChannels, 50)

	staleIDs := extractChannelIDs(staleChannels)
	channelIDs := extractChannelIDs(channels[:50])
	assert.ElementsMatch(t, staleIDs, channelIDs)
}

func TestSQLStore_GetStaleChannelsExclude(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	const channelCount = 20

	channels, err := th.CreateChannels(channelCount, "exclude-channel-test", th.User1.Id, th.Team1.Id)
	require.NoError(t, err)

	// make first 5 stale
	for i, ch := range channels {
		if i < 5 {
			setTimestamps(t, th, "Channels", ch.Id, yearAgo, yearAgo, 0)
		}
		// When you create the channel it creates a post, so we need to update the post timestamp
		setTimestamps(t, th, "Posts", ch.Id, yearAgo, yearAgo, 0)
	}

	// exclude the first 3
	exclude := []string{channels[0].Id, channels[1].Id, channels[2].Id}

	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
		ExcludeChannels:        exclude,
	}
	staleChannels, more, err := th.Store.GetStaleChannels(opts, 0, 0)
	require.NoError(t, err)
	assert.False(t, more)

	assert.Len(t, staleChannels, 2)

	staleIDs := extractChannelIDs(staleChannels)
	assert.ElementsMatch(t, staleIDs, []string{channels[3].Id, channels[4].Id})
}

func TestSQLStore_GetStaleChannelsNone(t *testing.T) {
	th := SetupHelper(t).SetupBasic(t)
	defer th.TearDown()

	const channelCount = 10

	_, err := th.CreateChannels(channelCount, "no-results-test", th.User1.Id, th.Team2.Id)
	require.NoError(t, err)

	opts := StaleChannelOpts{
		AgeInDays:              30,
		IncludeChannelTypeOpen: true,
	}
	staleChannels, more, err := th.Store.GetStaleChannels(opts, 0, 0)
	require.NoError(t, err)
	assert.False(t, more)
	assert.Empty(t, staleChannels)
}

func setTimestamps(t *testing.T, th *TestHelper, table string, channelID string, createAt, updateAt, deleteAt int64) {
	query := th.Store.builder.Update(table)

	if createAt >= 0 {
		query = query.Set("CreateAt", createAt)
	}
	if updateAt >= 0 {
		query = query.Set("UpdateAt", updateAt)
	}
	if deleteAt >= 0 {
		query = query.Set("DeleteAt", deleteAt)
	}

	switch table {
	case "Channels":
		query = query.Where(sq.Eq{"Id": channelID})
	case "Posts":
		query = query.Where(sq.Eq{"ChannelId": channelID})
	case "Reactions":
		// `reactions.channelid` does not exist in all server versions we need to support, therefore
		// we need to update all reactions belonging to posts in the channel.
		query = query.Where(sq.Eq{"ChannelId": channelID})
	default:
		panic("invalid table name")
	}

	sql, args, err := query.ToSql()
	require.NoError(t, err)

	t.Logf("setTimestamps: %s, %d", sql, args)

	result, err := query.Exec()
	require.NoError(t, err)

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)

	t.Logf("setTimestamps for channelID %s, for %s, %d rows affected.", channelID, table, rowsAffected)
}

func getTimestamps(t *testing.T, th *TestHelper, table string, id string) (createAt, updateAt, deleteAt int64) {
	query := th.Store.builder.Select("CreateAt", "UpdateAt", "DeleteAt").
		From(table)

	if table == "Reactions" {
		query = query.Where(sq.Eq{"ChannelId": id})
		query = query.Limit(1)
	} else {
		query = query.Where(sq.Eq{"Id": id})
	}

	row := query.QueryRow()
	err := row.Scan(&createAt, &updateAt, &deleteAt)
	require.NoError(t, err)

	return createAt, updateAt, deleteAt
}

func extractChannelIDs(channels []*model.Channel) []string {
	ids := make([]string, 0, len(channels))
	for _, ch := range channels {
		ids = append(ids, ch.Id)
	}
	return ids
}
