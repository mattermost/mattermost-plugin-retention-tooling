package store

import (
	"time"

	sq "github.com/Masterminds/squirrel"

	"github.com/mattermost/mattermost-server/v6/model"
)

var (
	defaultChannels = []string{"town-square", "off-topic"}
)

type StaleChannelOpts struct {
	AgeInDays                 int
	ExcludeChannels           []string
	IncludeChannelTypeOpen    bool
	IncludeChannelTypePrivate bool
	IncludeChannelTypeDirect  bool
	IncludeChannelTypeGroup   bool
}

func (ss *SQLStore) GetStaleChannels(opts StaleChannelOpts, page int, pageSize int) ([]*model.Channel, bool, error) {
	olderThan := model.GetMillisForTime(time.Now().AddDate(0, 0, -opts.AgeInDays))

	excludeChannels := make([]string, 0)
	excludeChannels = append(excludeChannels, opts.ExcludeChannels...)
	excludeChannels = append(excludeChannels, defaultChannels...)

	// find all channels where no posts or reactions have been modified,deleted since the olderThan timestamp.
	query := ss.builder.Select("ch.Id", "ch.Name").Distinct().
		From("Channels as ch").
		LeftJoin("Posts as p ON ch.Id=p.ChannelId").
		LeftJoin("Reactions as r ON p.Id=r.PostId"). // reactions.channelid does not exist in all versions of server
		Where(sq.Eq{"ch.DeleteAt": 0}).
		Where(sq.Lt{"ch.UpdateAt": olderThan}).
		Where(sq.Or{sq.Eq{"p.UpdateAt": nil}, sq.Lt{"p.UpdateAt": olderThan, "p.DeleteAt": olderThan}}).
		Where(sq.Or{sq.Eq{"r.UpdateAt": nil}, sq.Lt{"r.UpdateAt": olderThan, "r.DeleteAt": olderThan}}).
		OrderBy("ch.Id")

	if len(excludeChannels) > 0 {
		query = query.Where(sq.And{
			sq.NotEq{"ch.Id": excludeChannels},
			sq.NotEq{"ch.Name": excludeChannels},
		})
	}

	channelTypes := []string{}
	if opts.IncludeChannelTypeOpen {
		channelTypes = append(channelTypes, string(model.ChannelTypeOpen))
	}
	if opts.IncludeChannelTypePrivate {
		channelTypes = append(channelTypes, string(model.ChannelTypePrivate))
	}
	if opts.IncludeChannelTypeDirect {
		channelTypes = append(channelTypes, string(model.ChannelTypeDirect))
	}
	if opts.IncludeChannelTypeGroup {
		channelTypes = append(channelTypes, string(model.ChannelTypeGroup))
	}
	query = query.Where(sq.Eq{"ch.Type": channelTypes})

	if page > 0 {
		query = query.Offset(uint64(page * pageSize))
	}

	if pageSize > 0 {
		// N+1 to check if there's a next page for pagination
		query = query.Limit(uint64(pageSize) + 1)
	}

	rows, err := query.Query()
	if err != nil {
		ss.logger.Error("error fetching stale channels", "err", err)
		return nil, false, err
	}

	channels := []*model.Channel{}
	for rows.Next() {
		channel := &model.Channel{}

		if err := rows.Scan(&channel.Id, &channel.Name); err != nil {
			ss.logger.Error("error scanning stale channels", "err", err)
			return nil, false, err
		}
		channels = append(channels, channel)
	}

	var hasMore bool
	if pageSize > 0 && len(channels) > pageSize {
		hasMore = true
		channels = channels[0:pageSize]
	}

	return channels, hasMore, nil
}
