// Copyright © 2020 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime/debug"
	"runtime/pprof"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"github.com/thinkgos/strext"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/services/ccs"

	"github.com/thinkgos/jocasta/pkg/izap"
	"github.com/thinkgos/jocasta/services"
)

var (
	execCmd  *exec.Cmd
	hasDebug bool
	server   services.Service
	cfgFile  string
	daemon   bool
	forever  bool
	logfile  string

	cpuProfilingFile,
	memProfilingFile,
	blockProfilingFile,
	goroutineProfilingFile,
	threadcreateProfilingFile *os.File
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "cproxy",
	Short: "proxy tool with command",
	Long:  `proxy tool with command`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cproxy.yaml)")

	// Cobra also supports local flags, which will only run
	// when this action is called directly.
	rootCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")

	rootCmd.PersistentFlags().BoolVar(&hasDebug, "debug", false, "debug log output")
	rootCmd.PersistentFlags().BoolVar(&daemon, "daemon", false, "run in background")
	rootCmd.PersistentFlags().BoolVar(&forever, "forever", false, "run in forever, fail and retry")
	rootCmd.PersistentFlags().StringVar(&logfile, "log", "", "log file path")
	kcp(rootCmd)
	rootCmd.PersistentPreRun = preRun
	rootCmd.PersistentPostRun = postRun
}

func preRun(cmd *cobra.Command, args []string) {
	izap.InitLogger(izap.Config{Level: -1, Adapter: "console", Stack: true})
	log := zap.S()
	izap.SetLevel(zapcore.DebugLevel)

	execName := os.Args[0]

	// daemon运行
	if daemon {
		args := strext.DeleteAll(os.Args[1:], "--daemon")
		execCmd = exec.Command(execName, args...)
		execCmd.Start()
		// TODO: 检查相关进程是否已启动??
		format := "%s PID[ %d ] running...\n"
		if forever {
			format = "%s<forever> PID[ %d ] running...\n"
		}
		log.Infof(format, execName, execCmd.Process.Pid)

		os.Exit(0)
	}

	//set kcp config
	if kcpCfg.Mode != "manual" {
		kcpCfg.NoDelay, kcpCfg.Interval, kcpCfg.Resend, kcpCfg.NoCongestion = ccs.SKcpMode(kcpCfg.Mode)
	}
	if !cs.HasKcpBlockCrypt(kcpCfg.Method) {
		kcpCfg.Method = "aes"
	}
	kcpCfg.Block, _ = cs.NewKcpBlockCryptWithPbkdf2(kcpCfg.Method, kcpCfg.Key, "thinkgos-goproxy")

	if hasDebug {
		cpuProfilingFile, _ = os.Create("cpu.prof")
		memProfilingFile, _ = os.Create("memory.prof")
		blockProfilingFile, _ = os.Create("block.prof")
		goroutineProfilingFile, _ = os.Create("goroutine.prof")
		threadcreateProfilingFile, _ = os.Create("threadcreate.prof")
		pprof.StartCPUProfile(cpuProfilingFile)
		if logfile == "" {
			log.Infof("[profiling] cpu profiling save to file : cpu.prof")
			log.Infof("[profiling] memory profiling save to file : memory.prof")
			log.Infof("[profiling] block profiling save to file : block.prof")
			log.Infof("[profiling] goroutine profiling save to file : goroutine.prof")
			log.Infof("[profiling] threadcreate profiling save to file : threadcreate.prof")
		}
	}
}

func postRun(cmd *cobra.Command, args []string) {
	log := zap.S()

	execName := os.Args[0]

	if forever {
		go func() {
			args := strext.DeleteAll(os.Args[1:], "--forever")
			for {
				if execCmd != nil {
					execCmd.Process.Kill()
					time.Sleep(time.Second * 5)
				}
				execCmd = exec.Command(execName, args...)

				cmdReaderStderr, err := execCmd.StderrPipe()
				if err != nil {
					log.Errorf("[** forever **] stderr pipe, %s, restarting...\n", err)
					continue
				}
				cmdReader, err := execCmd.StdoutPipe()
				if err != nil {
					log.Errorf("[** forever **] stdout pipe, %s, restarting...\n", err)
					continue
				}

				go func() {
					defer func() {
						if err := recover(); err != nil {
							log.Errorf("[** forever **] crashed, %s\nstack:%s", err, string(debug.Stack()))
						}
					}()
					scanner := bufio.NewScanner(cmdReader)
					for scanner.Scan() {
						log.Infof(scanner.Text())
					}
				}()
				go func() {
					defer func() {
						if err := recover(); err != nil {
							log.Errorf("[** forever **] crashed, %s\nstack:%s", err, string(debug.Stack()))
						}
					}()
					scannerStdErr := bufio.NewScanner(cmdReaderStderr)
					for scannerStdErr.Scan() {
						log.Infof(scannerStdErr.Text())
					}
				}()

				if err := execCmd.Start(); err != nil {
					log.Errorf("[** forever **] child process start failed, %s, restarting...\n", err)
					continue
				}
				pid := execCmd.Process.Pid

				log.Infof("[** forever **] worker %s PID[ %d ] running...\n", execName, pid)
				if err := execCmd.Wait(); err != nil {
					log.Errorf("[** forever **] parent process wait, %s, restarting...", err)
					continue
				}
				log.Infof("[** forever **] worker %s PID[ %d ] unexpected exited, restarting...\n", execName, pid)
			}
		}()
	} else if server == nil {
		return
	}

	sysSignal := make(chan os.Signal, 1)
	signal.Notify(sysSignal, syscall.SIGHUP, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)

	<-sysSignal

	if server != nil {
		log.Infof("received an interrupt, stopping services...")
		server.Stop()
	}
	if execCmd != nil {
		log.Infof("kill process PID[ %d ]", execCmd.Process.Pid)
		execCmd.Process.Kill()
	}
	if hasDebug {
		SaveProfiling()
	}
}

func SaveProfiling() {
	pprof.Lookup("goroutine").WriteTo(goroutineProfilingFile, 1)
	pprof.Lookup("heap").WriteTo(memProfilingFile, 1)
	pprof.Lookup("block").WriteTo(blockProfilingFile, 1)
	pprof.Lookup("threadcreate").WriteTo(threadcreateProfilingFile, 1)
	pprof.StopCPUProfile()
}
