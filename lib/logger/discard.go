package logger

// Discard is an logger on which all Write calls succeed
// without doing anything.
type Discard struct{}

var _ Logger = (*Discard)(nil)

// NewDiscard a discard logger
func NewDiscard() Discard { return Discard{} }

func (sf Discard) Debugf(string, ...interface{})  {}
func (sf Discard) Infof(string, ...interface{})   {}
func (sf Discard) Errorf(string, ...interface{})  {}
func (sf Discard) Warnf(string, ...interface{})   {}
func (sf Discard) DPanicf(string, ...interface{}) {}
func (sf Discard) Fatalf(string, ...interface{})  {}
