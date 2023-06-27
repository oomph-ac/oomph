package player

import (
	"fmt"
	"math"

	"github.com/oomph-ac/oomph/utils"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

type PacketBuffer struct {
	Packets   []packet.Packet
	Input     *packet.PlayerAuthInput
	Frame     uint64
	Completed bool
}

func (b *PacketBuffer) Add(pk packet.Packet) {
	if i, ok := pk.(*packet.PlayerAuthInput); ok {
		b.Input = i
		b.Frame = i.Tick
		b.Completed = true
		return
	}
	b.Packets = append(b.Packets, pk)
}

// handlePacketQueue handles the packet queue for the player. This is a function that is called every tick, and
// will use the next available (valid) buffer, or fill for movement with the player's last sent input.
func (p *Player) handlePacketQueue() {
	var doDouble bool

	if !p.usePacketBuffer {
		return
	}

	if !p.ready || p.dead {
		p.expectedFrame = 0
		p.isBufferReady = false
		p.isBufferStarted = false
		return
	}

	if !p.isBufferReady {
		return
	}

	if !p.isBufferStarted {
		for len(p.bufferQueue) > 2 {
			b := p.bufferQueue[0]
			p.bufferQueue = p.bufferQueue[1:]
			p.expectedFrame = b.Frame + 1

			p.flushBuffer(b, true)
		}
	}

	// Increase the amount of credits by 1 every tick that the buffer is ready.
	p.queueCredits++

	var buffer, ignoredBuffer *PacketBuffer
	var ignored uint64
	startLen, startCredits := len(p.bufferQueue), p.queueCredits

	for p.queueCredits > 0 {
		nextBuffer, canContinue := p.getNextBuffer()
		if nextBuffer == nil {
			break
		}

		p.isBufferStarted = true
		p.queueCredits--

		if nextBuffer.Frame < p.expectedFrame {
			ignoredBuffer = nextBuffer
			p.flushBuffer(nextBuffer, false)
			p.mInfo.LastUsedInput = nextBuffer.Input
			ignored++

			if !canContinue || p.queueCredits == 0 {
				break
			}

			continue
		}

		buffer = nextBuffer
		break
	}
	p.expectedFrame++

	// The client ran at a point <20 TPS, this should account for that by setting the expected frame to the next
	// tick that the client should be on.
	if startCredits > 1 && startLen == 1 && buffer == nil {
		p.expectedFrame = ignoredBuffer.Frame + 1
		p.queueCredits = 0
	}

	p.validateBufferSize()
	if p.excessBufferScore > 80 {
		doDouble = true
		p.excessBufferScore = 0
	}

	if p.debugger.PacketBuffer {
		p.SendOomphDebug(fmt.Sprint("bufferSize=", startLen, " credits=", p.queueCredits, " ignored=", ignored, " doDouble=", doDouble, " filling=", buffer == nil), packet.TextTypeChat)
	}

	if buffer == nil {
		if !p.isBufferReady {
			return
		}

		if p.mInfo.LastUsedInput == nil {
			// This should never happen anyway, but I don't need a whole server to crash in the case it does.
			p.Disconnect("ERROR: No previous input available to fill - disconnecting.")
			return
		}
		input := *p.mInfo.LastUsedInput
		if ignoredBuffer != nil {
			input = *ignoredBuffer.Input
		}

		input.Tick = p.expectedFrame
		p.clientFrame.Store(p.expectedFrame)

		p.handlePlayerAuthInput(&input)
		p.sendPacketToServer(&input)

		return
	}

	p.flushBuffer(buffer, true)
	if doDouble {
		p.handlePacketQueue()
	}
}

func (p *Player) flushBuffer(b *PacketBuffer, input bool) {
	for _, pk := range b.Packets {
		if p.ClientProcess(pk) {
			continue
		}

		p.sendPacketToServer(pk)
	}

	if !input {
		return
	}

	p.ClientProcess(b.Input)
	p.sendPacketToServer(b.Input)
}

func (p *Player) validateBufferSize() {
	size := len(p.bufferQueue)
	if size <= 1 {
		p.excessBufferScore = math.Max(0, p.excessBufferScore-2)
		return
	}

	p.excessBufferScore++
	p.depletedBufferScore = 0

	if size >= 20 {
		p.Disconnect("Too many packets are being received - try restarting your game?")
	}
}

func (p *Player) flushUnusedBuffers() {
	p.buffListMu.Lock()
	defer p.buffListMu.Unlock()

	for _, b := range p.bufferQueue {
		p.flushBuffer(b, !p.ready)
	}
	p.bufferQueue = make([]*PacketBuffer, 0)

	p.flushBuffer(p.packetBuffer, false)
	p.packetBuffer = NewPacketBuffer()
}

func (p *Player) getNextBuffer() (buffer *PacketBuffer, canContinue bool) {
	p.buffListMu.Lock()
	defer p.buffListMu.Unlock()

	if len(p.bufferQueue) == 0 {
		canContinue = false
		return
	}

	buffer, canContinue = p.bufferQueue[0], true
	p.bufferQueue = p.bufferQueue[1:]

	return
}

func (p *Player) QueuePacket(pk packet.Packet) (send bool) {
	if !p.ready || p.dead {
		return !p.ClientProcess(pk)
	}

	p.packetBuffer.Add(pk)
	if !p.packetBuffer.Completed {
		return false
	}

	p.buffListMu.Lock()
	defer p.buffListMu.Unlock()

	p.bufferQueue = append(p.bufferQueue, p.packetBuffer)
	p.packetBuffer = NewPacketBuffer()

	if len(p.bufferQueue) > 1 && !p.isBufferReady {
		p.isBufferReady = true
		p.expectedFrame = p.bufferQueue[0].Frame
	}

	return false
}

func (p *Player) UsesPacketBuffer() bool {
	return p.usePacketBuffer
}

func (p *Player) UsePacketBuffering(b bool) {
	if p.movementMode != utils.ModeFullAuthoritative {
		b = false
	}

	if p.usePacketBuffer && !b {
		p.flushUnusedBuffers()
		p.isBufferReady = false
	}

	p.usePacketBuffer = b
}

// sendPacketToServer sends a packet to the server
func (p *Player) sendPacketToServer(pk packet.Packet) {
	if p.serverConn == nil {
		p.tMu.Lock()
		p.toSend = append(p.toSend, pk)
		p.tMu.Unlock()

		return
	}

	p.serverConn.WritePacket(pk)
}

func NewPacketBuffer() *PacketBuffer {
	return &PacketBuffer{
		Packets:   make([]packet.Packet, 0),
		Frame:     0,
		Completed: false,
	}
}
