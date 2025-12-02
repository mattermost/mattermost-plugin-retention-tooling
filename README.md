# Mattermost Retention Tooling plugin ![CI](https://github.com/mattermost/mattermost-plugin-retention-tooling/actions/workflows/ci.yml/badge.svg)

This plugin provides data retention tools to augment the [data retention capabilities](https://docs.mattermost.com/comply/data-retention-policy.html) of Mattermost Enterprise Edition.

**Not recommended for production use without Mattermost guidance. Please reach out to your Customer Success Manager to learn more.**

## Tools

### De-activated User Clean-up

Removes a specified user from all teams and channels, meant to be used after a user is deactivated.

The process is started by sending an HTTP POST request to the Mattermost server at `/plugins/mattermost-plugin-retention-tooling/remove_user_from_all_teams_and_channels`. It accepts either of the following JSON request bodies:

```
{"user_id": "someuserid"}

{"username": "someusername"}
```

The user submitting the HTTP request must be a system admin.

### Channel Archiver

Will auto-archive any channels that have had no activity for more than some configurable number of days.

**Job**: can be configured via the system console to run monthly/weekly/daily on a specific day of the week and time of day.

**Slash command**: Can be run on-demand via `/channel-archiver` slash command.

#### Configuration

**Days of inactivity**: Number of days a channel must be inactive before it's considered stale. Minimum value is 30 days. Default is 365 days.

**Frequency**: How often the Channel Archiver job runs. Options are:
- Monthly: Runs once per month on the specified day of week
- Weekly: Runs once per week on the specified day of week
- Daily: Runs every day at the specified time

**Day of week**: The day of the week the job runs (applies to Monthly and Weekly frequency).

**Time of day**: The time when the job runs. Format: `h:mmam/pm ±HHMM` (e.g., `1:00am -0700` for 1 AM Pacific, `9:30pm +0100` for 9:30 PM Central Europe).

**Exclude channels**: Comma-separated list of channel names (case sensitive) or channel IDs that should never be archived automatically.

**Batch size**: Number of channels to process in each batch. Default is 100. Adjust this value based on your server capacity.

**Dry run mode**: When enabled, the Channel Archiver identifies stale channels but does not archive them automatically. Stale channel reports are posted to the configured admin channel. To archive the channels after reviewing the list, you can either use the `/channel-archiver` slash command to manually trigger archiving, or disable dry run mode so channels will be archived automatically on the next scheduled run.

**Admin channel**: Channel ID where the Channel Archiver posts job updates. When dry run mode is enabled, stale channel reports are posted here. When channels are archived, a summary of archived channels is posted to this channel.

