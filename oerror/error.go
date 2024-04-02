package oerror

import "fmt"

type OomphError struct {
	Err string
}

func New(err string, a ...any) *OomphError {
	return &OomphError{Err: fmt.Sprintf(err, a...)}
}

func (e *OomphError) Error() string {
	return e.Err
}
