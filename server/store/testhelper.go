package store

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	sq "github.com/Masterminds/squirrel"
	mmcontainer "github.com/mattermost/testcontainers-mattermost-go"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
)

type TestHelper struct {
	mattermost  *mmcontainer.MattermostContainer
	AdminClient *model.Client4
	UserClient  *model.Client4

	Store *SQLStore

	Team1    *model.Team
	Team2    *model.Team
	Channel1 *model.Channel
	Channel2 *model.Channel
	User1    *model.User
}

func SetupHelper(t *testing.T) *TestHelper {
	t.Helper()
	ctx := context.TODO()
	mattermost, err := mmcontainer.RunContainer(ctx)
	require.NoError(t, err)

	th := &TestHelper{}
	th.mattermost = mattermost

	store, err := New(storeWrapper{mattermost}, &testLogger{t})
	require.NoError(t, err, "could not create store")
	th.Store = store

	adminClient, err := th.mattermost.GetAdminClient(ctx)
	require.NoError(t, err, "could not create admin client")
	th.AdminClient = adminClient

	return th
}

func (th *TestHelper) SetupBasic(t *testing.T) *TestHelper {
	// create some teams
	teams, err := th.CreateTeams(2, "test-team")
	require.NoError(t, err, "could not create teams")
	th.Team1 = teams[0]
	th.Team2 = teams[1]

	// create some users
	users, err := th.CreateUsers(2, "test.user")
	require.NoError(t, err)
	th.User1 = users[0]

	ctx := context.TODO()
	client, err := th.mattermost.GetClient(ctx, th.User1.Username, "test.user")
	require.NoError(t, err, "could not create client")
	th.UserClient = client

	return th
}

func (th *TestHelper) TearDown() {
	if th.mattermost != nil {
		err := th.mattermost.Terminate(context.TODO())
		if err != nil {
			panic(err)
		}
	}
}

func (th *TestHelper) CreateTeams(num int, namePrefix string) ([]*model.Team, error) {
	var teams []*model.Team
	for i := 0; i < num; i++ {
		team := &model.Team{
			Name:        fmt.Sprintf("%s-%d", namePrefix, i),
			DisplayName: fmt.Sprintf("%s-%d", namePrefix, i),
			Type:        model.TeamOpen,
		}

		team, _, err := th.AdminClient.CreateTeam(context.TODO(), team)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, nil
}

func (th *TestHelper) CreateChannels(num int, namePrefix string, userID string, teamID string) ([]*model.Channel, error) {
	var channels []*model.Channel
	for i := 0; i < num; i++ {
		channel := &model.Channel{
			Name:        fmt.Sprintf("%s-%d", namePrefix, i),
			DisplayName: fmt.Sprintf("%s-%d", namePrefix, i),
			Type:        model.ChannelTypeOpen,
			CreatorId:   userID,
			TeamId:      teamID,
		}
		channel, _, err := th.UserClient.CreateChannel(context.TODO(), channel)
		if err != nil {
			return nil, err
		}
		channels = append(channels, channel)
	}
	return channels, nil
}

func (th *TestHelper) CreateUsers(num int, namePrefix string) ([]*model.User, error) {
	var users []*model.User
	for i := 0; i < num; i++ {
		user := &model.User{
			Username: fmt.Sprintf("%s-%d", namePrefix, i),
			Password: namePrefix,
			Email:    fmt.Sprintf("%s@example.com", model.NewId()),
		}
		user, _, err := th.AdminClient.CreateUser(context.TODO(), user)
		if err != nil {
			return nil, err
		}

		_, _, err = th.AdminClient.AddTeamMember(context.TODO(), th.Team1.Id, user.Id)
		if err != nil {
			return nil, err
		}

		_, _, err = th.AdminClient.AddTeamMember(context.TODO(), th.Team2.Id, user.Id)
		if err != nil {
			return nil, err
		}

		users = append(users, user)
	}
	return users, nil
}

func (th *TestHelper) CreatePosts(num int, userID string, channelID string) ([]*model.Post, error) {
	var posts []*model.Post
	for i := 0; i < num; i++ {
		post := &model.Post{
			UserId:    userID,
			ChannelId: channelID,
			Type:      model.PostTypeDefault,
			Message:   fmt.Sprintf("test post %d of %d", i, num),
		}
		post, _, err := th.UserClient.CreatePost(context.TODO(), post)
		if err != nil {
			return nil, err
		}
		posts = append(posts, post)
	}
	return posts, nil
}

func (th *TestHelper) CreateReactions(posts []*model.Post, userID string) ([]*model.Reaction, error) {
	var reactions []*model.Reaction
	for _, post := range posts {
		reaction := &model.Reaction{
			PostId:    post.Id,
			UserId:    userID,
			EmojiName: "shrug",
			ChannelId: post.ChannelId,
		}
		reaction, _, err := th.UserClient.SaveReaction(context.TODO(), reaction)
		if err != nil {
			return nil, err
		}
		reactions = append(reactions, reaction)
	}
	return reactions, nil
}

func SetTimestamps(t *testing.T, th *TestHelper, table string, channelID string, createAt, updateAt, deleteAt int64) {
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

	result, err := query.Exec()
	require.NoError(t, err)

	rowsAffected, err := result.RowsAffected()
	require.NoError(t, err)

	t.Logf("SetTimestamps for channelID %s, for %s, %d rows affected.", channelID, table, rowsAffected)
}

// storeWrapper is a wrapper for MainHelper that implements SQLStoreSource interface.
type storeWrapper struct {
	mattermost *mmcontainer.MattermostContainer
}

func (sw storeWrapper) GetMasterDB() (*sql.DB, error) {
	sqlDB, err := sw.mattermost.PostgresConnection(context.TODO())
	if err != nil {
		return nil, err
	}
	return sqlDB, nil
}
func (sw storeWrapper) DriverName() string {
	return model.DatabaseDriverPostgres
}

type testLogger struct {
	tb testing.TB
}

// Error logs an error message, optionally structured with alternating key, value parameters.
func (l *testLogger) Error(message string, keyValuePairs ...interface{}) {
	l.log("error", message, keyValuePairs...)
}

// Warn logs an error message, optionally structured with alternating key, value parameters.
func (l *testLogger) Warn(message string, keyValuePairs ...interface{}) {
	l.log("warn", message, keyValuePairs...)
}

// Info logs an error message, optionally structured with alternating key, value parameters.
func (l *testLogger) Info(message string, keyValuePairs ...interface{}) {
	l.log("info", message, keyValuePairs...)
}

// Debug logs an error message, optionally structured with alternating key, value parameters.
func (l *testLogger) Debug(message string, keyValuePairs ...interface{}) {
	l.log("debug", message, keyValuePairs...)
}

func (l *testLogger) log(level string, message string, keyValuePairs ...interface{}) {
	var args strings.Builder

	if len(keyValuePairs) > 0 && len(keyValuePairs)%2 != 0 {
		keyValuePairs = keyValuePairs[:len(keyValuePairs)-1]
	}

	for i := 0; i < len(keyValuePairs); i += 2 {
		args.WriteString(fmt.Sprintf("%v:%v  ", keyValuePairs[i], keyValuePairs[i+1]))
	}

	l.tb.Logf("level=%s  message=%s  %s", level, message, args.String())
}
