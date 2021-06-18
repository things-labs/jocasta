package builder

import (
	"os"
	"runtime"
	"text/template"

	"github.com/denisbrodbeck/machineid"
)

var (
	// BuildTime 编译日期 由外部ldflags指定
	BuildTime = "unknown"
	// GitCommit git提交版本(短) 由外部ldflags指定
	GitCommit = "unknown"
	// GitFullCommit git提交版本(完整) 由外部ldflags指定
	GitFullCommit = "unknown"
	// Version 版本 由外部ldflags指定
	Version = "unknown"
	// APIVersion api版本 由外部ldflags指定
	APIVersion = "unknown"
	// Model 型号 由外部ldflags指定
	Model = "unknown"
	// Name 应用名称 由外部ldflags指定
	Name = "unknown"
)

const versionTpl = `  Name:             {{.Name}}
  Model:            {{.Model}}
  Version:          {{.Version}}
  API version:      {{.APIVersion}}
  Go version:       {{.GoVersion}}
  Git commit:       {{.GitCommit}}
  Git full commit:  {{.GitFullCommit}}
  Build time:       {{.BuildTime}}
  OS/Arch:          {{.GOOS}}/{{.GOARCH}}
  NumCPU:           {{.NumCPU}}
  MachineID:        {{.MachineID}}
`

// Version 版本信息
type Ver struct {
	Name          string
	Model         string
	Version       string
	APIVersion    string
	GoVersion     string
	GitCommit     string
	GitFullCommit string
	BuildTime     string
	GOOS          string
	GOARCH        string
	NumCPU        int
	MachineID     string
}

// PrintVersion 打印版本信息至os.Stdout
func PrintVersion() {
	mid, _ := machineid.ID()
	v := Ver{
		Name,
		Model,
		Version,
		APIVersion,
		runtime.Version(),
		GitCommit,
		GitFullCommit,
		BuildTime,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.NumCPU(),
		mid,
	}
	template.Must(template.New("version").Parse(versionTpl)).
		Execute(os.Stdout, v) // nolint: errcheck
}
