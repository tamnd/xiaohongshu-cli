package xiaohongshu

import "maps"

// Envelope records how a record was obtained, so a consumer can tell a fact
// from an absence.
//
// It describes the reading rather than the thing read. That matters most on a
// user, where up to three requests stand behind one row and any of them can be
// refused independently: without the envelope, a creator with no public
// collections and a creator whose collection count was never fetched produce
// the same row.
type Envelope struct {
	Endpoint string            `json:"endpoint"`         // the path that answered
	Host     string            `json:"host"`             // "web" or "api"
	Signed   bool              `json:"signed"`           // whether x-s was computed
	Status   string            `json:"status"`           // the Status name
	Approx   bool              `json:"approx,omitempty"` // counts were rounded, see below
	Fetched  string            `json:"fetched"`          // RFC3339
	Bytes    int               `json:"bytes"`
	Missed   map[string]string `json:"missed,omitempty"` // field -> why it is absent
}

// miss records that a field is absent and says what stopped it.
func (e *Envelope) miss(field, why string) {
	if e.Missed == nil {
		e.Missed = map[string]string{}
	}
	e.Missed[field] = why
}

// clone returns a copy safe to hand to a second record.
func (e Envelope) clone() Envelope {
	out := e
	if e.Missed != nil {
		out.Missed = make(map[string]string, len(e.Missed))
		maps.Copy(out.Missed, e.Missed)
	}
	return out
}

// markApprox records that at least one count on this record came from a display
// string the site had already rounded.
//
// The rendered pages print 1.9万 and 10万+ where the API sends the exact
// integer, so the anonymous path yields numbers correct to two significant
// figures and the credentialed path yields numbers. A consumer summing a column
// has no way to know which they have unless the record says.
func (e *Envelope) markApprox() { e.Approx = true }
