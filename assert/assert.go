package assert

import "github.com/oomph-ac/oomph/oerror"

func IsTrue(ok bool, message string, args ...interface{}) {
	if !ok {
		panic(oerror.New(message, args...))
	}
}
