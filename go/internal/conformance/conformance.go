// Package conformance checks a dialect against the language-neutral fixtures in /fixtures. Each
// dialect directory has a cases.json: a response file, its headers, and what the neutral model must
// hold after reading it. The C# client can read the same files, so two implementations are held to
// one definition.
package conformance

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialect"
	"github.com/lukepuplett/surfdude/go/hypertext"
)

type file struct {
	Dialect string `json:"dialect"`
	Cases   []struct {
		Name        string            `json:"name"`
		File        string            `json:"file"`
		ContentType string            `json:"contentType"`
		Headers     map[string]string `json:"headers"`
		Status      int               `json:"status"`
		Expect      expect            `json:"expect"`
	} `json:"cases"`
}

type expect struct {
	Status   int                `json:"status"`
	Notes    *int               `json:"notes"`
	Paging   *paging            `json:"paging"`
	Rels     []string           `json:"rels"`
	Controls map[string]control `json:"controls"`
	Items    []struct {
		Rels     []string           `json:"rels"`
		Controls map[string]control `json:"controls"`
	} `json:"items"`
	Problems []struct {
		Field string `json:"field"`
		Code  string `json:"code"`
	} `json:"problems"`
}

type paging struct {
	Skip, Take, Count int
	Total             *int
}

type control struct {
	Form        bool           `json:"form"`
	Method      string         `json:"method"`
	Inputs      []string       `json:"inputs"`
	Required    []string       `json:"required"`
	Extensions  map[string]any `json:"extensions"`
	IfMatch     string         `json:"ifMatch"`
	InputValues map[string]any `json:"inputValues"`
}

// Run reads dir/cases.json and checks every case against the dialect.
func Run(t *testing.T, d dialect.Dialect, dir string) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(dir, "cases.json"))
	if err != nil {
		t.Fatalf("Unable to read the conformance cases: %v", err)
	}
	var f file
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("Unable to parse cases.json: %v", err)
	}
	if f.Dialect != d.Name() {
		t.Fatalf("cases.json is for the dialect %q but this is %q", f.Dialect, d.Name())
	}

	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			body, err := os.ReadFile(filepath.Join(dir, c.File))
			if err != nil {
				t.Fatal(err)
			}
			resp := &http.Response{StatusCode: c.Status, Header: http.Header{}}
			resp.Header.Set("Content-Type", c.ContentType)
			for k, v := range c.Headers {
				resp.Header.Add(k, v)
			}

			if !d.Matches(resp, body) {
				t.Fatalf("the %s dialect should recognise this response", d.Name())
			}
			res, err := d.Read(resp, body)
			if err != nil {
				t.Fatalf("Read failed: %v", err)
			}

			check(t, res, c.Expect)
		})
	}
}

func check(t *testing.T, res *hypertext.Resource, want expect) {
	t.Helper()
	if res.Status != want.Status {
		t.Errorf("status = %d, want %d", res.Status, want.Status)
	}
	if want.Notes != nil && len(res.Notes) != *want.Notes {
		t.Errorf("notes = %d, want %d", len(res.Notes), *want.Notes)
	}
	if want.Paging != nil {
		got := res.Paging
		if got == nil || got.Skip != want.Paging.Skip || got.Take != want.Paging.Take || got.Count != want.Paging.Count || !reflect.DeepEqual(got.Total, want.Paging.Total) {
			t.Errorf("paging = %+v, want %+v", got, want.Paging)
		}
	}
	if want.Rels != nil && !slices.Equal(res.Rels(), want.Rels) {
		t.Errorf("rels = %v, want %v", res.Rels(), want.Rels)
	}
	for rel, w := range want.Controls {
		c, ok := res.Control(rel)
		if !ok {
			t.Errorf("missing control %q", rel)
			continue
		}
		checkControl(t, rel, c, w)
	}
	if want.Items != nil {
		if len(res.Items) != len(want.Items) {
			t.Fatalf("items = %d, want %d", len(res.Items), len(want.Items))
		}
		for i, w := range want.Items {
			it := res.Items[i]
			if w.Rels != nil && !slices.Equal(it.Rels(), w.Rels) {
				t.Errorf("item %d rels = %v, want %v", i, it.Rels(), w.Rels)
			}
			for rel, cw := range w.Controls {
				c, ok := it.Control(rel)
				if !ok {
					t.Errorf("item %d is missing control %q", i, rel)
					continue
				}
				checkControl(t, rel, c, cw)
			}
		}
	}
	if want.Problems != nil {
		if len(res.Problems) != len(want.Problems) {
			t.Fatalf("problems = %d, want %d", len(res.Problems), len(want.Problems))
		}
		for i, p := range want.Problems {
			if res.Problems[i].Field != p.Field || res.Problems[i].Code != p.Code {
				t.Errorf("problem %d = %+v, want %+v", i, res.Problems[i], p)
			}
		}
	}
}

func checkControl(t *testing.T, rel string, c hypertext.Control, w control) {
	t.Helper()
	if c.Form != w.Form {
		t.Errorf("%s: form = %v, want %v", rel, c.Form, w.Form)
	}
	if c.HTTPMethod() != w.Method {
		t.Errorf("%s: method = %s, want %s", rel, c.HTTPMethod(), w.Method)
	}
	if w.Inputs != nil {
		var names []string
		for _, in := range c.Inputs {
			names = append(names, in.Name)
		}
		if !slices.Equal(names, w.Inputs) {
			t.Errorf("%s: inputs = %v, want %v", rel, names, w.Inputs)
		}
	}
	if w.Required != nil {
		names := []string{}
		for _, in := range c.Inputs {
			if in.Required && in.Value == nil {
				names = append(names, in.Name)
			}
		}
		if !slices.Equal(names, w.Required) {
			t.Errorf("%s: required = %v, want %v", rel, names, w.Required)
		}
	}
	for name, want := range w.Extensions {
		var got any
		found, err := c.DecodeExtension(name, &got)
		if err != nil || !found || !reflect.DeepEqual(got, want) {
			t.Errorf("%s: extension %q = %v (found %v, err %v), want %v", rel, name, got, found, err, want)
		}
	}
	if c.IfMatch != w.IfMatch {
		t.Errorf("%s: ifMatch = %q, want %q", rel, c.IfMatch, w.IfMatch)
	}
	if w.InputValues != nil {
		for name, want := range w.InputValues {
			var got any
			var found bool
			for _, in := range c.Inputs {
				if in.Name == name {
					got, found = in.Value, true
					break
				}
			}
			if !found {
				t.Errorf("%s: input %q not found for inputValues check", rel, name)
				continue
			}
			if !inputValueEqual(got, want) {
				t.Errorf("%s: input %q value = %v (%T), want %v (%T)", rel, name, got, got, want, want)
			}
		}
	}
}

func inputValueEqual(got, want any) bool {
	if reflect.DeepEqual(got, want) {
		return true
	}
	return fmt.Sprint(got) == fmt.Sprint(want)
}
