package component

import (
	"sync/atomic"

	"github.com/chewxy/math32"
	"github.com/df-mc/dragonfly/server/item"
	"github.com/oomph-ac/oomph/oerror"
	"github.com/oomph-ac/oomph/player"
)

type invReq struct {
	id         int32
	prev, next *invReq
	actions    []invAction
}

func newInvRequest(reqID int32) *invReq {
	return &invReq{
		id:      reqID,
		actions: make([]invAction, 0, 2),
	}
}

func (req *invReq) append(action invAction) {
	req.actions = append(req.actions, action)
}

func (req *invReq) execute() {
	for _, action := range req.actions {
		action.execute()
	}
}

func (req *invReq) accept() {
	for _, action := range req.actions {
		action.close()
	}

	/* if req.next != nil {
		// Remove the request from the chain.
		req.next.prev = nil
		req.next = nil
	} */

	if req.prev != nil {
		panic(oerror.New("request was accepted out of order (prev=%d curr=%d)", req.prev.id, req.id))
	}
}

func (req *invReq) reject() {
	for i := len(req.actions) - 1; i >= 0; i-- {
		req.actions[i].revert()
		req.actions[i].close()
	}

	// If this request was rejected, we need to replay all other transactions after this one.
	if req.next != nil {
		req.next.execute()
	}
}

type invAction interface {
	execute()
	revert()
	close()
}

type transferAction struct {
	count int

	srcInv  int32
	srcSlot int
	srcItem item.Stack

	dstInv  int32
	dstSlot int
	dstItem item.Stack

	mPlayer atomic.Pointer[player.Player]
}

func newInvTransferAction(
	count int,
	srcInv int32,
	srcSlot int,
	srcItem item.Stack,
	dstInv int32,
	dstSlot int,
	dstItem item.Stack,
	mPlayer *player.Player,
) *transferAction {
	a := &transferAction{
		count:   count,
		srcInv:  srcInv,
		srcSlot: srcSlot,
		srcItem: srcItem,
		dstInv:  dstInv,
		dstSlot: dstSlot,
		dstItem: dstItem,
	}
	a.mPlayer.Store(mPlayer)
	return a
}

func (a *transferAction) close() {
	a.mPlayer.Store(nil)
}

func (a *transferAction) execute() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	srcInv, foundSrcInv := mPlayer.Inventory().WindowFromContainerID(int32(a.srcInv))
	if !foundSrcInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}

	dstInv, foundDstInv := mPlayer.Inventory().WindowFromContainerID(int32(a.dstInv))
	if !foundDstInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.dstInv)
		return
	}

	if a.srcItem.Empty() {
		mPlayer.Log().Debugf("unexpected empty source item")
		return
	}
	dstItem := a.dstItem
	if dstItem.Empty() {
		dstItem = a.srcItem.Grow(-math32.MaxInt32)
	}

	srcInv.SetSlot(a.srcSlot, a.srcItem.Grow(-a.count))
	dstInv.SetSlot(a.dstSlot, dstItem.Grow(a.count))
}

func (a *transferAction) revert() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	srcInv, foundSrcInv := mPlayer.Inventory().WindowFromContainerID(int32(a.srcInv))
	if !foundSrcInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}

	dstInv, foundDstInv := mPlayer.Inventory().WindowFromContainerID(int32(a.dstInv))
	if !foundDstInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.dstInv)
		return
	}

	srcInv.SetSlot(a.srcSlot, a.srcItem)
	dstInv.SetSlot(a.dstSlot, a.dstItem)
}

type swapAction struct {
	srcInv  int32
	srcItem item.Stack
	srcSlot int

	dstInv  int32
	dstItem item.Stack
	dstSlot int

	mPlayer atomic.Pointer[player.Player]
}

func newInvSwapAction(
	srcInv int32,
	srcItem item.Stack,
	srcSlot int,
	dstInv int32,
	dstItem item.Stack,
	dstSlot int,
	mPlayer *player.Player,
) *swapAction {
	a := &swapAction{
		srcInv:  srcInv,
		srcItem: srcItem,
		srcSlot: srcSlot,
		dstInv:  dstInv,
		dstItem: dstItem,
		dstSlot: dstSlot,
	}
	a.mPlayer.Store(mPlayer)
	return a
}

func (a *swapAction) close() {
	a.mPlayer.Store(nil)
}

func (a *swapAction) execute() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	srcInv, foundSrcInv := mPlayer.Inventory().WindowFromContainerID(a.srcInv)
	if !foundSrcInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}

	dstInv, foundDstInv := mPlayer.Inventory().WindowFromContainerID(a.dstInv)
	if !foundDstInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.dstInv)
		return
	}

	srcInv.SetSlot(a.srcSlot, a.dstItem)
	dstInv.SetSlot(a.dstSlot, a.srcItem)
}

func (a *swapAction) revert() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	srcInv, foundSrcInv := mPlayer.Inventory().WindowFromContainerID(a.srcInv)
	if !foundSrcInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}

	dstInv, foundDstInv := mPlayer.Inventory().WindowFromContainerID(a.dstInv)
	if !foundDstInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.dstInv)
		return
	}

	srcInv.SetSlot(a.srcSlot, a.srcItem)
	dstInv.SetSlot(a.dstSlot, a.dstItem)
}

type destroyAction struct {
	mPlayer atomic.Pointer[player.Player]

	srcItem item.Stack
	count   int
	srcSlot int
	srcInv  int32
	isDrop  bool
}

func newDestroyAction(
	srcItem item.Stack,
	count int,
	srcSlot int,
	srcInv int32,
	isDrop bool,
	mPlayer *player.Player,
) *destroyAction {
	a := &destroyAction{
		srcItem: srcItem,
		count:   count,
		srcSlot: srcSlot,
		srcInv:  srcInv,
		isDrop:  isDrop,
	}
	a.mPlayer.Store(mPlayer)
	return a
}

func (a *destroyAction) close() {
	a.mPlayer.Store(nil)
}

func (a *destroyAction) execute() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	inv, foundInv := mPlayer.Inventory().WindowFromContainerID(a.srcInv)
	if !foundInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}

	if a.count > a.srcItem.Count() {
		if a.isDrop {
			mPlayer.Log().Debugf("attempted to drop %d items, but only %d are available", a.count, a.srcItem.Count())
		} else {
			mPlayer.Log().Debugf("attempted to destroy %d items, but only %d are available", a.count, a.srcItem.Count())
		}
		return
	}
	inv.SetSlot(a.srcSlot, a.srcItem.Grow(-a.count))
}

func (a *destroyAction) revert() {
	mPlayer := a.mPlayer.Load()
	if mPlayer == nil {
		return
	}

	inv, foundInv := mPlayer.Inventory().WindowFromContainerID(a.srcInv)
	if !foundInv {
		mPlayer.Log().Debugf("no inventory with container id %d found", a.srcInv)
		return
	}
	inv.SetSlot(a.srcSlot, a.srcItem)
}
