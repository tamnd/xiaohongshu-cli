package xiaohongshu

import (
	"context"
	"time"
)

type rawTagPage struct {
	PageInfo struct {
		Name    string `json:"name"`
		ID      string `json:"id"`
		Type    string `json:"type"`
		ViewNum int64  `json:"view_num"`
		FansNum int64  `json:"fans_num"`
	} `json:"page_info"`
}

// Tag fetches a topic page by keyword. The endpoint returns the canonical topic
// name, id, and view count, which is enough to build the topic link.
func (c *Client) Tag(ctx context.Context, keyword string) (Tag, error) {
	payload := map[string]any{
		"keyword":               keyword,
		"suggest_topic_request": map[string]any{"keyword": keyword, "page": map[string]int{"page_size": 20, "page": 1}},
	}
	var raw rawTagPage
	if err := c.PostJSON(ctx, "/api/sns/web/v1/search/topics", payload, &raw); err != nil {
		return Tag{}, err
	}
	t := Tag{
		ID:        raw.PageInfo.ID,
		Name:      raw.PageInfo.Name,
		Type:      raw.PageInfo.Type,
		ViewNum:   raw.PageInfo.ViewNum,
		FetchedAt: c.now().UTC().Format(time.RFC3339),
	}
	if t.Name == "" {
		t.Name = keyword
	}
	if t.ID != "" {
		t.Link = "https://www.xiaohongshu.com/page/topics/" + t.ID
	}
	return t, nil
}
