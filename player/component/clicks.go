package component

import (
	"github.com/oomph-ac/oomph/player"
	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol"
)

type ClicksComponent struct {
	mPlayer *player.Player

	clicksLeft  *utils.CircularQueue[int64]
	clicksRight *utils.CircularQueue[int64]

	delayLeft  int64
	delayRight int64

	lastLeftClick  int64
	lastRightClick int64

	cpsLeft  int64
	cpsRight int64

	hooksLeft  []player.ClickHook
	hooksRight []player.ClickHook
}

func NewClicksComponent(p *player.Player) *ClicksComponent {
	return &ClicksComponent{
		mPlayer:     p,
		clicksLeft:  utils.NewCircularQueue(player.TicksPerSecond, func() int64 { return 0 }),
		clicksRight: utils.NewCircularQueue(player.TicksPerSecond, func() int64 { return 0 }),
	}
}

func (c *ClicksComponent) HandleAttack(dat *protocol.UseItemOnEntityTransactionData) {
	if dat.ActionType == protocol.UseItemOnEntityActionAttack {
		c.clickLeft()
	}
}

func (c *ClicksComponent) HandleSwing() {
	c.clickLeft()
}

func (c *ClicksComponent) HandleRight(dat *protocol.UseItemTransactionData) {
	// On versions before 1.21.20, we cannot determine if the right click action was caused by a player input or due to MCBE's assisted simulation actions.
	// This isn't much of a problem for Oomph specifically - as it *officially* supports 1.21.20+
	if !c.mPlayer.VersionInRange(player.GameVersion1_21_20, protocol.CurrentProtocol) || dat.TriggerType != protocol.TriggerTypePlayerInput {
		return
	}
	c.clickRight()
}

func (c *ClicksComponent) DelayLeft() int64 {
	return c.delayLeft
}

func (c *ClicksComponent) DelayRight() int64 {
	return c.delayRight
}

func (c *ClicksComponent) CPSLeft() int64 {
	return c.cpsLeft
}

func (c *ClicksComponent) CPSRight() int64 {
	return c.cpsRight
}

func (c *ClicksComponent) AddLeftHook(hook player.ClickHook) {
	c.hooksLeft = append(c.hooksLeft, hook)
}

func (c *ClicksComponent) AddRightHook(hook player.ClickHook) {
	c.hooksRight = append(c.hooksRight, hook)
}

func (c *ClicksComponent) Tick() {
	leftClicksOldest, err := c.clicksLeft.Get(0)
	if err != nil {
		c.mPlayer.Log().Debug("clicksLeft circular queue read failed - resetting values & re-creating click queue")
		c.resetAndPropagateLeft()
	} else {
		c.cpsLeft -= leftClicksOldest
	}

	rightClicksOldest, err := c.clicksRight.Get(0)
	if err != nil {
		c.mPlayer.Log().Debug("clicksRight circular queue read failed - resetting values & re-creating click queue")
		c.resetAndPropagateRight()
	} else {
		c.cpsRight -= rightClicksOldest
	}

	_ = c.clicksLeft.Append(0)
	_ = c.clicksRight.Append(0)
}

func (c *ClicksComponent) clickLeft() {
	index := c.clicksLeft.Size() - 1
	current, err := c.clicksLeft.Get(index)
	if err != nil {
		c.mPlayer.Log().Error("clickLeft: failed to obtain value from queue - resetting values & re-creating click queue")
		c.resetAndPropagateLeft()
		return
	}
	if err := c.clicksLeft.Set(index, current+1); err != nil {
		c.mPlayer.Log().Error("clickLeft: failed to set value in queue - resetting values & re-creating click queue")
		c.resetAndPropagateLeft()
		return
	}
	c.cpsLeft++
	c.delayLeft = c.mPlayer.InputCount - c.lastLeftClick
	c.lastLeftClick = c.mPlayer.InputCount
	for _, hook := range c.hooksLeft {
		hook()
	}
}

func (c *ClicksComponent) clickRight() {
	index := c.clicksRight.Size() - 1
	current, err := c.clicksRight.Get(index)
	if err != nil {
		c.mPlayer.Log().Error("clickRight: failed to obtain value from queue - resetting values & re-creating click queue")
		c.resetAndPropagateRight()
		return
	}
	if err := c.clicksRight.Set(index, current+1); err != nil {
		c.mPlayer.Log().Error("clickRight: failed to set value in queue - resetting values & re-creating click queue")
		c.resetAndPropagateRight()
		return
	}
	c.cpsRight++
	c.delayRight = c.mPlayer.InputCount - c.lastRightClick
	c.lastRightClick = c.mPlayer.InputCount
	for _, hook := range c.hooksRight {
		hook()
	}
}

func (c *ClicksComponent) resetAndPropagateLeft() {
	c.cpsLeft = 0
	c.delayLeft = 0
	c.lastLeftClick = 0
	c.clicksLeft = utils.NewCircularQueue(player.TicksPerSecond, func() int64 { return 0 })
}

func (c *ClicksComponent) resetAndPropagateRight() {
	c.cpsRight = 0
	c.delayRight = 0
	c.lastRightClick = 0
	c.clicksRight = utils.NewCircularQueue(player.TicksPerSecond, func() int64 { return 0 })
}
