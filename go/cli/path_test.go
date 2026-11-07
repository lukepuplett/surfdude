package cli

import "testing"

// Feature: reading one value out of a response (-q and --keep)

const sample = `{"status":200,"message":"OK","data":[{"token":"abc","count":3,"big":12345678901234567890,"ok":true,"none":null,"nested":{"a":[10,20]}}],"notes":[]}`

func TestLookup(t *testing.T) {
	cases := []struct {
		path   string
		want   string
		wantOK bool
	}{
		{"status", "200", true},
		{"message", "OK", true},
		{"data.0.token", "abc", true},
		{"data.0.count", "3", true},
		{"data.0.big", "12345678901234567890", true}, // no float rounding of a big number
		{"data.0.ok", "true", true},
		{"data.0.none", "", true},
		{"data.0.nested.a.1", "20", true},
		{"data.0.nested", `{"a":[10,20]}`, true},
		{"data.1.token", "", false},   // no second item
		{"data.0.missing", "", false}, // no such key
		{"data.x", "", false},         // a key where an index belongs
		{"nope", "", false},
		{"", "", false}, // an empty path asks for nothing
	}

	for _, c := range cases {
		got, ok := lookup([]byte(sample), c.path)
		if ok != c.wantOK || (ok && got != c.want) {
			t.Errorf("lookup(%q) = %q, %v; want %q, %v", c.path, got, ok, c.want, c.wantOK)
		}
	}
}

func TestLookup_NotJSON(t *testing.T) {
	if _, ok := lookup([]byte("<html>oops</html>"), "data.0.token"); ok {
		t.Fatal("found a value in something that is not JSON")
	}
}
