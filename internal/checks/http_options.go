package checks

import "strings"

// HTTPOptions holds optional HTTP check settings.
type HTTPOptions struct {
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	Keyword       string `json:"keyword,omitempty"`
	KeywordInvert bool   `json:"keyword_invert,omitempty"`
}

// Redacted returns a copy without password.
func (o *HTTPOptions) Redacted() *HTTPOptions {
	if o == nil {
		return nil
	}
	c := *o
	c.Password = ""
	return &c
}

// MergeHTTPOptions applies patch semantics.
func MergeHTTPOptions(existing, incoming *HTTPOptions) *HTTPOptions {
	if incoming == nil {
		return existing
	}
	out := HTTPOptions{}
	if existing != nil {
		out = *existing
	}
	if u := strings.TrimSpace(incoming.Username); u != "" {
		out.Username = u
	}
	if incoming.Password != "" {
		out.Password = incoming.Password
	}
	out.Keyword = strings.TrimSpace(incoming.Keyword)
	out.KeywordInvert = incoming.KeywordInvert
	if out.Username == "" && out.Password == "" && out.Keyword == "" && !out.KeywordInvert {
		return nil
	}
	return &out
}
