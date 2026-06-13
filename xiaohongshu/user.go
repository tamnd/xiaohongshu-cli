package xiaohongshu

import (
	"context"
	"iter"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
)

type rawUserInfo struct {
	BasicInfo struct {
		Nickname   string `json:"nickname"`
		RedID      string `json:"red_id"`
		Desc       string `json:"desc"`
		Gender     int    `json:"gender"`
		Images     string `json:"images"`
		IPLocation string `json:"ip_location"`
	} `json:"basic_info"`
	Interactions []struct {
		Type  string `json:"type"`
		Name  string `json:"name"`
		Count string `json:"count"`
	} `json:"interactions"`
	Tags []struct {
		Name string `json:"name"`
	} `json:"tags"`
}

// User fetches a creator's profile by user id.
func (c *Client) User(ctx context.Context, userID string) (User, error) {
	var raw rawUserInfo
	params := map[string]string{"target_user_id": userID}
	if err := c.GetJSON(ctx, "/api/sns/web/v1/user/otherinfo", params, &raw); err != nil {
		return User{}, err
	}
	u := User{
		UserID:     userID,
		Nickname:   raw.BasicInfo.Nickname,
		RedID:      raw.BasicInfo.RedID,
		Desc:       raw.BasicInfo.Desc,
		Gender:     gender(raw.BasicInfo.Gender),
		Avatar:     raw.BasicInfo.Images,
		IPLocation: raw.BasicInfo.IPLocation,
		URL:        "https://www.xiaohongshu.com/user/profile/" + userID,
		FetchedAt:  c.now().UTC().Format(time.RFC3339),
	}
	for _, in := range raw.Interactions {
		switch in.Type {
		case "follows":
			u.Follows = atoi(in.Count)
		case "fans":
			u.Fans = atoi(in.Count)
		case "interaction":
			u.Interaction = atoi(in.Count)
		}
	}
	for _, t := range raw.Tags {
		if t.Name != "" {
			u.Tags = append(u.Tags, t.Name)
		}
	}
	return u, nil
}

func gender(g int) string {
	switch g {
	case 0:
		return "male"
	case 1:
		return "female"
	default:
		return ""
	}
}

type rawUserPosted struct {
	Notes []struct {
		NoteID       string      `json:"note_id"`
		Type         string      `json:"type"`
		DisplayTitle string      `json:"display_title"`
		XsecToken    string      `json:"xsec_token"`
		User         rawNoteUser `json:"user"`
		Cover        struct {
			URLDefault string `json:"url_default"`
		} `json:"cover"`
		InteractInfo struct {
			LikedCount string `json:"liked_count"`
		} `json:"interact_info"`
	} `json:"notes"`
	Cursor  string `json:"cursor"`
	HasMore bool   `json:"has_more"`
}

// UserNotes streams a creator's posted notes, following the cursor. It stops
// after the stream is exhausted or limit items are yielded; limit <= 0 means no
// cap. Each error stops the stream.
func (c *Client) UserNotes(ctx context.Context, userID string, limit int) iter.Seq2[FeedItem, error] {
	return func(yield func(FeedItem, error) bool) {
		cursor := ""
		count := 0
		for {
			params := map[string]string{
				"num":           "30",
				"cursor":        cursor,
				"user_id":       userID,
				"image_formats": "jpg,webp,avif",
			}
			var page rawUserPosted
			if err := c.GetJSON(ctx, "/api/sns/web/v1/user_posted", params, &page); err != nil {
				yield(FeedItem{}, err)
				return
			}
			for _, n := range page.Notes {
				fi := FeedItem{
					NoteID:     n.NoteID,
					Type:       n.Type,
					Title:      n.DisplayTitle,
					UserID:     n.User.UserID,
					Nickname:   n.User.Nickname,
					Cover:      n.Cover.URLDefault,
					LikedCount: atoi(n.InteractInfo.LikedCount),
					XsecToken:  n.XsecToken,
					URL:        xhsurl.NoteURL(n.NoteID, n.XsecToken),
					FetchedAt:  c.now().UTC().Format(time.RFC3339),
				}
				if !yield(fi, nil) {
					return
				}
				count++
				if limit > 0 && count >= limit {
					return
				}
			}
			if !page.HasMore || page.Cursor == "" || len(page.Notes) == 0 {
				return
			}
			cursor = page.Cursor
		}
	}
}
