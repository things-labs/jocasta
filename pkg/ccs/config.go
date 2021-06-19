package ccs

import (
	"fmt"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/thinkgos/jocasta/cs"
	"github.com/thinkgos/jocasta/internal/bytesconv"
	"github.com/thinkgos/jocasta/pkg/extssh"
)

// FilterConfig filter config
type FilterConfig struct {
	// 代理模式 default: intelligent
	//      direct 不在blocked都直连
	//      proxy  不在direct都走代理
	//      intelligent blocked和direct都没有,智能判断
	Intelligent string
	ProxyFile   string        // 代理域文件名 default: blocked
	DirectFile  string        // 直连域文件名 default: direct
	Timeout     time.Duration // dial超时时间 default: 3s
	Interval    time.Duration // 域名探测间隔 default: 10s
}

// AuthConfig basic auth 配置
type AuthConfig struct {
	File       string        // 授权文件,一行一条(格式user:password), default empty
	UserPasses []string      // 授权用户密码对, default empty
	URL        string        // 外部认证授权url, default: empty
	Timeout    time.Duration // 外部认证授权超时时间, default: 3s
	OkCode     int           // 外部认证授权成功的code, default: 204
	Retry      uint          // 外部认证授权重试次数, default: 1
}

// DNSConfig 自定义dns服务
type DNSConfig struct {
	Addr string // dns 解析服务器地址 default: empty
	TTL  int    // dns 解析结果缓存时间,单位秒 default: 300s
}

// LbConfig loadbalance config
type LbConfig struct {
	Method     string        // 负载均衡方法, random|roundrobin|leastconn|hash|addrhash|leasttime|weight default: roundrobin
	Timeout    time.Duration // 负载均衡dial超时时间 default 500ms
	RetryTime  time.Duration // 负载均衡重试时间间隔 default 1000ms
	HashTarget bool          // hash方法时,选择hash的目标, default: false
}

// SSHConfig ssh config
type SSHConfig struct {
	User        string // ssh 用户 default: empty
	Password    string // ssh 密码 default: empty
	KeyFile     string // ssh 私有key文件 default: empty
	KeyFileSalt string // ssh 私有key加盐 default: empty
}

// Parse parse SSHConfig
func (sf *SSHConfig) Parse() (ssh.AuthMethod, error) {
	if sf.User == "" {
		return nil, fmt.Errorf("user required")
	}
	if sf.KeyFile == "" && sf.Password == "" {
		return nil, fmt.Errorf("password or key file required")
	}

	if sf.Password != "" {
		return ssh.Password(sf.Password), nil
	}
	if sf.KeyFileSalt != "" {
		return extssh.LoadPrivateKey2AuthMethod(sf.KeyFile, bytesconv.Str2Bytes(sf.KeyFileSalt))
	}
	return extssh.LoadPrivateKey2AuthMethod(sf.KeyFile)
}

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
