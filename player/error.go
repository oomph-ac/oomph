package player

import (
	"fmt"
	"time"

	"github.com/getsentry/sentry-go"
)

func (p *Player) recoverError() {
	if v := recover(); v != nil {
		hub := sentry.CurrentHub().Clone()
		hub.Recover(v)
		hub.Flush(time.Second * 5)

		p.log.Errorf("%v", v)
		if p.conn != nil {
			// Disconnect the player if oomph encounters an error
			p.Disconnect(fmt.Sprintf("Proxy encountered an error:\n%v", v))
		}
	}
}
