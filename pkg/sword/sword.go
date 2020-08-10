package sword

import (
	"github.com/go-playground/validator/v10"
	"github.com/panjf2000/ants/v2"

	"github.com/thinkgos/jocasta/core/binding"
)

// BindingSize binding buffer size
const BindingSize = 4096

// Binding binding
var Binding = binding.New(BindingSize, binding.WithGPool(&GPool))

// Validate validator
var Validate = validator.New()

// AntsPool ants pool instance
var AntsPool, _ = ants.NewPool(500000)
