package agenttrader_test

import (
	"net/http"
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/agenttrader"
	"github.com/lukepuplett/surfdude/go/internal/conformance"
)

// The envelope dialect reads the samples our API is designed around, through the same neutral model.
func TestAgentTrader_Conformance(t *testing.T) {
	conformance.Run(t, agenttrader.New(), "../../../fixtures/agenttrader")
}

// Scenario: a body that is not the envelope is not claimed
//
//	Given JSON without a status and a data list, and no profile link
//	Then the dialect does not match it, so another dialect can have it
func TestAgentTrader_DoesNotClaimOtherJSON(t *testing.T) {
	resp := &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": {"application/json"}}}

	if agenttrader.New().Matches(resp, []byte(`{"links":[]}`)) {
		t.Error("JSON that is not the envelope must not be claimed")
	}
	if agenttrader.New().Matches(&http.Response{Header: http.Header{"Content-Type": {"text/html"}}}, []byte(`<html></html>`)) {
		t.Error("HTML must not be claimed")
	}
}
