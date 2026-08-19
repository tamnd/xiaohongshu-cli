package xiaohongshu

import "context"

type rawMe struct {
	GUest    bool   `json:"guest"`
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	RedID    string `json:"red_id"`
}

// Me reports the login state of the configured cookie. An anonymous session
// comes back as guest; a logged-in cookie carries the account's id and handle.
//
// A login refusal is this command's answer rather than its failure. Everywhere
// else in the tool a -101 means the caller asked for something they cannot have,
// but the question here is whether they are logged in, and "you are not" is a
// complete and correct reply. Reporting it as an error made the one command that
// exists to diagnose a missing cookie fail whenever the cookie was missing.
func (c *Client) Me(ctx context.Context) (Me, error) {
	var raw rawMe
	if err := c.GetJSON(ctx, "/api/sns/web/v2/user/me", nil, &raw); err != nil {
		if StatusOf(err) == StatusLogin {
			return Me{}, nil
		}
		return Me{}, err
	}
	return Me{
		LoggedIn: !raw.GUest && raw.UserID != "",
		UserID:   raw.UserID,
		Nickname: raw.Nickname,
		RedID:    raw.RedID,
	}, nil
}
