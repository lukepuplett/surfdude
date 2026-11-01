package simple_test

import (
	"testing"

	"github.com/lukepuplett/surfdude/go/dialects/simple"
	"github.com/lukepuplett/surfdude/go/internal/conformance"
)

// The simple dialect reads Surfdude's own media type, and holds to the same language-neutral
// fixtures the C# client can read.
func TestSimple_Conformance(t *testing.T) {
	conformance.Run(t, simple.New(), "../../../fixtures/simple")
}
