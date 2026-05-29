package checks

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestHTTPCheck_rejectsURLUserinfo(t *testing.T) {
	res := HTTPCheck(context.Background(), "http://user:pass@example.com/", time.Second, nil)
	if res.Success {
		t.Fatal("expected failure for URL userinfo")
	}
	if !strings.Contains(res.Error, "credentials in URL") {
		t.Fatalf("unexpected error: %q", res.Error)
	}
}

func TestMatchHTTPKeyword(t *testing.T) {
	body := []byte("hello world")
	tests := []struct {
		name    string
		keyword string
		invert  bool
		wantErr string
	}{
		{name: "empty keyword", keyword: "", wantErr: ""},
		{name: "found", keyword: "world", wantErr: ""},
		{name: "missing", keyword: "missing", wantErr: "keyword not found"},
		{name: "invert ok", keyword: "missing", invert: true, wantErr: ""},
		{name: "invert fail", keyword: "world", invert: true, wantErr: "forbidden keyword"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := matchHTTPKeyword(body, tc.keyword, tc.invert)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("want error containing %q, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestMergeHTTPOptions(t *testing.T) {
	existing := &HTTPOptions{Username: "u1", Password: "p1", Keyword: "k1"}
	incoming := &HTTPOptions{Username: "u2", Keyword: "k2", KeywordInvert: true}
	got := MergeHTTPOptions(existing, incoming)
	if got.Username != "u2" || got.Password != "p1" || got.Keyword != "k2" || !got.KeywordInvert {
		t.Fatalf("unexpected merge: %+v", got)
	}
	if MergeHTTPOptions(nil, &HTTPOptions{}) != nil {
		t.Fatal("empty patch should return nil")
	}
}

func TestHTTPOptions_Redacted(t *testing.T) {
	o := &HTTPOptions{Username: "u", Password: "secret", Keyword: "k"}
	r := o.Redacted()
	if r.Password != "" || r.Username != "u" {
		t.Fatalf("redacted: %+v", r)
	}
}
