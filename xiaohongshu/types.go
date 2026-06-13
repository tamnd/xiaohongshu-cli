package xiaohongshu

// Note is the central record: one post, either an image note or a video note.
type Note struct {
	NoteID         string   `json:"note_id"`
	Type           string   `json:"type"` // "normal" (image) or "video"
	Title          string   `json:"title"`
	Desc           string   `json:"desc"`
	UserID         string   `json:"user_id"`
	Nickname       string   `json:"nickname"`
	Avatar         string   `json:"avatar,omitempty"`
	LikedCount     int64    `json:"liked_count"`
	CollectedCount int64    `json:"collected_count"`
	CommentCount   int64    `json:"comment_count"`
	ShareCount     int64    `json:"share_count"`
	Time           int64    `json:"time"`
	TimeText       string   `json:"time_text,omitempty"`
	LastUpdateTime int64    `json:"last_update_time,omitempty"`
	IPLocation     string   `json:"ip_location,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	AtUsers        []string `json:"at_users,omitempty"`
	Images         []Image  `json:"images,omitempty"`
	Video          *Video   `json:"video,omitempty"`
	XsecToken      string   `json:"xsec_token,omitempty"`
	URL            string   `json:"url"`
	FetchedAt      string   `json:"fetched_at"`
}

// Image is one image in an image note.
type Image struct {
	URL       string `json:"url"`
	Width     int    `json:"width,omitempty"`
	Height    int    `json:"height,omitempty"`
	TraceID   string `json:"trace_id,omitempty"`
	LivePhoto bool   `json:"live_photo,omitempty"`
	StreamURL string `json:"stream_url,omitempty"`
}

// Video is the video payload of a video note.
type Video struct {
	Duration int      `json:"duration_seconds,omitempty"`
	Width    int      `json:"width,omitempty"`
	Height   int      `json:"height,omitempty"`
	Cover    string   `json:"cover,omitempty"`
	MD5      string   `json:"md5,omitempty"`
	Masters  []string `json:"masters,omitempty"` // playable stream urls
}

// User is a creator's profile and stats.
type User struct {
	UserID         string   `json:"user_id"`
	Nickname       string   `json:"nickname"`
	RedID          string   `json:"red_id,omitempty"` // the public handle
	Desc           string   `json:"desc,omitempty"`
	Gender         string   `json:"gender,omitempty"`
	Avatar         string   `json:"avatar,omitempty"`
	IPLocation     string   `json:"ip_location,omitempty"`
	Follows        int64    `json:"follows"`
	Fans           int64    `json:"fans"`
	Interaction    int64    `json:"interaction"` // total likes + collects
	NoteCount      int64    `json:"note_count"`
	CollectedCount int64    `json:"collected_count,omitempty"`
	Tags           []string `json:"tags,omitempty"`
	URL            string   `json:"url"`
	FetchedAt      string   `json:"fetched_at"`
}

// Comment is one comment, with nested replies when expanded.
type Comment struct {
	CommentID       string    `json:"comment_id"`
	NoteID          string    `json:"note_id"`
	Content         string    `json:"content"`
	UserID          string    `json:"user_id"`
	Nickname        string    `json:"nickname"`
	Avatar          string    `json:"avatar,omitempty"`
	LikeCount       int64     `json:"like_count"`
	SubCommentCount int64     `json:"sub_comment_count"`
	AtUsers         []string  `json:"at_users,omitempty"`
	IPLocation      string    `json:"ip_location,omitempty"`
	CreateTime      int64     `json:"create_time"`
	Status          int       `json:"status,omitempty"`
	SubComments     []Comment `json:"sub_comments,omitempty"`
	FetchedAt       string    `json:"fetched_at"`
}

// Tag is a topic or hashtag.
type Tag struct {
	ID        string `json:"id,omitempty"`
	Name      string `json:"name"`
	Type      string `json:"type,omitempty"`
	ViewNum   int64  `json:"view_num,omitempty"`
	Link      string `json:"link,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

// Board is a user's collection of notes.
type Board struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	NoteCount int64  `json:"note_count"`
	OwnerID   string `json:"owner_id,omitempty"`
	FetchedAt string `json:"fetched_at"`
}

// SearchResult is a discriminated wrapper over a note or a user search hit.
type SearchResult struct {
	ModelType string `json:"model_type"` // "note" or "user"
	Note      *Note  `json:"note,omitempty"`
	User      *User  `json:"user,omitempty"`
}

// FeedItem wraps a note coming from the homefeed or explore stream, carrying
// the track id and the xsec_token needed to open the note's detail.
type FeedItem struct {
	NoteID     string `json:"note_id"`
	Type       string `json:"type"`
	Title      string `json:"title"`
	UserID     string `json:"user_id"`
	Nickname   string `json:"nickname"`
	Cover      string `json:"cover,omitempty"`
	LikedCount int64  `json:"liked_count"`
	TrackID    string `json:"track_id,omitempty"`
	XsecToken  string `json:"xsec_token,omitempty"`
	URL        string `json:"url"`
	FetchedAt  string `json:"fetched_at"`
}

// Me is the login state of the configured cookie.
type Me struct {
	LoggedIn bool   `json:"logged_in"`
	UserID   string `json:"user_id,omitempty"`
	Nickname string `json:"nickname,omitempty"`
	RedID    string `json:"red_id,omitempty"`
}
