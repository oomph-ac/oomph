package oerror

type OomphError struct {
	Err string
}

func NewOomphError(err string) *OomphError {
	return &OomphError{Err: err}
}

func (e *OomphError) Error() string {
	return e.Err
}
