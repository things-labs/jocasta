module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.1.1
	github.com/thinkgos/go-core-package v0.1.10
	github.com/thinkgos/jocasta v0.0.0
	go.uber.org/zap v1.16.0
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad // indirect
	golang.org/x/sys v0.0.0-20201221093633-bc327ba9c2f0 // indirect
)

replace github.com/thinkgos/jocasta => ../
