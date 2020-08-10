package sword

import (
	"github.com/go-playground/validator/v10"

	"github.com/thinkgos/jocasta/core/binding"
)

// BindingSize binding buffer size
const BindingSize = 2048

// Binding binding
var Binding = binding.New(BindingSize, binding.WithGPool(&GPool))

// Validate validator
var Validate = validator.New()
