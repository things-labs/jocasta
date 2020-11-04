module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.1.1
	github.com/thinkgos/go-core-package v0.1.5
	github.com/thinkgos/jocasta v0.0.0-20200904151820-11ed85782725
	go.uber.org/zap v1.16.0
)

replace github.com/thinkgos/jocasta => ../
