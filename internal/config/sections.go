package config

import "encoding/json"

// Which sections a config patch actually mentioned.
//
// Decoding a patch into a struct loses the one thing a partial update needs to
// know: whether a field was ABSENT or set to zero. Both arrive as the zero
// value, so a screen that does not render the Telegram fields would send
// telegram:{} and switch the channel off.
//
// That did not matter while every setting lived in one form and every save sent
// the lot. It matters now: the settings belong to the screens they are about —
// channels to notifications, peers to the peers screen — and each screen must
// be able to save its own part without touching anyone else's.
//
// So the raw JSON is inspected for top-level keys before it is decoded, and the
// merge is gated on presence.
type Sections struct {
	present map[string]bool
}

// ParseSections records which top-level keys the patch carries.
func ParseSections(body []byte) (Sections, error) {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return Sections{}, err
	}
	present := make(map[string]bool, len(raw))
	for k := range raw {
		present[k] = true
	}
	return Sections{present: present}, nil
}

// Has reports whether the patch mentioned a key.
//
// A patch with NO recognised keys at all is treated as mentioning everything.
// That keeps the old whole-config clients working: they always send every
// section, so the answer is the same either way, and a genuinely empty body
// changes nothing regardless.
func (s Sections) Has(key string) bool {
	if len(s.present) == 0 {
		return true
	}
	return s.present[key]
}

// AllSections is the patch that touches everything — what a caller that has no
// section information should pass.
func AllSections() Sections { return Sections{} }
