package player

// nopLogger is a no-op implementation of the Logger interface to suppress logging on worlds/expected missing chunks.
type nopLogger struct{}

func (n nopLogger) Debugf(string, ...interface{}) {}
func (n nopLogger) Infof(string, ...interface{})  {}
func (n nopLogger) Errorf(string, ...interface{}) {}
func (n nopLogger) Fatalf(string, ...interface{}) {}
