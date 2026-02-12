package main

import (
	"github.com/devnullvoid/pvetui/internal/tools/queuedrawcheck"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	singlechecker.Main(queuedrawcheck.Analyzer)
}
