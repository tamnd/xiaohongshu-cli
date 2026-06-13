package cli

import (
	"iter"
	"strings"

	"github.com/spf13/cobra"
	"github.com/tamnd/xiaohongshu-cli/pkg/xhsurl"
	"github.com/tamnd/xiaohongshu-cli/xiaohongshu"
)

// emitAll writes a slice of records and closes the output.
func emitAll[T any](a *App, items []T) error {
	out, err := a.newOutput()
	if err != nil {
		return err
	}
	for _, it := range items {
		if err := out.Emit(it); err != nil {
			return err
		}
	}
	return out.Close()
}

// emitSeq streams records from an iterator and closes the output.
func emitSeq[T any](a *App, seq iter.Seq2[T, error]) error {
	out, err := a.newOutput()
	if err != nil {
		return err
	}
	var seqErr error
	for v, e := range seq {
		if e != nil {
			seqErr = e
			break
		}
		if err := out.Emit(v); err != nil {
			return err
		}
	}
	if cerr := out.Close(); cerr != nil && seqErr == nil {
		seqErr = cerr
	}
	return seqErr
}

// emitSeqFunc is emitSeq with a per-record transform, used to unwrap a wrapper
// record such as SearchResult into its concrete payload before emitting.
func emitSeqFunc[T any](a *App, seq iter.Seq2[T, error], fn func(T) any) error {
	out, err := a.newOutput()
	if err != nil {
		return err
	}
	var seqErr error
	for v, e := range seq {
		if e != nil {
			seqErr = e
			break
		}
		if err := out.Emit(fn(v)); err != nil {
			return err
		}
	}
	if cerr := out.Close(); cerr != nil && seqErr == nil {
		seqErr = cerr
	}
	return seqErr
}

func emitOne(a *App, v any) error {
	return emitAll(a, []any{v})
}

// ---- note ----

func newNoteCmd(a *App) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:     "note <id|url>...",
		Short:   "Resolve one or more notes to full metadata",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs note 6849c2f0000000001e034c8e --token <xsec_token>\n  xhs note 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>'\n  xhs search 'latte art' -o url | xhs note -",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			var notes []xiaohongshu.Note
			for _, raw := range readArgsOrStdin(args) {
				ref := xhsurl.Parse(raw)
				tok := firstNonEmpty(token, ref.XsecToken)
				n, err := c.Note(a.ctx(), ref.NoteID, tok)
				if err != nil {
					a.progress("skip %s: %v", ref.NoteID, err)
					continue
				}
				notes = append(notes, n)
			}
			return emitAll(a, notes)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "xsec_token for the note (from a listing or share URL)")
	return cmd
}

// ---- user ----

func newUserCmd(a *App) *cobra.Command {
	var notes bool
	cmd := &cobra.Command{
		Use:     "user <id|url>...",
		Short:   "Resolve a creator profile, or list a creator's notes with --notes",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs user 5ff0e6500000000001008400\n  xhs user <id> --notes -n 50",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			if notes {
				for _, raw := range readArgsOrStdin(args) {
					ref := xhsurl.ParseUser(raw)
					if err := emitSeq(a, c.UserNotes(a.ctx(), ref.UserID, a.limit)); err != nil {
						return err
					}
				}
				return nil
			}
			var users []xiaohongshu.User
			for _, raw := range readArgsOrStdin(args) {
				ref := xhsurl.ParseUser(raw)
				u, err := c.User(a.ctx(), ref.UserID)
				if err != nil {
					a.progress("skip %s: %v", ref.UserID, err)
					continue
				}
				users = append(users, u)
			}
			return emitAll(a, users)
		},
	}
	cmd.Flags().BoolVar(&notes, "notes", false, "list the creator's posted notes instead of the profile")
	return cmd
}

// ---- search ----

func newSearchCmd(a *App) *cobra.Command {
	var users bool
	cmd := &cobra.Command{
		Use:     "search <keyword>",
		Short:   "Search notes (or users with --users) for a keyword",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs search 'latte art' -n 40\n  xhs search 'travel japan' --users",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			keyword := joinArgs(args)
			if users {
				return emitSeqFunc(a, c.SearchUsers(a.ctx(), keyword, a.limit), unwrapSearch)
			}
			return emitSeqFunc(a, c.SearchNotes(a.ctx(), keyword, a.limit), unwrapSearch)
		},
	}
	cmd.Flags().BoolVar(&users, "users", false, "search users instead of notes")
	return cmd
}

func unwrapSearch(r xiaohongshu.SearchResult) any {
	if r.Note != nil {
		return r.Note
	}
	if r.User != nil {
		return r.User
	}
	return r
}

// ---- comments ----

func newCommentsCmd(a *App) *cobra.Command {
	var token string
	var deep bool
	cmd := &cobra.Command{
		Use:     "comments <note-id|url>",
		Short:   "Stream a note's comments, with --deep to include replies",
		Args:    cobra.ExactArgs(1),
		Example: "  xhs comments <note-id> --token <t> -n 100\n  xhs comments <url> --deep",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			ref := xhsurl.Parse(args[0])
			tok := firstNonEmpty(token, ref.XsecToken)
			return emitSeq(a, c.Comments(a.ctx(), ref.NoteID, tok, deep, a.limit))
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "xsec_token for the note")
	cmd.Flags().BoolVar(&deep, "deep", false, "also fetch each comment's replies")
	return cmd
}

// ---- feed ----

func newFeedCmd(a *App) *cobra.Command {
	var category string
	var list bool
	cmd := &cobra.Command{
		Use:     "feed",
		Short:   "Stream the recommendation homefeed for a category",
		Example: "  xhs feed -n 40\n  xhs feed --category food\n  xhs feed --list",
		RunE: func(_ *cobra.Command, _ []string) error {
			if list {
				var rows []map[string]any
				for _, name := range xiaohongshu.FeedCategories() {
					rows = append(rows, map[string]any{"category": name})
				}
				return emitAll(a, rows)
			}
			c := a.Client()
			return emitSeq(a, c.Feed(a.ctx(), category, a.limit))
		},
	}
	cmd.Flags().StringVar(&category, "category", "recommend", "homefeed channel name")
	cmd.Flags().BoolVar(&list, "list", false, "list the available category names")
	return cmd
}

// ---- related ----

func newRelatedCmd(a *App) *cobra.Command {
	var token string
	cmd := &cobra.Command{
		Use:     "related <note-id|url>",
		Short:   "List notes recommended alongside a given note",
		Args:    cobra.ExactArgs(1),
		Example: "  xhs related <note-id> --token <t> -n 20",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			ref := xhsurl.Parse(args[0])
			tok := firstNonEmpty(token, ref.XsecToken)
			items, err := c.Related(a.ctx(), ref.NoteID, tok, a.limit)
			if err != nil {
				return err
			}
			return emitAll(a, items)
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "xsec_token for the note")
	return cmd
}

// ---- tag ----

func newTagCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "tag <keyword>",
		Short:   "Resolve a topic page to its canonical name, id, and view count",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs tag 'coffee'",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			t, err := c.Tag(a.ctx(), joinArgs(args))
			if err != nil {
				return err
			}
			return emitOne(a, t)
		},
	}
	return cmd
}

// ---- suggest ----

func newSuggestCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "suggest <keyword>",
		Short:   "Print search autocomplete suggestions for a partial keyword",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs suggest 'coff'",
		RunE: func(_ *cobra.Command, args []string) error {
			c := a.Client()
			terms, err := c.Suggest(a.ctx(), joinArgs(args))
			if err != nil {
				return err
			}
			var rows []map[string]any
			for _, t := range terms {
				rows = append(rows, map[string]any{"term": t})
			}
			return emitAll(a, rows)
		},
	}
	return cmd
}

// ---- me ----

func newMeCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "me",
		Short: "Show the login state of the configured cookie",
		RunE: func(_ *cobra.Command, _ []string) error {
			c := a.Client()
			me, err := c.Me(a.ctx())
			if err != nil {
				return err
			}
			return emitOne(a, me)
		},
	}
	return cmd
}

// ---- id ----

func newIDCmd(a *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "id <input>...",
		Short:   "Parse ids, urls, and tokens out of any xiaohongshu link",
		Args:    cobra.MinimumNArgs(1),
		Example: "  xhs id 'https://www.xiaohongshu.com/explore/<id>?xsec_token=<t>'",
		RunE: func(_ *cobra.Command, args []string) error {
			var rows []map[string]any
			for _, raw := range readArgsOrStdin(args) {
				ref := xhsurl.Parse(raw)
				rows = append(rows, map[string]any{
					"kind":        kindName(ref.Kind),
					"note_id":     ref.NoteID,
					"user_id":     ref.UserID,
					"xsec_token":  ref.XsecToken,
					"xsec_source": ref.XsecSource,
				})
			}
			return emitAll(a, rows)
		},
	}
	return cmd
}

func kindName(k xhsurl.Kind) string {
	switch k {
	case xhsurl.KindNote:
		return "note"
	case xhsurl.KindUser:
		return "user"
	default:
		return "unknown"
	}
}

func joinArgs(args []string) string {
	return strings.Join(args, " ")
}
