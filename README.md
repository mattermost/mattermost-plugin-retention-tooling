# Mattermost Rentention Tooling plugin ![CI](https://github.com/mattermost/mattermost-plugin-retention-tooling/actions/workflows/ci.yml/badge.svg)

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

