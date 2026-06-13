package xiaohongshu

import (
	"context"
	"iter"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
)

type rawHomefeed struct {
	Items []struct {
		ID        string `json:"id"`
		ModelType string `json:"model_type"`
		NoteCard  struct {
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
		} `json:"note_card"`
	} `json:"items"`
	Cursor string `json:"cursor_score"`
}

// feedCategories are the homefeed channel ids the web client offers.
var feedCategories = map[string]string{
	"recommend": "homefeed_recommend",
	"fashion":   "homefeed.fashion_v3",
	"food":      "homefeed.food_v3",
	"cosmetics": "homefeed.cosmetics_v3",
	"movie":     "homefeed.movie_and_tv_v3",
	"career":    "homefeed.career_v3",
	"love":      "homefeed.love_v3",
	"household": "homefeed.household_product_v3",
	"gaming":    "homefeed.gaming_v3",
	"travel":    "homefeed.travel_v3",
	"fitness":   "homefeed.fitness_v3",
}

// FeedCategories lists the available homefeed channel names.
func FeedCategories() []string {
	out := make([]string, 0, len(feedCategories))
	for k := range feedCategories {
		out = append(out, k)
	}
	return out
}

// Feed streams the recommendation homefeed for a category. An unknown category
// name falls back to the recommend channel. limit caps the items yielded.
func (c *Client) Feed(ctx context.Context, category string, limit int) iter.Seq2[FeedItem, error] {
	return func(yield func(FeedItem, error) bool) {
		channel, ok := feedCategories[category]
		if !ok {
			channel = feedCategories["recommend"]
		}
		cursor := ""
		count := 0
		refresh := 1
		for {
			payload := map[string]any{
				"cursor_score":         cursor,
				"num":                  35,
				"refresh_type":         refresh,
				"note_index":           count,
				"unread_begin_note_id": "",
				"unread_end_note_id":   "",
				"unread_note_count":    0,
				"category":             channel,
				"search_key":           "",
				"need_num":             10,
				"image_formats":        []string{"jpg", "webp", "avif"},
			}
			var res rawHomefeed
			if err := c.PostJSON(ctx, "/api/sns/web/v1/homefeed", payload, &res); err != nil {
				yield(FeedItem{}, err)
				return
			}
			if len(res.Items) == 0 {
				return
			}
			for _, it := range res.Items {
				nc := it.NoteCard
				if nc.NoteID == "" {
					continue
				}
				fi := FeedItem{
					NoteID:     nc.NoteID,
					Type:       nc.Type,
					Title:      nc.DisplayTitle,
					UserID:     nc.User.UserID,
					Nickname:   nc.User.Nickname,
					Cover:      nc.Cover.URLDefault,
					LikedCount: atoi(nc.InteractInfo.LikedCount),
					XsecToken:  nc.XsecToken,
					URL:        xhsurl.NoteURL(nc.NoteID, nc.XsecToken),
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
			cursor = res.Cursor
			refresh = 3
			if cursor == "" {
				return
			}
		}
	}
}

type rawRelated struct {
	Items []struct {
		ID       string `json:"id"`
		NoteCard struct {
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
		} `json:"note_card"`
	} `json:"items"`
}

// Related returns notes recommended alongside a given note.
func (c *Client) Related(ctx context.Context, noteID, xsecToken string, limit int) ([]FeedItem, error) {
	params := map[string]string{
		"note_id":       noteID,
		"xsec_token":    xsecToken,
		"image_formats": "jpg,webp,avif",
	}
	var res rawRelated
	if err := c.GetJSON(ctx, "/api/sns/web/v1/note/related", params, &res); err != nil {
		return nil, err
	}
	out := make([]FeedItem, 0, len(res.Items))
	for _, it := range res.Items {
		nc := it.NoteCard
		if nc.NoteID == "" {
			continue
		}
		out = append(out, FeedItem{
			NoteID:     nc.NoteID,
			Type:       nc.Type,
			Title:      nc.DisplayTitle,
			UserID:     nc.User.UserID,
			Nickname:   nc.User.Nickname,
			Cover:      nc.Cover.URLDefault,
			LikedCount: atoi(nc.InteractInfo.LikedCount),
			XsecToken:  nc.XsecToken,
			URL:        xhsurl.NoteURL(nc.NoteID, nc.XsecToken),
			FetchedAt:  c.now().UTC().Format(time.RFC3339),
		})
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}
