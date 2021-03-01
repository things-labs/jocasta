package sword

import (
	"runtime/debug"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"
)

// goPool go routine pool
type goPool struct {
	*ants.Pool
}

// Go implement Pool interface
func (sf goPool) Go(f func()) {
	if sf.Pool != nil && sf.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					zap.L().DPanic("sword goPool Go", zap.Any("crashed", err), zap.ByteString("stack", debug.Stack()))
				}
			}()
			f()
		}()
	}
}
