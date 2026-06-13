package xiaohongshu

import "fmt"

// APIError is a non-success Xiaohongshu response mapped to a clean message.
type APIError struct {
	Code    int
	Message string // upstream message, often Chinese
	Hint    string // English hint
	Kind    ErrKind
}

// ErrKind groups API errors so the CLI can map them to exit codes.
type ErrKind int

const (
	ErrGeneric ErrKind = iota
	ErrNotFound
	ErrAccess
	ErrRate
	ErrAntibot
	ErrNetwork
)

func (e *APIError) Error() string {
	if e.Hint != "" {
		return fmt.Sprintf("xiaohongshu %d: %s (%s)", e.Code, e.Hint, e.Message)
	}
	return fmt.Sprintf("xiaohongshu %d: %s", e.Code, e.Message)
}

// apiError maps a code/message into a typed error with an English hint.
func apiError(code int, message string) *APIError {
	e := &APIError{Code: code, Message: message, Kind: ErrGeneric}
	switch code {
	case -100:
		e.Hint, e.Kind = "session expired or not logged in: pass a logged-in cookie via --cookie or XHS_COOKIE", ErrAccess
	case -101:
		e.Hint, e.Kind = "account state error: this surface needs a logged-in cookie", ErrAccess
	case 461, 406:
		e.Hint, e.Kind = "request rejected by anti-bot: the signature or IP was refused, use a residential IP and a logged-in cookie", ErrAntibot
	case -510, 10013:
		e.Hint, e.Kind = "too many requests: slow down with --rate and retry", ErrRate
	case 300012, 300015, 300013:
		e.Hint, e.Kind = "risk control (browser/network anomaly): XHS blocked this anonymous request, use a residential IP and a logged-in cookie", ErrAntibot
	case -1, 0:
		e.Hint = "server error"
	}
	switch message {
	case "笔记不存在", "该笔记不存在或已删除":
		e.Hint, e.Kind = "note not found or removed", ErrNotFound
	case "用户不存在":
		e.Hint, e.Kind = "user not found", ErrNotFound
	}
	return e
}

// Kind reports the ErrKind of an error if it is an APIError.
func Kind(err error) ErrKind {
	if ae, ok := err.(*APIError); ok {
		return ae.Kind
	}
	return ErrGeneric
}
