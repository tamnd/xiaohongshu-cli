package xiaohongshu

import (
	"context"
	"encoding/json"
	"iter"
	"strconv"
	"strings"
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

// ssrUserState is the slice of __INITIAL_STATE__ that carries a profile page.
type ssrUserState struct {
	User struct {
		UserPageData struct {
			BasicInfo struct {
				Nickname   string   `json:"nickname"`
				RedID      string   `json:"redId"`
				Desc       string   `json:"desc"`
				Gender     looseInt `json:"gender"`
				Images     string   `json:"images"`
				IPLocation string   `json:"ipLocation"`
			} `json:"basicInfo"`
			Interactions []struct {
				Type  string `json:"type"`
				Name  string `json:"name"`
				Count string `json:"count"`
			} `json:"interactions"`
			Tags []struct {
				Name string `json:"name"`
			} `json:"tags"`
		} `json:"userPageData"`
		Notes [][]ssrFeedItem `json:"notes"`
	} `json:"user"`
}

// looseInt accepts a JSON number, a quoted number, or a non-numeric string
// (which it ignores), so a field whose type varies across pages never fails the
// whole decode.
type looseInt int64

func (n *looseInt) UnmarshalJSON(b []byte) error {
	s := strings.Trim(string(b), `"`)
	if s == "" || s == "null" {
		return nil
	}
	if v, err := strconv.ParseInt(s, 10, 64); err == nil {
		*n = looseInt(v)
	}
	return nil
}

// userState fetches and decodes a profile's server-rendered page once. Repeated
// calls for the same id are served from the on-disk cache.
func (c *Client) userState(ctx context.Context, userID string) (ssrUserState, error) {
	st, err := c.getState(ctx, "/user/profile/"+userID, nil, false)
	if err != nil {
		return ssrUserState{}, err
	}
	var s ssrUserState
	if err := json.Unmarshal(st, &s); err != nil {
		return ssrUserState{}, apiError(0, "用户不存在")
	}
	return s, nil
}

// User fetches a creator's profile by user id. It reads the server-rendered
// profile page, which carries the profile anonymously, and falls back to the
// signed API only when a logged-in cookie is configured.
func (c *Client) User(ctx context.Context, userID string) (User, error) {
	s, err := c.userState(ctx, userID)
	if err != nil {
		if c.LoggedIn() {
			return c.userAPI(ctx, userID)
		}
		return User{}, err
	}
	d := s.User.UserPageData
	if d.BasicInfo.Nickname == "" {
		if c.LoggedIn() {
			return c.userAPI(ctx, userID)
		}
		return User{}, apiError(0, "用户不存在")
	}
	u := User{
		UserID:     userID,
		Nickname:   d.BasicInfo.Nickname,
		RedID:      d.BasicInfo.RedID,
		Desc:       d.BasicInfo.Desc,
		Gender:     gender(int(d.BasicInfo.Gender)),
		Avatar:     d.BasicInfo.Images,
		IPLocation: d.BasicInfo.IPLocation,
		URL:        "https://www.xiaohongshu.com/user/profile/" + userID,
		FetchedAt:  c.now().UTC().Format(time.RFC3339),
	}
	for _, in := range d.Interactions {
		switch in.Type {
		case "follows":
			u.Follows = humanCount(in.Count)
		case "fans":
			u.Fans = humanCount(in.Count)
		case "interaction":
			u.Interaction = humanCount(in.Count)
		}
	}
	for _, t := range d.Tags {
		if t.Name != "" {
			u.Tags = append(u.Tags, t.Name)
		}
	}
	return u, nil
}

// userAPI fetches a creator's profile over the signed JSON API.
func (c *Client) userAPI(ctx context.Context, userID string) (User, error) {
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

// UserNotes streams a creator's posted notes. Without a cookie it yields the
// notes embedded in the server-rendered profile page (the first page the grid
// shows). With a logged-in cookie it follows the signed API's cursor to walk the
// full archive. limit <= 0 means no cap; each error stops the stream.
func (c *Client) UserNotes(ctx context.Context, userID string, limit int) iter.Seq2[FeedItem, error] {
	if c.LoggedIn() {
		return c.userNotesAPI(ctx, userID, limit)
	}
	return func(yield func(FeedItem, error) bool) {
		s, err := c.userState(ctx, userID)
		if err != nil {
			yield(FeedItem{}, err)
			return
		}
		count := 0
		seen := map[string]bool{}
		for _, grp := range s.User.Notes {
			for _, it := range grp {
				fi := c.feedItem(it)
				if fi.NoteID == "" || seen[fi.NoteID] {
					continue
				}
				seen[fi.NoteID] = true
				if !yield(fi, nil) {
					return
				}
				count++
				if limit > 0 && count >= limit {
					return
				}
			}
		}
	}
}

// userNotesAPI streams a creator's posted notes over the signed API cursor.
func (c *Client) userNotesAPI(ctx context.Context, userID string, limit int) iter.Seq2[FeedItem, error] {
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
