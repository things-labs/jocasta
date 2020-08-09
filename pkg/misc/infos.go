package misc

import (
	"fmt"
	"runtime"

	"github.com/denisbrodbeck/machineid"
	"github.com/thinkgos/jocasta/builder"
)

const Author = "thinkgos"

func PrintVersion() {
	mid, _ := machineid.ID()
	fmt.Printf("Author: %s\r\n", Author)
	fmt.Printf("Model: %s\r\n", builder.Model)
	fmt.Printf("Version: %s\r\n", builder.Version)
	fmt.Printf("API version: %s\r\n", builder.APIVersion)
	fmt.Printf("Go version: %s\r\n", runtime.Version())
	fmt.Printf("Git commit: %s\r\n", builder.GitCommit)
	fmt.Printf("Git full commit: %s\r\n", builder.GitFullCommit)
	fmt.Printf("Build time: %s\r\n", builder.BuildTime)
	fmt.Printf("OS/Arch: %s/%s\r\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("Machine id: %s\r\n", mid)
}
