package surf

import (
	"encoding/json"
	"net/http"
)

// MatchX402Payment matches a pure x402 payment challenge: HTTP 402 and a body with x402Version, not
// the AgentTrader envelope (no numeric status+data list at the top level without x402Version, and
// typically no profile link).
func MatchX402Payment(status int, header http.Header, body []byte) bool {
	if status != http.StatusPaymentRequired {
		return false
	}
	var probe struct {
		X402Version *int `json:"x402Version"`
		Status      *int `json:"status"`
		Data        any  `json:"data"`
	}
	if err := json.Unmarshal(body, &probe); err != nil {
		return false
	}
	return probe.X402Version != nil
}

// IsX402Body reports whether raw JSON looks like an x402 payment request.
func IsX402Body(body []byte) bool {
	var probe struct {
		X402Version *int `json:"x402Version"`
	}
	return json.Unmarshal(body, &probe) == nil && probe.X402Version != nil
}
