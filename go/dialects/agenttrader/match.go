package agenttrader

import (
	"encoding/json"
	"net/http"
	"strings"
)

// EnvelopeProbe is the small part of the envelope challenges need without reading the whole dialect.
type EnvelopeProbe struct {
	Status  int
	Missing string
}

// ParseEnvelopeProbe reads status and missing from a JSON body when present.
func ParseEnvelopeProbe(body []byte) (EnvelopeProbe, bool) {
	var env struct {
		Status  *int   `json:"status"`
		Missing string `json:"missing"`
		Data    []json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil || env.Status == nil || env.Data == nil {
		return EnvelopeProbe{}, false
	}
	return EnvelopeProbe{Status: *env.Status, Missing: env.Missing}, true
}

// HasProfileLink reports whether the response carries the envelope profile Link header.
func HasProfileLink(header http.Header) bool {
	for _, l := range header.Values("Link") {
		if strings.Contains(l, "<"+ProfileURI+">") && strings.Contains(l, `rel="profile"`) {
			return true
		}
	}
	return false
}

// MatchCapabilityGap returns a matcher for AgentTrader envelope refusals with a given HTTP status
// and missing capability value (for example agent-session or listing-weeks). It requires the
// profile link or a valid envelope shape, so a pure x402 body does not match.
func MatchCapabilityGap(status int, missing string) func(int, http.Header, []byte) bool {
	return func(got int, header http.Header, body []byte) bool {
		if got != status {
			return false
		}
		if IsX402Body(body) {
			return false
		}
		probe, ok := ParseEnvelopeProbe(body)
		if !ok || probe.Missing != missing {
			return false
		}
		return HasProfileLink(header) || probe.Status == got
	}
}

// IsX402Body is re-exported logic for dialect consumers; surf has its own copy for matchers.
func IsX402Body(body []byte) bool {
	var probe struct {
		X402Version *int `json:"x402Version"`
	}
	return json.Unmarshal(body, &probe) == nil && probe.X402Version != nil
}
