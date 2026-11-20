package apilint_test

import (
	"testing"

	"github.com/wasinuddy/ai-b2b-saas-template/tools/apilint"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	dir := analysistest.TestData()
	analysistest.Run(t, dir, apilint.Analyzer,
		"example.com/internal/handler",
		"example.com/internal/model",
	)
}
