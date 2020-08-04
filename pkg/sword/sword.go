package sword

import (
	"runtime/debug"

	"github.com/go-playground/validator/v10"
	"github.com/panjf2000/ants/v2"
	"go.uber.org/zap"

	"github.com/thinkgos/jocasta/core/binding"
	"github.com/thinkgos/jocasta/pkg/izap"
)

// BindingSize binding buffer size
const BindingSize = 2048

// GPool goroutine pool
var GPool, _ = ants.NewPool(500000)

// Submit submit function f to done
func Submit(f func()) {
	if GPool.Submit(f) != nil {
		go func() {
			defer func() {
				if err := recover(); err != nil {
					izap.Logger.DPanic("sword gpool submit", zap.Any("crashed", err), zap.ByteString("stack", debug.Stack()))
				}
			}()
			f()
		}()
	}
}

// Binding binding
var Binding = binding.New(BindingSize, binding.WithGPool(GPool))

// Validate validator
var Validate = validator.New()
