package cli

import (
	"strings"
	"unicode"
)

// hiddenFieldNames are the full field names whose values are hidden wherever the command shows a
// request or a response. A name matches when it equals one of these after it is lower-cased and its
// underscores and hyphens are removed, so access_token, API-Key and Credential match, while
// passwordWasRight, tokenizer, secretary and keyboard do not. It is a short list on purpose: Luke
// chose full names over guessing from part of a name, which hid fields that were not secret and is
// no safer. It is read-only; the usage text and the docs list it, and a test pins it.
var hiddenFieldNames = []string{
	"credential", "token", "accesstoken", "refreshtoken", "password",
	"secret", "clientsecret", "apikey", "privatekey", "authorization",
	// Added on 2026-11-08 at Luke's word, after noticing an API could call a token apiToken.
	"apitoken", "authtoken", "bearertoken", "idtoken", "sessiontoken", "userpassword",
}

// hiddenByName says whether a field name is on the hidden list. A dotted name, such as a nested
// field user.password or a path like data.0.credential, is judged by its last part, because the
// whole of it would never equal a name. The agent key is public and is not on the list.
func hiddenByName(name string) bool {
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	plain := normaliseName(name)
	if plain == "" {
		return false
	}
	for _, hidden := range hiddenFieldNames {
		if plain == hidden {
			return true
		}
	}

	return false
}

// normaliseName lower-cases a name and removes underscores and hyphens, so the same field written
// accessToken, access_token and ACCESS-TOKEN is one name.
func normaliseName(name string) string {
	var b strings.Builder
	for _, r := range name {
		if r == '_' || r == '-' {
			continue
		}
		b.WriteRune(unicode.ToLower(r))
	}

	return b.String()
}

// masking is how this run hides values by their field name. The zero value hides them. It is passed
// down to everything that prints a value, rather than kept in a package variable, so a run's choice
// cannot leak into another.
type masking struct {
	// reveal turns off hiding by field name. It never turns off hiding by source: a value the user
	// gave from --secret, standard input or the environment stays hidden, because the user chose to
	// hide it. Nor does it show the bearer token.
	reveal bool
}

// hides says whether a value under this field name must not be shown.
func (m masking) hides(name string) bool { return !m.reveal && hiddenByName(name) }

// hidesKept says whether a value that --keep took from a response is secret. It is judged by the name
// it was kept under and by the path it came from, so cred=data.0.credential is secret because of
// where it came from.
func (m masking) hidesKept(k keep) bool { return m.hides(k.Name) || m.hides(k.Path) }
