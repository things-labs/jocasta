package logger

import (
	"log"
	"sync/atomic"
)

type Logger interface {
	Debugf(format string, arg ...interface{})
	Infof(format string, arg ...interface{})
	Warnf(format string, arg ...interface{})
	Errorf(format string, arg ...interface{})
	DPanicf(format string, arg ...interface{})
	Fatalf(format string, arg ...interface{})
}

// 标准输出
type Std struct {
	*log.Logger
	has uint32
}

var _ Logger = (*Std)(nil)

type Option func(std *Std)

func WithEnable(enable bool) Option {
	return func(std *Std) {
		std.Mode(enable)
	}
}

func New(l *log.Logger, opts ...Option) *Std {
	s := &Std{l, 0}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (sf *Std) Mode(enable bool) {
	if enable {
		atomic.StoreUint32(&sf.has, 1)
	} else {
		atomic.StoreUint32(&sf.has, 0)
	}
}

func (sf Std) Debugf(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[D]: "+format, args...)
	}
}

func (sf Std) Infof(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[I]: "+format, args...)
	}
}

func (sf Std) Errorf(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[E]: "+format, args...)
	}
}

func (sf Std) Warnf(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[W]: "+format, args...)
	}
}

func (sf Std) DPanicf(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[P]: "+format, args...)
	}
}

func (sf Std) Fatalf(format string, args ...interface{}) {
	if atomic.LoadUint32(&sf.has) == 1 {
		sf.Logger.Printf("[F]: "+format, args...)
	}
}
