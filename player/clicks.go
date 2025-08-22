package player

import "github.com/sandertv/gophertunnel/minecraft/protocol"

type ClicksComponent interface {
	HandleAttack(*protocol.UseItemOnEntityTransactionData)
	HandleSwing()
	HandleRight(*protocol.UseItemTransactionData)

	DelayLeft() int64
	DelayRight() int64

	CPSLeft() int64
	CPSRight() int64

	AddLeftHook(hook ClickHook)
	AddRightHook(hook ClickHook)

	Tick()
}

type ClickHook func()

func (p *Player) SetClicks(c ClicksComponent) {
	p.clicks = c
}

func (p *Player) Clicks() ClicksComponent {
	return p.clicks
}
