package simulation

import (
	"fmt"

	"github.com/oomph-ac/oomph/handler"
	"github.com/oomph-ac/oomph/player"
)

type MovementSimulator struct {
}

func (MovementSimulator) Simulate(p *player.Player) {
	movementHandler := p.Handler(handler.HandlerIDMovement).(*handler.MovementHandler)
	fmt.Println(movementHandler.ClientPosition)
}
