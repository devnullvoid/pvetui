package queuedrawcheck

import (
	"go/ast"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports nested QueueUpdateDraw calls, which are a known source of
// tview deadlocks when callbacks re-enter UI update queues.
var Analyzer = &analysis.Analyzer{
	Name: "queuedrawcheck",
	Doc:  "reports QueueUpdateDraw calls inside QueueUpdateDraw callbacks",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	for _, file := range pass.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			outerCall, ok := n.(*ast.CallExpr)
			if !ok || !isQueueUpdateDrawCall(outerCall) || len(outerCall.Args) == 0 {
				return true
			}

			fnLit, ok := outerCall.Args[0].(*ast.FuncLit)
			if !ok {
				return true
			}

			hasNested := false
			ast.Inspect(fnLit.Body, func(inner ast.Node) bool {
				// Do not inspect nested function literals (e.g. goroutines declared
				// inside the callback). They execute in separate contexts and should
				// be analyzed at their own call sites.
				if _, ok := inner.(*ast.FuncLit); ok {
					return false
				}

				innerCall, ok := inner.(*ast.CallExpr)
				if !ok {
					return true
				}
				if isQueueUpdateDrawCall(innerCall) {
					hasNested = true
					pass.Reportf(innerCall.Pos(), "nested QueueUpdateDraw inside QueueUpdateDraw callback can deadlock tview")
					return false
				}
				return true
			})

			// If no nested call was found, continue scanning sibling nodes.
			if !hasNested {
				return true
			}
			return true
		})
	}

	return nil, nil
}

func isQueueUpdateDrawCall(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	return selector.Sel != nil && selector.Sel.Name == "QueueUpdateDraw"
}
