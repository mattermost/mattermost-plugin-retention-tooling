package main

import (
	"strings"

	"github.com/pkg/errors"

	"github.com/mattermost/mattermost/server/public/model"
)

func (p *Plugin) ensureSystemAdmin(userID string) (bool, error) {
	user, appErr := p.API.GetUser(userID)
	if appErr != nil {
		return false, errors.Wrapf(appErr, "failed to get user with id %s", userID)
	}

	if !strings.Contains(user.Roles, model.SystemAdminRoleId) {
		return false, nil
	}

	return true, nil
}

// CutPrefix returns s without the provided leading prefix string
// and reports whether it found the prefix.
// If s doesn't start with prefix, CutPrefix returns s, false.
// If prefix is the empty string, CutPrefix returns s, true.
func CutPrefix(s, prefix string) (after string, found bool) {
	if !strings.HasPrefix(s, prefix) {
		return s, false
	}
	return s[len(prefix):], true
}
