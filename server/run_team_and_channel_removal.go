package main

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost-server/v6/model"
)

type Payload struct {
	UserID   string `json:"user_id"`
	Username string `json:"username"`
}

func (p *Plugin) handleRemoveUserFromAllTeamsAndChannels(w http.ResponseWriter, r *http.Request) {
	var writeError = func(errorString string, statusCode int) {
		w.WriteHeader(statusCode)
		_ = json.NewEncoder(w).Encode(ErrorResponse{errorString})
	}

	if r.Method != http.MethodPost {
		writeError(fmt.Sprintf("unexpected HTTP method %s. Should be POST", r.Method), http.StatusMethodNotAllowed)
		return
	}

	requesterID := r.Header.Get("Mattermost-User-Id")
	if requesterID == "" {
		writeError("request is not from an authenticated user", http.StatusUnauthorized)
		return
	}

	isAdmin, err := p.ensureSystemAdmin(requesterID)
	if err != nil {
		writeError(fmt.Sprintf("error verifying whether user %s is a system admin: %s", requesterID, err.Error()), http.StatusUnauthorized)
		return
	}

	if !isAdmin {
		writeError(fmt.Sprintf("user %s is not a system admin", requesterID), http.StatusUnauthorized)
		return
	}

	err = p.removeUserFromAllTeamsAndChannels(r, requesterID)
	if err != nil {
		err = errors.Wrap(err, "error processing request")
		p.API.LogError(err.Error())
		writeError(err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(SuccessResponse{true})
}

func (p *Plugin) removeUserFromAllTeamsAndChannels(r *http.Request, requesterID string) error {
	var payload Payload
	err := json.NewDecoder(r.Body).Decode(&payload)
	if err != nil {
		return errors.Wrap(err, "error decoding user info payload")
	}
	r.Body.Close()

	var user *model.User
	var appErr *model.AppError

	// Get user from info supplied in request payload
	switch {
	case payload.UserID != "":
		user, appErr = p.API.GetUser(payload.UserID)
		if appErr != nil {
			return errors.Wrapf(appErr, "failed to get user with id %s", payload.UserID)
		}
	case payload.Username != "":
		user, appErr = p.API.GetUserByUsername(payload.Username)
		if appErr != nil {
			return errors.Wrapf(appErr, "failed to get user with username %s", payload.Username)
		}
	default:
		return errors.New("please provide either user_id or username in the request payload")
	}

	// Start team/channel removal process
	teamMembers, appErr := p.API.GetTeamMembersForUser(user.Id, 0, 1000)
	if appErr != nil {
		return errors.Wrapf(appErr, "failed to get team members for user. user=%s", user.Username)
	}

	for _, tm := range teamMembers {
		err = p.processTeamMember(user, tm.TeamId, requesterID)
		if err != nil {
			return errors.Wrapf(err, "failed to process team member. user=%s team=%s", user.Username, tm.TeamId)
		}
	}

	p.API.LogDebug("Finished for user.", "username", user.Username)

	return nil
}

func (p *Plugin) processTeamMember(user *model.User, teamID string, requesterID string) error {
	var appErr *model.AppError

	// Remove user from channels in this team
	channelMembers, appErr := p.API.GetChannelMembersForUser(user.Id, teamID, 0, 1000)
	if appErr != nil {
		return errors.Wrapf(appErr, "failed to get channel members")
	}

	for _, cm := range channelMembers {
		err := p.processChannelMember(user, cm.ChannelId)
		if err != nil {
			return errors.Wrapf(err, "failed to process channel member. channel=%s", cm.ChannelId)
		}
	}

	// Remove user from team
	appErr = p.API.DeleteTeamMember(teamID, user.Id, requesterID)
	if appErr != nil {
		return errors.Wrap(appErr, "failed to remove user from team")
	}

	p.API.LogDebug("Removed user from all channels in team.", "username", user.Username, "team", teamID)

	return nil
}

func (p *Plugin) processChannelMember(user *model.User, channelID string) error {
	// Remove user from channel
	appErr := p.API.DeleteChannelMember(channelID, user.Id)
	if appErr != nil {
		c, channelErr := p.API.GetChannel(channelID)
		if channelErr != nil {
			return errors.Wrapf(channelErr, "failed to get channel %s", channelID)
		}

		if c.Name == model.DefaultChannelName {
			return nil
		}

		return errors.Wrap(appErr, "failed to remove user from channel")
	}

	return nil
}
