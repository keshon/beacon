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
	tests := []struct {
		name    string
		body    string
		keyword string
		invert  bool
		wantErr string
	}{
		{name: "empty keyword", body: "hello world", keyword: "", wantErr: ""},
		{name: "found single word", body: "hello world", keyword: "world", wantErr: ""},
		{name: "found exact phrase", body: "hello world", keyword: "hello world", wantErr: ""},
		{name: "found phrase extra spaces", body: "hello   world", keyword: "hello world", wantErr: ""},
		{name: "found phrase html between words", body: "hello <b>world</b>", keyword: "hello world", wantErr: ""},
		{name: "found phrase newline", body: "hello\nworld", keyword: "hello world", wantErr: ""},
		{name: "missing", body: "hello world", keyword: "missing", wantErr: "keyword not found"},
		{name: "missing phrase wrong order", body: "world hello", keyword: "hello world", wantErr: "keyword not found"},
		{name: "invert ok", body: "hello world", keyword: "missing", invert: true, wantErr: ""},
		{name: "invert fail", body: "hello world", keyword: "world", invert: true, wantErr: "forbidden keyword"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := matchHTTPKeyword([]byte(tc.body), tc.keyword, tc.invert)
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

func TestBodyContainsKeyword(t *testing.T) {
	if !bodyContainsKeyword([]byte("a  b  c"), "a b c") {
		t.Fatal("expected flexible phrase match")
	}
	if bodyContainsKeyword([]byte("abc"), "a b") {
		t.Fatal("should not match non-contiguous letters as words")
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
