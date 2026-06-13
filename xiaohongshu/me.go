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
func (c *Client) Me(ctx context.Context) (Me, error) {
	var raw rawMe
	if err := c.GetJSON(ctx, "/api/sns/web/v2/user/me", nil, &raw); err != nil {
		return Me{}, err
	}
	return Me{
		LoggedIn: !raw.GUest && raw.UserID != "",
		UserID:   raw.UserID,
		Nickname: raw.Nickname,
		RedID:    raw.RedID,
	}, nil
}
