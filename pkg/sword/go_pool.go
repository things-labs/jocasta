package sword

import (
	"runtime/debug"

	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/pkg/izap"
)

// GPool goroutine pool
var GPool = GoPool{AntsPool}

// Go submit function f to done
func Go(f func()) { GPool.Go(f) }

type GoPool struct {
	pool *ants.Pool
}

func (sf GoPool) Go(f func()) {
	if sf.pool != nil && sf.pool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					izap.Logger.DPanic("sword GoPool Go", zap.Any("crashed", err), zap.ByteString("stack", debug.Stack()))
				}
			}()
			f()
		}()
	}
}

func (sf GoPool) Tune(size int) {
	sf.pool.Tune(size)
}

func (sf GoPool) Running() int {
	return sf.pool.Running()
}

func (sf GoPool) Free() int {
	return sf.pool.Free()
}

func (sf GoPool) Cap() int {
	return sf.pool.Cap()
}
