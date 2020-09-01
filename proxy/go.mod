module github.com/thinkgos/jocasta/proxy

go 1.14

require (
	github.com/spf13/cobra v1.0.0
	github.com/spf13/pflag v1.0.5 // indirect
	github.com/thinkgos/go-core-package v0.0.8
	github.com/thinkgos/jocasta v0.0.0-20200827024558-3c2f7f334f6c
	github.com/thinkgos/strext v0.3.3
	go.uber.org/zap v1.15.0
	golang.org/x/sys v0.0.0-20200831180312-196b9ba8737a // indirect
	golang.org/x/tools v0.0.0-20200831203904-5a2aa26beb65 // indirect
)

replace github.com/thinkgos/jocasta => ../
