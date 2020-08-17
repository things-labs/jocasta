package ccs

import (
	"github.com/thinkgos/jocasta/cs"
)

// SKCPConfig kcp full config
type SKCPConfig struct {
	// 加密的方法
	// sm4,tea,xor,none,aes-128,aes-192,blowfish,twofish,cast5,3des,xtea,salsa20,aes
	// 默认aes
	Method string
	// 加密的key
	Key string
	// 工作模式: 对应工作模式参数,见下面工作模式参数
	// normal:  0, 40, 2, 1
	// fast:  0, 30, 2, 1
	// fast2:  1, 20, 2, 1
	// fast3:  1, 10, 2, 1
	Mode string

	cs.KcpConfig
}

// SKcpMode KCP 工作模式
// mode support:
// 		normal(default)
// 		fast
// 		fast2
// 		fast3
func SKcpMode(mode string) (noDelay int, interval int, resend int, noCongestion int) {
	switch mode {
	case "fast":
		return 0, 30, 2, 1
	case "fast2":
		return 1, 20, 2, 1
	case "fast3":
		return 1, 10, 2, 1
	default: // "normal"
		return 0, 40, 2, 1
	}
}
