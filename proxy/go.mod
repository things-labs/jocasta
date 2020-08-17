module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/thinkgos/jocasta v0.0.0-20200811005104-fbf0f8bdd363
	github.com/thinkgos/strext v0.3.2
	github.com/xtaci/kcp-go/v5 v5.5.15
	go.uber.org/zap v1.15.0
	golang.org/x/crypto v0.0.0-20200728195943-123391ffb6de
)

replace github.com/thinkgos/jocasta => ../
