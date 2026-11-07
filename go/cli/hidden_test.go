package cli

import (
	"sort"
	"strings"
	"testing"
)

// Feature: values are hidden by full field name, and --reveal is the switch
//
// Luke: a hard list of full field names and a switch, not a guess from part of a name. So
// passwordWasRight and tokenizer are shown, and credential and access_token are hidden.

// Scenario: the list is exactly the one Luke approved
func TestHiddenFieldNames_IsTheApprovedList(t *testing.T) {
	want := []string{"accesstoken", "apikey", "authorization", "clientsecret", "credential", "password", "privatekey", "refreshtoken", "secret", "token"}

	got := append([]string(nil), hiddenFieldNames...)
	sort.Strings(got)

	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("hiddenFieldNames = %v, want %v", got, want)
	}
}

// Scenario: each listed name is hidden whatever its case or separators, and a dotted name by its last part
func TestHiddenByName_HidesTheListedNamesAndTheirVariants(t *testing.T) {
	for _, name := range []string{
		"credential", "Credential", "CREDENTIAL",
		"token", "Token", "TOKEN",
		"accessToken", "access_token", "access-token", "ACCESS_TOKEN", "AccessToken",
		"refreshToken", "refresh_token", "refresh-token",
		"password", "Password", "PASSWORD",
		"secret", "Secret",
		"clientSecret", "client_secret", "client-secret", "CLIENT_SECRET",
		"apiKey", "api_key", "api-key", "API-Key", "API_KEY",
		"privateKey", "private_key", "private-key",
		"authorization", "Authorization", "AUTHORIZATION",
		"user.password", "data.0.credential", "a.b.client_secret",
	} {
		if !hiddenByName(name) {
			t.Errorf("hiddenByName(%q) = false, want true", name)
		}
	}
}

// Scenario: a name that only contains or resembles a listed one is shown
func TestHiddenByName_ShowsNearMisses(t *testing.T) {
	for _, name := range []string{
		"passwordWasRight", "tokenizer", "secretary", "keyboard", "tokens", "credentials",
		"userPassword", "apiToken", "authToken", "secretKey", "mysecret", "passwordHint",
		"agentKey", "email", "code", "key", "otp", "pin",
		"", "password.count", "token.",
	} {
		if hiddenByName(name) {
			t.Errorf("hiddenByName(%q) = true, want false: only a full listed name is hidden", name)
		}
	}
}

// Scenario: the response summary hides the listed names and shows the near misses
func TestHidden_SummaryHidesListedNamesAndShowsNearMisses(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, api.URL, "secrets")

	for _, want := range []string{"token=<secret>", "accessToken=<secret>", "credential=<secret>", "apiKey=<secret>",
		"passwordWasRight=true", "tokenizer=tokenizer-visible", "agentKey=agt_key"} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q:\n%s", want, r.Out)
		}
	}
	for _, secret := range []string{"tok-sum-1", "acc-sum-2", "cred-sum-3", "key-sum-4"} {
		if strings.Contains(r.Out+r.ErrOut, secret) {
			t.Errorf("%s was printed:\n%s", secret, r.Out)
		}
	}
}

// Scenario: --reveal shows the values hidden by name in the summary
func TestHidden_RevealShowsTheNameHiddenValuesInTheSummary(t *testing.T) {
	api := newFakeAPI(t)

	r := runCLI(t, "", nil, "--reveal", api.URL, "secrets")

	for _, want := range []string{"token=tok-sum-1", "accessToken=acc-sum-2", "credential=cred-sum-3", "apiKey=key-sum-4"} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("stdout lacks %q with --reveal:\n%s", want, r.Out)
		}
	}
}

// Scenario: --json and -q show the real value with or without --reveal
func TestHidden_JSONAndQueryAlwaysShowTheRealValue(t *testing.T) {
	api := newFakeAPI(t)

	if r := runCLI(t, "", nil, "-q", "data.0.token", api.URL, "secrets"); r.Out != "tok-sum-1\n" {
		t.Errorf("-q printed %q, want the real token", r.Out)
	}
	if r := runCLI(t, "", nil, "--json", api.URL, "secrets"); !strings.Contains(r.Out, "tok-sum-1") || !strings.Contains(r.Out, "cred-sum-3") {
		t.Errorf("--json hid a value:\n%s", r.Out)
	}
}

// Scenario: the dry run hides a value by its name, and --reveal shows it
func TestHidden_DryRunByNameAndReveal(t *testing.T) {
	api := newFakeAPI(t)
	args := []string{"--dry-run", api.URL, "set-password", "email=a@b.test", "password=hunter2-typed"}

	hidden := runCLI(t, "", nil, args...)
	if strings.Contains(hidden.Out, "hunter2-typed") || !strings.Contains(hidden.Out, `"password":"<secret>"`) {
		t.Errorf("the typed password was not hidden by its name:\n%s", hidden.Out)
	}

	shown := runCLI(t, "", nil, append([]string{"--reveal"}, args...)...)
	if !strings.Contains(shown.Out, `"password":"hunter2-typed"`) {
		t.Errorf("--reveal did not show the password in the dry run:\n%s", shown.Out)
	}

	// The query string follows the same rule.
	if r := runCLI(t, "", nil, "--dry-run", api.URL, "peek", "token=sekrit123", "to=x@y.test"); strings.Contains(r.Out, "sekrit123") {
		t.Errorf("the token in the query was shown:\n%s", r.Out)
	}
	if r := runCLI(t, "", nil, "--reveal", "--dry-run", api.URL, "peek", "token=sekrit123", "to=x@y.test"); !strings.Contains(r.Out, "token=sekrit123") {
		t.Errorf("--reveal did not show the token in the query:\n%s", r.Out)
	}
}

// Scenario: --reveal never shows a value the user hid by its source, nor the bearer token
func TestHidden_RevealNeverShowsAValueHiddenBySource(t *testing.T) {
	api := newFakeAPI(t)
	env := map[string]string{"PW": "from-env-77"}

	cases := []struct {
		name   string
		args   []string
		stdin  string
		secret string
	}{
		{"from the environment", []string{"set-password", "email=a@b.test", "password=env:PW"}, "", "from-env-77"},
		{"from standard input", []string{"set-password", "email=a@b.test", "password=@-"}, "from-stdin-88\n", "from-stdin-88"},
		{"asked for with --secret, under a name that is not on the list", []string{"set-password", "--secret", "email", "password=x"}, "private@b.test\n", "private@b.test"},
	}
	for _, c := range cases {
		r := runCLI(t, c.stdin, env, append([]string{"--reveal", "--dry-run", api.URL}, c.args...)...)

		if r.Code != ExitOK {
			t.Errorf("%s: exit %d\nstderr: %s", c.name, r.Code, r.ErrOut)
			continue
		}
		if strings.Contains(r.Out+r.ErrOut, c.secret) {
			t.Errorf("%s: --reveal showed a value that was hidden by its source:\n%s", c.name, r.Out)
		}
	}

	r := runCLI(t, "", nil, "--reveal", "--dry-run", "--bearer", "bearer-sekrit-9", api.URL, "set-password", "email=a@b.test", "password=x")
	if strings.Contains(r.Out+r.ErrOut, "bearer-sekrit-9") || !strings.Contains(r.Out, "Authorization: Bearer <secret>") {
		t.Errorf("--reveal showed the bearer token:\n%s", r.Out)
	}
}

// Scenario: describe hides a prefilled value under a listed name, and --reveal shows it
func TestHidden_DescribeHidesAPrefilledValueByNameUnlessRevealed(t *testing.T) {
	api := newFakeAPI(t)

	hidden := runCLI(t, "", nil, "describe", api.URL, "set-prefilled")
	if strings.Contains(hidden.Out, "prefilled-pw-77") || !strings.Contains(hidden.Out, "prefilled: <secret>") {
		t.Errorf("the prefilled password was shown:\n%s", hidden.Out)
	}
	if !strings.Contains(hidden.Out, "prefilled: prefilled-hint-88") {
		t.Errorf("passwordHint, which is not on the list, was hidden:\n%s", hidden.Out)
	}

	shown := runCLI(t, "", nil, "--reveal", "describe", api.URL, "set-prefilled")
	if !strings.Contains(shown.Out, "prefilled-pw-77") {
		t.Errorf("--reveal did not show the prefilled password:\n%s", shown.Out)
	}
}

// Scenario: a kept value is secret when its name or the path it came from ends in a listed name
func TestHidden_AKeptValueIsJudgedByItsNameAndByTheLastPartOfItsPath(t *testing.T) {
	cases := []struct {
		k      keep
		secret bool
	}{
		{keep{Name: "cred", Path: "data.0.credential"}, true}, // by where it came from
		{keep{Name: "token", Path: "data.0.text"}, true},      // by what it is called
		{keep{Name: "c", Path: "data.0.access_token"}, true},
		{keep{Name: "code", Path: "data.0.text"}, false},
		{keep{Name: "c", Path: "data.0.tokenizer"}, false}, // a near miss
		{keep{Name: "c", Path: "data.credential.count"}, false},
	}
	for _, c := range cases {
		if got := (masking{}).hidesKept(c.k); got != c.secret {
			t.Errorf("hidesKept(%+v) = %v, want %v", c.k, got, c.secret)
		}
		if got := (masking{reveal: true}).hidesKept(c.k); got {
			t.Errorf("hidesKept(%+v) with --reveal = true, want false", c.k)
		}
	}
}

// Scenario: the usage text lists the hidden names and the switch
func TestHidden_UsageTextListsTheNamesAndTheSwitch(t *testing.T) {
	r := runCLI(t, "", nil, "help")

	for _, want := range []string{"--reveal", "credential", "accesstoken", "refreshtoken", "clientsecret", "apikey", "privatekey", "authorization"} {
		if !strings.Contains(r.Out, want) {
			t.Errorf("the usage text lacks %q:\n%s", want, r.Out)
		}
	}
}
