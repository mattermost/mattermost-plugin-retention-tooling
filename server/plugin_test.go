package main

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost-server/v6/model"
	"github.com/mattermost/mattermost-server/v6/plugin/plugintest"
)

const deleteChannelMembersRoute = "/remove_user_from_all_teams_and_channels"

func TestServeHTTP(t *testing.T) {
	for name, tc := range map[string]struct {
		makeRequest    func(api *plugintest.API) *http.Request
		expectedStatus int
		expectedError  string
	}{
		"blank path": {
			makeRequest: func(_ *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/", nil)
				return r
			},
			expectedStatus: 404,
			expectedError:  "no handler for route /",
		},
		"invalid path": {
			makeRequest: func(_ *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodGet, "/invalid", nil)
				return r
			},
			expectedStatus: 404,
			expectedError:  "no handler for route /invalid",
		},
		"valid path, invalid http method": {
			makeRequest: func(_ *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodGet, deleteChannelMembersRoute, nil)
				return r
			},
			expectedStatus: 405,
			expectedError:  "unexpected HTTP method GET. Should be POST",
		},
		"missing user session": {
			makeRequest: func(_ *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, nil)
				return r
			},
			expectedStatus: 401,
			expectedError:  "request is not from an authenticated user",
		},
		"user not found": {
			makeRequest: func(api *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, nil)
				r.Header.Set("Mattermost-User-Id", "requesting_user_id")

				api.On("GetUser", "requesting_user_id").Return(nil, &model.AppError{DetailedError: "user not found"})

				return r
			},
			expectedStatus: 401,
			expectedError:  "error verifying whether user requesting_user_id is a system admin: failed to get user with id requesting_user_id: : , user not found",
		},
		"user is not sysadmin": {
			makeRequest: func(api *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, nil)
				r.Header.Set("Mattermost-User-Id", "requesting_user_id")

				api.On("GetUser", "requesting_user_id").Return(&model.User{
					Roles: "system_user",
				}, nil)

				return r
			},
			expectedStatus: 401,
			expectedError:  "user requesting_user_id is not a system admin",
		},
		"missing user info in request": {
			makeRequest: func(api *plugintest.API) *http.Request {
				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, nil)
				r.Header.Set("Mattermost-User-Id", "requesting_user_id")

				api.On("GetUser", "requesting_user_id").Return(&model.User{
					Roles: "system_user system_admin",
				}, nil)

				api.On("LogError", "error processing request: error decoding user info payload: EOF")

				return r
			},
			expectedStatus: 500,
			expectedError:  "error processing request: error decoding user info payload: EOF",
		},
		"user id provided in request": {
			makeRequest: func(api *plugintest.API) *http.Request {
				payload := Payload{
					UserID: "deactivated_user_id",
				}
				b, _ := json.Marshal(payload)

				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, bytes.NewReader(b))
				r.Header.Set("Mattermost-User-Id", "requesting_user_id")

				api.On("GetUser", "requesting_user_id").Return(&model.User{
					Roles: "system_user system_admin",
				}, nil)

				api.On("GetUser", "deactivated_user_id").Return(&model.User{
					Id:       "deactivated_user_id",
					Username: "deactivated_username",
				}, nil)

				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{}, nil)

				api.On("LogDebug", "Finished for user.", "username", "deactivated_username")
				return r
			},
			expectedStatus: 200,
			expectedError:  "",
		},
		"username provided in request": {
			makeRequest: func(api *plugintest.API) *http.Request {
				payload := Payload{
					UserID: "deactivated_user_id",
				}
				b, _ := json.Marshal(payload)

				r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, bytes.NewReader(b))
				r.Header.Set("Mattermost-User-Id", "requesting_user_id")

				api.On("GetUser", "requesting_user_id").Return(&model.User{
					Roles: "system_user system_admin",
				}, nil)

				api.On("GetUser", "deactivated_user_id").Return(&model.User{
					Id:       "deactivated_user_id",
					Username: "deactivated_username",
				}, nil)

				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{}, nil)

				api.On("LogDebug", "Finished for user.", "username", "deactivated_username")
				return r
			},
			expectedStatus: 200,
			expectedError:  "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := &Plugin{}
			api := &plugintest.API{}
			p.SetAPI(api)

			w := httptest.NewRecorder()
			r := tc.makeRequest(api)

			p.ServeHTTP(nil, w, r)

			result := w.Result()
			require.NotNil(t, result)
			defer result.Body.Close()
			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)

			require.Equal(t, result.Header.Get("Content-Type"), "application/json")
			require.Equal(t, tc.expectedStatus, result.StatusCode)

			var errResponse ErrorResponse
			err = json.Unmarshal(bodyBytes, &errResponse)
			require.NoError(t, err)
			if tc.expectedError != "" {
				require.Equal(t, tc.expectedError, errResponse.Error)
			} else {
				require.Equal(t, "", errResponse.Error)
			}
		})
	}
}

func TestHandleRemoveUserFromAllTeamsAndChannels(t *testing.T) {
	for name, tc := range map[string]struct {
		runAssertions  func(api *plugintest.API)
		expectedStatus int
		expectedError  string
	}{
		"happy path, user is member of one team": {
			runAssertions: func(api *plugintest.API) {
				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{{
					TeamId: "teamid1",
					UserId: "deactivated_user_id",
				}}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid1", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid1",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid2",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid3",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("DeleteChannelMember", "channelid1", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid2", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid3", "deactivated_user_id").Return(nil)

				api.On("DeleteTeamMember", "teamid1", "deactivated_user_id", "requesting_user_id").Return(nil)

				api.On("LogDebug", "Removed user from all channels in team.", "username", "deactivated_username", "team", "teamid1")
				api.On("LogDebug", "Finished for user.", "username", "deactivated_username")
			},
			expectedStatus: 200,
			expectedError:  "",
		},
		"happy path, user is member of two teams": {
			runAssertions: func(api *plugintest.API) {
				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{
					{
						TeamId: "teamid1",
						UserId: "deactivated_user_id",
					}, {
						TeamId: "teamid2",
						UserId: "deactivated_user_id",
					},
				}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid1", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid1",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid2",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid3",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid2", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid4",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid5",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid6",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("DeleteChannelMember", "channelid1", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid2", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid3", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid4", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid5", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid6", "deactivated_user_id").Return(nil)

				api.On("DeleteTeamMember", "teamid1", "deactivated_user_id", "requesting_user_id").Return(nil)
				api.On("DeleteTeamMember", "teamid2", "deactivated_user_id", "requesting_user_id").Return(nil)

				api.On("LogDebug", "Removed user from all channels in team.", "username", "deactivated_username", "team", "teamid1")
				api.On("LogDebug", "Removed user from all channels in team.", "username", "deactivated_username", "team", "teamid2")
				api.On("LogDebug", "Finished for user.", "username", "deactivated_username")
			},
			expectedStatus: 200,
			expectedError:  "",
		},
		"error deleting team member": {
			runAssertions: func(api *plugintest.API) {
				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{
					{
						TeamId: "teamid1",
						UserId: "deactivated_user_id",
					},
				}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid1", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid1",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid2",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid3",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("DeleteChannelMember", "channelid1", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid2", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid3", "deactivated_user_id").Return(nil)

				api.On("DeleteTeamMember", "teamid1", "deactivated_user_id", "requesting_user_id").Return(&model.AppError{DetailedError: "some database error"})

				api.On("LogError", "error processing request: failed to process team member. user=deactivated_username team=teamid1: failed to remove user from team: : , some database error")
			},
			expectedStatus: 500,
			expectedError:  "error processing request: failed to process team member. user=deactivated_username team=teamid1: failed to remove user from team: : , some database error",
		},
		"error deleting channel member": {
			runAssertions: func(api *plugintest.API) {
				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{
					{
						TeamId: "teamid1",
						UserId: "deactivated_user_id",
					},
				}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid1", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid1",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid2",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid3",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("DeleteChannelMember", "channelid1", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid2", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid3", "deactivated_user_id").Return(&model.AppError{DetailedError: "some database error"})

				api.On("GetChannel", "channelid3").Return(&model.Channel{Name: "channelname3"}, nil)

				api.On("LogError", "error processing request: failed to process team member. user=deactivated_username team=teamid1: failed to process channel member. channel=channelid3: failed to remove user from channel: : , some database error")
			},
			expectedStatus: 500,
			expectedError:  "error processing request: failed to process team member. user=deactivated_username team=teamid1: failed to process channel member. channel=channelid3: failed to remove user from channel: : , some database error",
		},
		"handle town square case": {
			runAssertions: func(api *plugintest.API) {
				api.On("GetTeamMembersForUser", "deactivated_user_id", 0, 1000).Return([]*model.TeamMember{
					{
						TeamId: "teamid1",
						UserId: "deactivated_user_id",
					},
				}, nil)

				api.On("GetChannelMembersForUser", "deactivated_user_id", "teamid1", 0, 1000).Return([]*model.ChannelMember{
					{
						ChannelId: "channelid1",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid2",
						UserId:    "deactivated_user_id",
					},
					{
						ChannelId: "channelid3",
						UserId:    "deactivated_user_id",
					},
				}, nil)

				api.On("DeleteChannelMember", "channelid1", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid2", "deactivated_user_id").Return(nil)
				api.On("DeleteChannelMember", "channelid3", "deactivated_user_id").Return(&model.AppError{DetailedError: "some database error"})

				api.On("GetChannel", "channelid3").Return(&model.Channel{Name: "town-square"}, nil)

				api.On("DeleteTeamMember", "teamid1", "deactivated_user_id", "requesting_user_id").Return(nil)

				api.On("LogDebug", "Removed user from all channels in team.", "username", "deactivated_username", "team", "teamid1")
				api.On("LogDebug", "Finished for user.", "username", "deactivated_username")
			},
			expectedStatus: 200,
			expectedError:  "",
		},
	} {
		t.Run(name, func(t *testing.T) {
			p := &Plugin{}
			api := &plugintest.API{}
			p.SetAPI(api)

			w := httptest.NewRecorder()

			payload := Payload{
				Username: "deactivated_username",
			}
			b, _ := json.Marshal(payload)

			r := httptest.NewRequest(http.MethodPost, deleteChannelMembersRoute, bytes.NewReader(b))
			r.Header.Set("Mattermost-User-Id", "requesting_user_id")

			api.On("GetUser", "requesting_user_id").Return(&model.User{
				Roles: "system_user system_admin",
			}, nil)

			api.On("GetUserByUsername", "deactivated_username").Return(&model.User{
				Id:       "deactivated_user_id",
				Username: "deactivated_username",
			}, nil)

			tc.runAssertions(api)

			p.ServeHTTP(nil, w, r)

			result := w.Result()
			require.NotNil(t, result)
			defer result.Body.Close()
			bodyBytes, err := io.ReadAll(result.Body)
			require.NoError(t, err)

			require.Equal(t, result.Header.Get("Content-Type"), "application/json")
			require.Equal(t, tc.expectedStatus, result.StatusCode)

			var errResponse ErrorResponse
			err = json.Unmarshal(bodyBytes, &errResponse)
			require.NoError(t, err)
			if tc.expectedError != "" {
				require.Equal(t, tc.expectedError, errResponse.Error)
			} else {
				require.Equal(t, "", errResponse.Error)
			}
		})
	}
}
