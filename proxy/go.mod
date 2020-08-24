module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/thinkgos/go-core-package v0.0.3
	github.com/thinkgos/jocasta v0.0.0-20200823125400-e32a09de1a1e
	github.com/thinkgos/strext v0.3.2
	go.uber.org/zap v1.15.0
	golang.org/x/net v0.0.0-20200822124328-c89045814202 // indirect
	golang.org/x/tools v0.0.0-20200823205832-c024452afbcd // indirect
)

replace github.com/thinkgos/jocasta => ../
