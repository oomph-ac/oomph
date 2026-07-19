package main

import (
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/oomph-ac/oomph/anticheat/player"
	"github.com/sandertv/gophertunnel/minecraft/text"
)

func recoverPlayerFn(p *player.Player, err any) {
	// Print the entire stack trace into stdout
	//debug.PrintStack()

	hub := sentry.CurrentHub().Clone()
	hub.WithScope(func(scope *sentry.Scope) {
		scope.SetTag("xuid", p.Conn().IdentityData().XUID)
		scope.SetTag("player_ign", p.IdentityDat.DisplayName)
		if c := p.Conn(); c != nil {
			scope.SetTag("client_addr", c.RemoteAddr().String())
		}
	})
	sentryErrID := hub.Recover(err)
	_ = hub.Flush(5 * time.Second)
	p.Disconnect(text.Colourf("<red><bold>An error occured while processing your connection.\nMake a report with the provided error ID.</bold></red>\nError ID: %s", *sentryErrID))
}
