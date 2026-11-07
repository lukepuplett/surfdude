package cli

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

// recorded is one request the fake API received.
type recorded struct {
	Method, Path, Query, Auth, ContentType, Body string
}

// fakeAPI is a small hypermedia API in the AgentTrader envelope. Its front door offers register,
// verify-email, sign-in, set-count (a typed form), refuse (always 422), boom (always 500), a protected
// route that needs a token, and a mailbox link with a URI template.
type fakeAPI struct {
	*httptest.Server
	mu   sync.Mutex
	reqs []recorded
}

const (
	profileLink = `<https://agenttrader.net/profiles/envelope/v1>; rel="profile"`

	registerAction = `{"rel":"register","action":"/auth/email/start","method":"POST","fields":[` +
		`{"name":"email","required":true,"schema":{"type":"string","format":"email"}}]}`
	verifyAction = `{"rel":"verify-email","action":"/auth/email/verify","method":"POST","fields":[` +
		`{"name":"email","required":true,"schema":{"type":"string","format":"email"}},` +
		`{"name":"code","required":true,"schema":{"type":"string","pattern":"^[0-9]{6}$"}}]}`
	signInAction = `{"rel":"sign-in","action":"/auth/token","method":"POST","fields":[` +
		`{"name":"credential","required":true,"schema":{"type":"string"}}]}`
	countAction = `{"rel":"set-count","action":"/count","method":"POST","fields":[` +
		`{"name":"count","required":true,"schema":{"type":"integer","minimum":1,"maximum":10,"description":"how many"}},` +
		`{"name":"note","required":false,"schema":{"type":"string","maxLength":20}}]}`
	secretAction = `{"rel":"set-password","action":"/password","method":"POST","fields":[` +
		`{"name":"password","required":true,"schema":{"type":"string"}},` +
		`{"name":"email","required":true,"schema":{"type":"string"}}]}`
)

func newFakeAPI(t *testing.T) *fakeAPI {
	t.Helper()
	f := &fakeAPI{}
	mux := http.NewServeMux()

	send := func(w http.ResponseWriter, status int, body string) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Add("Link", profileLink)
		w.WriteHeader(status)
		_, _ = io.WriteString(w, body)
	}
	jsonBody := func(r *http.Request) map[string]any {
		var m map[string]any
		_ = json.NewDecoder(strings.NewReader(f.last().Body)).Decode(&m)
		return m
	}

	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, `{"status":200,"message":"OK","notes":["This is a fake."],"data":[{"name":"Fake"}],`+
			`"_links":[{"rel":"health","href":"/health","method":"GET"},`+
			`{"rel":"outbox","href":"/sandbox/outbox{?to}","method":"GET"},`+
			`{"rel":"protected","href":"/protected","method":"GET"},`+
			`{"rel":"peek","href":"/peek{?token,to}","method":"GET"}],`+
			`"_actions":[`+registerAction+`,`+verifyAction+`,`+signInAction+`,`+countAction+`,`+secretAction+`,`+
			`{"rel":"refuse","action":"/refuse","method":"POST","fields":[]},`+
			`{"rel":"boom","action":"/boom","method":"POST","fields":[]}]}`)
	})
	mux.HandleFunc("POST /auth/email/start", func(w http.ResponseWriter, r *http.Request) {
		send(w, 202, `{"status":202,"message":"Accepted","notes":["A code is on its way."],"data":[],"_actions":[`+verifyAction+`]}`)
	})
	mux.HandleFunc("GET /sandbox/outbox", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, `{"status":200,"message":"OK","data":[{"to":"`+r.URL.Query().Get("to")+`","text":"Your code is 654321"}]}`)
	})
	mux.HandleFunc("POST /auth/email/verify", func(w http.ResponseWriter, r *http.Request) {
		if jsonBody(r)["code"] != "654321" {
			send(w, 422, `{"status":422,"message":"Unable to verify the code. The code was not accepted.","data":[{"field":"code","code":"wrong","message":"No."}]}`)
			return
		}
		send(w, 200, `{"status":200,"message":"OK","notes":["Store the credential now."],"data":[{"agentKey":"agt_key","credential":"agt_testcredential","token":"tok_verify","recovered":false}]}`)
	})
	mux.HandleFunc("POST /auth/token", func(w http.ResponseWriter, r *http.Request) {
		if jsonBody(r)["credential"] != "agt_testcredential" {
			send(w, 401, `{"status":401,"message":"Unable to sign in. Not right.","data":[]}`)
			return
		}
		send(w, 200, `{"status":200,"message":"OK","data":[{"agentKey":"agt_key","token":"tok_signed"}]}`)
	})
	mux.HandleFunc("GET /protected", func(w http.ResponseWriter, r *http.Request) {
		if f.last().Auth != "Bearer tok_signed" {
			w.Header().Set("WWW-Authenticate", `Bearer realm="fake"`)
			send(w, 401, `{"status":401,"message":"Sign in again","data":[],"_actions":[`+signInAction+`]}`)
			return
		}
		send(w, 200, `{"status":200,"message":"OK","data":[{"hello":"you"}]}`)
	})
	mux.HandleFunc("GET /peek", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, `{"status":200,"message":"OK","data":[{"peeked":true}]}`)
	})
	mux.HandleFunc("POST /count", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, `{"status":200,"message":"OK","data":[{"stored":true}]}`)
	})
	mux.HandleFunc("POST /password", func(w http.ResponseWriter, r *http.Request) {
		send(w, 200, `{"status":200,"message":"OK","data":[{"changed":true}]}`)
	})
	mux.HandleFunc("POST /refuse", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "90")
		send(w, 422, `{"status":422,"message":"Unable to refuse. Not allowed.","notes":["Try something else."],"data":[{"field":"email","code":"invalid","message":"Use a real address."}]}`)
	})
	mux.HandleFunc("POST /boom", func(w http.ResponseWriter, r *http.Request) {
		send(w, 500, `{"status":500,"message":"Something went wrong on our side.","data":[]}`)
	})

	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		f.mu.Lock()
		f.reqs = append(f.reqs, recorded{r.Method, r.URL.Path, r.URL.RawQuery, r.Header.Get("Authorization"), r.Header.Get("Content-Type"), string(body)})
		f.mu.Unlock()
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(f.Close)

	return f
}

// last is the most recent request.
func (f *fakeAPI) last() recorded {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.reqs) == 0 {
		return recorded{}
	}

	return f.reqs[len(f.reqs)-1]
}

// all is every request, oldest first.
func (f *fakeAPI) all() []recorded {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]recorded(nil), f.reqs...)
}

// count is how many requests of a method and path were received.
func (f *fakeAPI) count(method, path string) int {
	n := 0
	for _, r := range f.all() {
		if r.Method == method && r.Path == path {
			n++
		}
	}

	return n
}

// result is what a run of the command produced.
type result struct {
	Code        int
	Out, ErrOut string
}

// runCLI runs the command against the fake API with the given standard input and environment.
func runCLI(t *testing.T, stdin string, env map[string]string, args ...string) result {
	t.Helper()
	var out, errOut bytes.Buffer
	code := Run(args, IO{Stdin: strings.NewReader(stdin), Stdout: &out, Stderr: &errOut}, func(k string) string { return env[k] })

	return result{Code: code, Out: out.String(), ErrOut: errOut.String()}
}
