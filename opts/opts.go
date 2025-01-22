package opts

import "os"

type RuntimeOpts struct {
	APIToken string
}

var Global = RuntimeOpts{
	APIToken: os.Getenv("OOMPH_OPT_API_TOKEN"),
}
