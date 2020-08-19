package cs

import (
	"crypto/sha1"
	"errors"
	"fmt"

	"github.com/xtaci/kcp-go/v5"
	"golang.org/x/crypto/pbkdf2"
)

// KcpConfig kcp 配置
type KcpConfig struct {
	// 最大传输单元,协议不负责探测MTU,默认MTU: 1400字节
	MTU int
	// 最大发送窗口,默认32 单位:包
	// 最大接收窗口,默认32 单位:包
	SndWnd, RcvWnd int
	// FEC data shard 默认10
	// FEC parity shard 默认3
	DataShard, ParityShard int
	// 在IPv4头设置6bit DSCP域,
	// 或在IPv6头设置 8bit Traffic Class.
	// 默认0
	DSCP int
	// 接收到每一个包立即进行应答,默认true
	AckNodelay bool
	// 工作模式参数
	// 是否启用nodelay模式 0: 不启用 1:启用
	// 协议内部毫秒,比如10ms或者20ms
	// 快速重传模式, 0关闭,可以设置为2(2次ack跨越将会直接重传)
	// 是否关闭流控制, 0: 不关闭 1: 关闭
	// 例
	// 普通模式: 0,40,2,1 (default)
	// 极速模式: 1,10,2,1
	NoDelay, Interval, Resend, NoCongestion int

	SockBuf   int            // 读写缓存器, 默认 4194304 4M
	KeepAlive int            // TODO: 未用 默认10
	Block     kcp.BlockCrypt // block encryption
}

type blockCryptInfo struct {
	newBlockCrypt func(key []byte) (kcp.BlockCrypt, error)
	keyLen        int
}

var blockCrypts = map[string]blockCryptInfo{
	"sm4":      {kcp.NewSM4BlockCrypt, 16},
	"tea":      {kcp.NewTEABlockCrypt, 16},
	"xor":      {kcp.NewSimpleXORBlockCrypt, 32},
	"none":     {kcp.NewNoneBlockCrypt, 32},
	"aes-128":  {kcp.NewAESBlockCrypt, 16},
	"aes-192":  {kcp.NewAESBlockCrypt, 24},
	"blowfish": {kcp.NewBlowfishBlockCrypt, 32},
	"twofish":  {kcp.NewTwofishBlockCrypt, 32},
	"cast5":    {kcp.NewCast5BlockCrypt, 16},
	"3des":     {kcp.NewTripleDESBlockCrypt, 24},
	"xtea":     {kcp.NewXTEABlockCrypt, 16},
	"salsa20":  {kcp.NewSalsa20BlockCrypt, 32},
	"aes":      {kcp.NewAESBlockCrypt, 32},
}

// NewKcpBlockCrypt 根据method和key生成kcp.BlockCrypt
// Note: key大于或等于对应加密方法的key长度
func NewKcpBlockCrypt(method string, key []byte) (kcp.BlockCrypt, error) {
	bc, ok := blockCrypts[method]
	if !ok {
		return nil, errors.New("not support block crypt method")
	}
	if len(key) < bc.keyLen {
		return nil, fmt.Errorf("key length expect %d but %d", bc.keyLen, len(key))
	}
	return bc.newBlockCrypt(key[:bc.keyLen])
}

// NewKcpBlockCryptWithPbkdf2 使用pbkdf2(给的password和salt)生成所需的key,通过指定method生成kcp.BlockCrypt
func NewKcpBlockCryptWithPbkdf2(method, password, salt string) (kcp.BlockCrypt, error) {
	return NewKcpBlockCrypt(method, pbkdf2.Key([]byte(password), []byte(salt), 4096, 32, sha1.New))
}

// HasKcpBlockCrypt 是否支持指定的加密方法
func HasKcpBlockCrypt(method string) (ok bool) {
	_, ok = blockCrypts[method]
	return
}

// KcpBlockCryptMethods 获得支持kcp所有加密方法
func KcpBlockCryptMethods() []string {
	keys := make([]string, 0, len(blockCrypts))
	for key := range blockCrypts {
		keys = append(keys, key)
	}
	return keys
}
