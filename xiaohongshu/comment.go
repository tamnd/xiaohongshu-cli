package xiaohongshu

import (
	"context"
	"iter"
	"time"
)

type rawCommentPage struct {
	Comments []rawComment `json:"comments"`
	Cursor   string       `json:"cursor"`
	HasMore  bool         `json:"has_more"`
}

type rawComment struct {
	ID         string `json:"id"`
	NoteID     string `json:"note_id"`
	Content    string `json:"content"`
	LikeCount  string `json:"like_count"`
	CreateTime int64  `json:"create_time"`
	IPLocation string `json:"ip_location"`
	Status     int    `json:"status"`
	UserInfo   struct {
		UserID   string `json:"user_id"`
		Nickname string `json:"nickname"`
		Image    string `json:"image"`
	} `json:"user_info"`
	AtUsers []struct {
		Nickname string `json:"nickname"`
	} `json:"at_users"`
	SubCommentCount   string       `json:"sub_comment_count"`
	SubCommentCursor  string       `json:"sub_comment_cursor"`
	SubCommentHasMore bool         `json:"sub_comment_has_more"`
	SubComments       []rawComment `json:"sub_comments"`
}

// Comments streams top-level comments for a note. When deep is true it also
// follows each comment's replies. limit caps the number of top-level comments
// yielded; limit <= 0 means no cap.
func (c *Client) Comments(ctx context.Context, noteID, xsecToken string, deep bool, limit int) iter.Seq2[Comment, error] {
	return func(yield func(Comment, error) bool) {
		cursor := ""
		count := 0
		for {
			params := map[string]string{
				"note_id":        noteID,
				"cursor":         cursor,
				"top_comment_id": "",
				"image_formats":  "jpg,webp,avif",
				"xsec_token":     xsecToken,
			}
			var page rawCommentPage
			if err := c.GetJSON(ctx, "/api/sns/web/v2/comment/page", params, &page); err != nil {
				yield(Comment{}, err)
				return
			}
			for _, rc := range page.Comments {
				cm := convertComment(rc, noteID, c.now())
				if deep && rc.SubCommentCount != "" && atoi(rc.SubCommentCount) > int64(len(cm.SubComments)) {
					subs, err := c.subComments(ctx, noteID, rc.ID, rc.SubCommentCursor, xsecToken)
					if err != nil {
						yield(Comment{}, err)
						return
					}
					cm.SubComments = append(cm.SubComments, subs...)
				}
				if !yield(cm, nil) {
					return
				}
				count++
				if limit > 0 && count >= limit {
					return
				}
			}
			if !page.HasMore || page.Cursor == "" || len(page.Comments) == 0 {
				return
			}
			cursor = page.Cursor
		}
	}
}

func (c *Client) subComments(ctx context.Context, noteID, rootID, startCursor, xsecToken string) ([]Comment, error) {
	var out []Comment
	cursor := startCursor
	for {
		params := map[string]string{
			"note_id":         noteID,
			"root_comment_id": rootID,
			"num":             "10",
			"cursor":          cursor,
			"image_formats":   "jpg,webp,avif",
			"top_comment_id":  "",
			"xsec_token":      xsecToken,
		}
		var page rawCommentPage
		if err := c.GetJSON(ctx, "/api/sns/web/v2/comment/sub/page", params, &page); err != nil {
			return out, err
		}
		for _, rc := range page.Comments {
			out = append(out, convertComment(rc, noteID, c.now()))
		}
		if !page.HasMore || page.Cursor == "" || len(page.Comments) == 0 {
			return out, nil
		}
		cursor = page.Cursor
	}
}

func convertComment(rc rawComment, noteID string, now time.Time) Comment {
	cm := Comment{
		CommentID:       rc.ID,
		NoteID:          noteID,
		Content:         rc.Content,
		UserID:          rc.UserInfo.UserID,
		Nickname:        rc.UserInfo.Nickname,
		Avatar:          rc.UserInfo.Image,
		LikeCount:       atoi(rc.LikeCount),
		SubCommentCount: atoi(rc.SubCommentCount),
		IPLocation:      rc.IPLocation,
		CreateTime:      rc.CreateTime,
		Status:          rc.Status,
		FetchedAt:       now.UTC().Format(time.RFC3339),
	}
	for _, a := range rc.AtUsers {
		if a.Nickname != "" {
			cm.AtUsers = append(cm.AtUsers, a.Nickname)
		}
	}
	for _, sub := range rc.SubComments {
		cm.SubComments = append(cm.SubComments, convertComment(sub, noteID, now))
	}
	return cm
}
