module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.1.1
	github.com/things-go/encrypt v0.0.1
	github.com/things-go/x v0.0.4 // indirect
	github.com/thinkgos/jocasta v0.0.0
	github.com/thinkgos/x v0.3.0
	go.uber.org/zap v1.16.0
)

replace github.com/thinkgos/jocasta => ../
