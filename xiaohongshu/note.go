package xiaohongshu

import (
	"context"
	"strconv"
	"time"
)

// rawNoteFeed is the shape of the feed/note detail response.
type rawNoteFeed struct {
	Items []struct {
		ID       string  `json:"id"`
		NoteCard rawNote `json:"note_card"`
	} `json:"items"`
}

type rawNote struct {
	NoteID       string      `json:"note_id"`
	Type         string      `json:"type"`
	Title        string      `json:"title"`
	Desc         string      `json:"desc"`
	Time         int64       `json:"time"`
	LastUpdate   int64       `json:"last_update_time"`
	IPLocation   string      `json:"ip_location"`
	User         rawNoteUser `json:"user"`
	InteractInfo struct {
		LikedCount     string `json:"liked_count"`
		CollectedCount string `json:"collected_count"`
		CommentCount   string `json:"comment_count"`
		ShareCount     string `json:"share_count"`
	} `json:"interact_info"`
	ImageList []struct {
		URLDefault string `json:"url_default"`
		Width      int    `json:"width"`
		Height     int    `json:"height"`
		TraceID    string `json:"trace_id"`
		LivePhoto  bool   `json:"live_photo"`
		Stream     struct {
			H264 []struct {
				MasterURL string `json:"master_url"`
			} `json:"h264"`
		} `json:"stream"`
	} `json:"image_list"`
	Video   *rawVideo `json:"video"`
	TagList []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
		Type string `json:"type"`
	} `json:"tag_list"`
	AtUserList []struct {
		Nickname string `json:"nickname"`
	} `json:"at_user_list"`
}

type rawNoteUser struct {
	UserID   string `json:"user_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type rawVideo struct {
	Capa struct {
		Duration int `json:"duration"`
	} `json:"capa"`
	Image struct {
		FirstFrame  string `json:"first_frame_fileid"`
		ThumbFileid string `json:"thumbnail_fileid"`
	} `json:"image"`
	Media struct {
		VideoID int64 `json:"video_id"`
		Video   struct {
			MD5 string `json:"md5"`
		} `json:"video"`
		Stream struct {
			H264 []rawVideoStream `json:"h264"`
			H265 []rawVideoStream `json:"h265"`
		} `json:"stream"`
	} `json:"media"`
}

type rawVideoStream struct {
	MasterURL string `json:"master_url"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

// Note fetches a single note's detail. xsecToken is the per-note token harvested
// from a listing or feed; the detail endpoint refuses to open a note without it.
func (c *Client) Note(ctx context.Context, noteID, xsecToken string) (Note, error) {
	payload := map[string]any{
		"source_note_id": noteID,
		"image_formats":  []string{"jpg", "webp", "avif"},
		"extra":          map[string]bool{"need_body_topic": true},
		"xsec_source":    "pc_feed",
		"xsec_token":     xsecToken,
	}
	var feed rawNoteFeed
	if err := c.PostJSON(ctx, "/api/sns/web/v1/feed", payload, &feed); err != nil {
		return Note{}, err
	}
	if len(feed.Items) == 0 {
		return Note{}, apiError(0, "笔记不存在")
	}
	card := feed.Items[0].NoteCard
	if card.NoteID == "" {
		card.NoteID = noteID
	}
	return convertNote(card, xsecToken, c.now()), nil
}

func convertNote(r rawNote, xsecToken string, now time.Time) Note {
	n := Note{
		NoteID:         r.NoteID,
		Type:           r.Type,
		Title:          r.Title,
		Desc:           r.Desc,
		UserID:         r.User.UserID,
		Nickname:       r.User.Nickname,
		Avatar:         r.User.Avatar,
		LikedCount:     atoi(r.InteractInfo.LikedCount),
		CollectedCount: atoi(r.InteractInfo.CollectedCount),
		CommentCount:   atoi(r.InteractInfo.CommentCount),
		ShareCount:     atoi(r.InteractInfo.ShareCount),
		Time:           r.Time,
		LastUpdateTime: r.LastUpdate,
		IPLocation:     r.IPLocation,
		XsecToken:      xsecToken,
		URL:            "https://www.xiaohongshu.com/explore/" + r.NoteID,
		FetchedAt:      now.UTC().Format(time.RFC3339),
	}
	if r.Time > 0 {
		n.TimeText = time.UnixMilli(r.Time).UTC().Format(time.RFC3339)
	}
	for _, t := range r.TagList {
		if t.Name != "" {
			n.Tags = append(n.Tags, t.Name)
		}
	}
	for _, a := range r.AtUserList {
		if a.Nickname != "" {
			n.AtUsers = append(n.AtUsers, a.Nickname)
		}
	}
	for _, im := range r.ImageList {
		img := Image{URL: im.URLDefault, Width: im.Width, Height: im.Height, TraceID: im.TraceID, LivePhoto: im.LivePhoto}
		if len(im.Stream.H264) > 0 {
			img.StreamURL = im.Stream.H264[0].MasterURL
		}
		n.Images = append(n.Images, img)
	}
	if r.Video != nil {
		v := &Video{
			Duration: r.Video.Capa.Duration,
			MD5:      r.Video.Media.Video.MD5,
		}
		for _, s := range append(r.Video.Media.Stream.H264, r.Video.Media.Stream.H265...) {
			if s.MasterURL != "" {
				v.Masters = append(v.Masters, s.MasterURL)
				if v.Width == 0 {
					v.Width, v.Height = s.Width, s.Height
				}
			}
		}
		n.Video = v
	}
	return n
}

func atoi(s string) int64 {
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}
