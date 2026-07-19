package player

func (p *Player) recoverError() {
	if recvFn := p.recoverFunc; recvFn != nil {
		if v := recover(); v != nil {
			recvFn(p, v)
		}
	}
}
