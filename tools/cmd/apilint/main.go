package main

import (
	"github.com/wasinuddy/ai-b2b-saas-template/tools/apilint"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(apilint.Analyzer)
}
