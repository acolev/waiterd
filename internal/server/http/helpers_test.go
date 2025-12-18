package httpserver

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestResolvePathTemplate(t *testing.T) {
	tests := []struct {
		name     string
		endpoint string
		call     string
		actual   string
		want     string
	}{
		{"no placeholders", "/posts", "/posts", "/posts", "/posts"},
		{"single id", "/posts/{id}", "/posts/{id}", "/posts/42", "/posts/42"},
		{"call has no placeholder", "/posts/{id}", "/other", "/posts/42", "/other"},
		{"multiple placeholders", "/a/{id}/b/{id}", "/c/{id}/d/{id}", "/a/1/b/2", "/c/1/d/2"},
		{"call more placeholders than endpoint", "/a/{id}", "/c/{id}/d/{id}", "/a/7", "/c/7/d/7"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolvePathTemplate(tt.endpoint, tt.call, tt.actual)
			if got != tt.want {
				t.Fatalf("resolvePathTemplate() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSingleJoinPath(t *testing.T) {
	tests := []struct {
		a, b string
		want string
	}{
		{"", "", "/"},
		{"", "foo", "/foo"},
		{"foo", "", "/foo"},
		{"/foo/", "/bar", "/foo/bar"},
		{"foo", "bar", "foo/bar"},
	}
	for _, tt := range tests {
		if got := singleJoinPath(tt.a, tt.b); got != tt.want {
			t.Fatalf("singleJoinPath(%q,%q)=%q want %q", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestDecodeWithMapping(t *testing.T) {
	body := []byte(`{"title":"hello","body":"world"}`)

	got := decodeWithMapping(body, map[string]string{"t": "title"})
	want := map[string]any{"t": "hello"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("decodeWithMapping mapped = %#v, want %#v", got, want)
	}

	gotAll := decodeWithMapping(body, nil)
	var parsed map[string]any
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("json unmarshal failed: %v", err)
	}
	if !reflect.DeepEqual(gotAll, parsed) {
		t.Fatalf("decodeWithMapping full = %#v, want %#v", gotAll, parsed)
	}

	nonJSON := []byte("plain")
	if s := decodeWithMapping(nonJSON, nil); s != "plain" {
		t.Fatalf("decodeWithMapping nonJSON = %v", s)
	}
}

func TestBuildAggregateResponse(t *testing.T) {
	perCall := map[string]any{
		"post": map[string]any{"title": "a", "body": "b"},
		"test": "ok",
	}

	mapping := map[string]string{"title": "post.title", "message": "test"}
	got := buildAggregateResponse(mapping, perCall)
	want := map[string]any{"title": "a", "message": "ok"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildAggregateResponse mapped = %#v, want %#v", got, want)
	}

	// empty mapping returns perCall as-is
	if got2 := buildAggregateResponse(nil, perCall); !reflect.DeepEqual(got2, perCall) {
		t.Fatalf("buildAggregateResponse passthrough = %#v, want %#v", got2, perCall)
	}
}
