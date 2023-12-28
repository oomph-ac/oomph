package game

const (
	ErrorNoInput           = "ERROR #-1: Unable to find previous input, contact admin ASAP."
	ErrorBadAckOrder       = "ERROR #0: Acknowledgements recieved out of order."
	ErrorInvalidInput      = "ERROR #152: Unexpected input recieved - try restarting your game."
	ErrorNoTickSync        = "ERROR #456: Client did not sync tick with server."
	ErrorNoAcks            = "ERROR #654: Client did not respond to acknowledgements."
	ErrorInvalidBlockBreak = "ERROR #777: Invalid block break response recieved."
)
