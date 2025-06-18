package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/wiggin77/merror"

	"github.com/mattermost/mattermost-plugin-retention-tooling/server/bot"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/channels"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/config"
	"github.com/mattermost/mattermost-plugin-retention-tooling/server/store"
	"github.com/mattermost/mattermost/server/public/plugin"
	pluginapi "github.com/mattermost/mattermost/server/public/pluginapi"
	"github.com/mattermost/mattermost/server/public/pluginapi/cluster"
)

type ChannelArchiverJob struct {
	mux      sync.Mutex
	settings *ChannelArchiverJobSettings
	job      *cluster.Job
	runner   *runInstance

	id       string
	papi     plugin.API
	client   *pluginapi.Client
	bot      *bot.Bot
	sqlstore *store.SQLStore
}

func NewChannelArchiverJob(id string, api plugin.API, client *pluginapi.Client, sqlstore *store.SQLStore) (*ChannelArchiverJob, error) {
	bot, err := bot.New(client)
	if err != nil {
		return nil, fmt.Errorf("cannot create bot for job: %w", err)
	}

	return &ChannelArchiverJob{
		settings: &ChannelArchiverJobSettings{},
		id:       id,
		papi:     api,
		client:   client,
		bot:      bot,
		sqlstore: sqlstore,
	}, nil
}

func (j *ChannelArchiverJob) GetID() string {
	return j.id
}

// OnConfigurationChange is called by the job manager whenenver the plugin settings have changed.
// Stop current job (if any) and start a new job (if enabled) with new settings.
func (j *ChannelArchiverJob) OnConfigurationChange(cfg *config.Configuration) error {
	settings, err := parseChannelArchiverJobSettings(cfg)
	if err != nil {
		return err
	}

	// stop existing job (if any)
	if err := j.Stop(time.Second * 10); err != nil {
		j.client.Log.Error("Error stopping Channel Archiver job for config change", "err", err)
	}

	if settings.EnableChannelArchiver {
		return j.start(settings)
	}

	return nil
}

// start schedules a new job with specified settings.
func (j *ChannelArchiverJob) start(settings *ChannelArchiverJobSettings) error {
	j.mux.Lock()
	defer j.mux.Unlock()

	j.settings = settings

	job, err := cluster.Schedule(j.papi, j.id, j.nextWaitInterval, j.run)
	if err != nil {
		return fmt.Errorf("cannot start Channel Archiver: %w", err)
	}
	j.job = job

	j.client.Log.Debug("Channel Archiver started", "dow", settings.DayOfWeek)

	return nil
}

// Stop stops the current job (if any). If the timeout is exceeded an error
// is returned.
func (j *ChannelArchiverJob) Stop(timeout time.Duration) error {
	var job *cluster.Job
	var runner *runInstance

	j.mux.Lock()
	job = j.job
	runner = j.runner
	j.job = nil
	j.runner = nil
	j.mux.Unlock()

	merr := merror.New()

	if job != nil {
		if err := job.Close(); err != nil {
			merr.Append(fmt.Errorf("error closing job: %w", err))
		}
	}

	if runner != nil {
		if err := runner.stop(timeout); err != nil {
			merr.Append(fmt.Errorf("error stopping job runner: %w", err))
		}
	}

	j.client.Log.Debug("Channel Archiver stopped", "err", merr.ErrorOrNil())

	return merr.ErrorOrNil()
}

func (j *ChannelArchiverJob) getSettings() *ChannelArchiverJobSettings {
	j.mux.Lock()
	defer j.mux.Unlock()
	return j.settings.Clone()
}

// nextWaitInterval is called by the cluster job scheduler to determine how long to wait until the
// next job run.
func (j *ChannelArchiverJob) nextWaitInterval(now time.Time, metaData cluster.JobMetadata) time.Duration {
	settings := j.getSettings()

	lastFinished := metaData.LastFinished
	if lastFinished.IsZero() {
		lastFinished = now
	}

	next := settings.Frequency.CalcNext(lastFinished, settings.DayOfWeek, settings.TimeOfDay)
	delta := next.Sub(now)

	j.client.Log.Debug("Channel Archiver next run scheduled", "last", lastFinished.Format(FullLayout), "next", next.Format(FullLayout), "wait", delta.String())

	return delta
}

func (j *ChannelArchiverJob) run() {
	exitSignal := make(chan struct{})
	ctx, canceller := context.WithCancel(context.Background())

	runner := &runInstance{
		canceller:  canceller,
		exitSignal: exitSignal,
	}

	var oldRunner *runInstance
	var settings *ChannelArchiverJobSettings
	j.mux.Lock()
	oldRunner = j.runner
	j.runner = runner
	settings = j.settings.Clone()
	j.mux.Unlock()

	defer func() {
		close(exitSignal)
		j.mux.Lock()
		j.runner = nil
		j.mux.Unlock()
	}()

	if oldRunner != nil {
		j.client.Log.Error("Multiple Channel Archiver jobs scheduled concurrently; there can be only one")
		return
	}

	opts := channels.ArchiverOpts{
		StaleChannelOpts: store.StaleChannelOpts{
			AgeInDays:                 settings.AgeInDays,
			IncludeChannelTypeOpen:    true,
			IncludeChannelTypePrivate: true,
			ExcludeChannels:           settings.ExcludeChannels,
			AdminChannel:              settings.AdminChannel,
		},
		BatchSize: settings.BatchSize,
		Bot:       j.bot,
		ListOnly:  settings.EnableChannelArchiverDryRunMode,
	}

	results, err := channels.ArchiveStaleChannels(ctx, j.sqlstore, j.client, opts)
	if err != nil {
		j.client.Log.Error("Error running Channel Archiver job: %w", err)
		return
	}

	j.client.Log.Info("Channel Archiver job", "channels_archived", len(results.ChannelsArchived), "status", results.ExitReason, "duration", results.Duration.String())
}

type runInstance struct {
	canceller  func()        // called to stop a currently executing run
	exitSignal chan struct{} // closed when the currently executing run has exited
}

func (r *runInstance) stop(timeout time.Duration) error {
	// cancel the run
	r.canceller()

	// wait for it to exit
	select {
	case <-r.exitSignal:
		return nil
	case <-time.After(timeout):
		return fmt.Errorf("waiting on job to stop timed out after %s", timeout.String())
	}
}
