package xiaohongshu

import (
	"context"
	"iter"
	"strings"
	"time"

	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
)

type rawSearchNotes struct {
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
		UserInfo struct {
			UserID    string `json:"id"`
			Nickname  string `json:"name"`
			RedID     string `json:"red_id"`
			Image     string `json:"image"`
			Fans      string `json:"fans"`
			NoteCount string `json:"note_count"`
		} `json:"user"`
	} `json:"items"`
	HasMore bool `json:"has_more"`
}

// searchID is the per-search id required by the search endpoint. It must look
// like a 21-char base-36 token; the exact value does not matter to the server as
// long as it is stable across a paged search, so it is derived from the query
// and the page is varied by the caller.
func searchID(seed string) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789"
	h := crc32Seed(seed)
	var sb strings.Builder
	for range 21 {
		sb.WriteByte(alphabet[h%uint32(len(alphabet))])
		h = h*1103515245 + 12345
	}
	return sb.String()
}

func crc32Seed(s string) uint32 {
	var h uint32 = 2166136261
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= 16777619
	}
	if h == 0 {
		h = 1
	}
	return h
}

// SearchNotes streams note search results for a keyword, following pages until
// the stream is exhausted or limit results are yielded.
func (c *Client) SearchNotes(ctx context.Context, keyword string, limit int) iter.Seq2[SearchResult, error] {
	return func(yield func(SearchResult, error) bool) {
		sid := searchID(keyword)
		count := 0
		for page := 1; ; page++ {
			payload := map[string]any{
				"keyword":       keyword,
				"page":          page,
				"page_size":     20,
				"search_id":     sid,
				"sort":          "general",
				"note_type":     0,
				"image_formats": []string{"jpg", "webp", "avif"},
			}
			var res rawSearchNotes
			if err := c.PostJSON(ctx, "/api/sns/web/v1/search/notes", payload, &res); err != nil {
				yield(SearchResult{}, err)
				return
			}
			yielded := 0
			for _, it := range res.Items {
				if it.NoteCard.NoteID == "" {
					continue
				}
				nc := it.NoteCard
				note := Note{
					NoteID:     nc.NoteID,
					Type:       nc.Type,
					Title:      nc.DisplayTitle,
					UserID:     nc.User.UserID,
					Nickname:   nc.User.Nickname,
					Avatar:     nc.User.Avatar,
					LikedCount: atoi(nc.InteractInfo.LikedCount),
					XsecToken:  nc.XsecToken,
					URL:        xhsurl.NoteURL(nc.NoteID, nc.XsecToken),
					FetchedAt:  c.now().UTC().Format(time.RFC3339),
				}
				if len(nc.Cover.URLDefault) > 0 {
					note.Images = []Image{{URL: nc.Cover.URLDefault}}
				}
				if !yield(SearchResult{ModelType: "note", Note: &note}, nil) {
					return
				}
				yielded++
				count++
				if limit > 0 && count >= limit {
					return
				}
			}
			if !res.HasMore || yielded == 0 {
				return
			}
		}
	}
}

// SearchUsers streams user search results for a keyword.
func (c *Client) SearchUsers(ctx context.Context, keyword string, limit int) iter.Seq2[SearchResult, error] {
	return func(yield func(SearchResult, error) bool) {
		sid := searchID(keyword)
		count := 0
		for page := 1; ; page++ {
			payload := map[string]any{
				"search_user_request": map[string]any{
					"keyword":    keyword,
					"page":       page,
					"page_size":  15,
					"search_id":  sid,
					"biz_type":   "web_search_user",
					"request_id": sid,
				},
			}
			var res rawSearchNotes
			if err := c.PostJSON(ctx, "/api/sns/web/v1/search/usersearch", payload, &res); err != nil {
				yield(SearchResult{}, err)
				return
			}
			yielded := 0
			for _, it := range res.Items {
				if it.UserInfo.UserID == "" {
					continue
				}
				u := User{
					UserID:    it.UserInfo.UserID,
					Nickname:  it.UserInfo.Nickname,
					RedID:     it.UserInfo.RedID,
					Avatar:    it.UserInfo.Image,
					Fans:      atoi(it.UserInfo.Fans),
					NoteCount: atoi(it.UserInfo.NoteCount),
					URL:       xhsurl.UserURL(it.UserInfo.UserID),
					FetchedAt: c.now().UTC().Format(time.RFC3339),
				}
				if !yield(SearchResult{ModelType: "user", User: &u}, nil) {
					return
				}
				yielded++
				count++
				if limit > 0 && count >= limit {
					return
				}
			}
			if !res.HasMore || yielded == 0 {
				return
			}
		}
	}
}

type rawSuggest struct {
	SugItems []struct {
		Text       string `json:"text"`
		SearchType string `json:"search_type"`
	} `json:"sug_items"`
}

// Suggest returns search autocomplete suggestions for a partial keyword.
func (c *Client) Suggest(ctx context.Context, keyword string) ([]string, error) {
	params := map[string]string{"keyword": keyword}
	var raw rawSuggest
	if err := c.GetJSON(ctx, "/api/sns/web/v1/search/recommend", params, &raw); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(raw.SugItems))
	for _, s := range raw.SugItems {
		if s.Text != "" {
			out = append(out, s.Text)
		}
	}
	return out, nil
}
