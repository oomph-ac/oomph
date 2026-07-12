package player

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/sandertv/gophertunnel/minecraft"
	"github.com/sandertv/gophertunnel/minecraft/protocol/packet"
)

func TestDirectPacketQueueUsesFIFOOrder(t *testing.T) {
	p := newDirectQueueTestPlayer()
	p.enqueueDirectPacket(&packet.Text{Message: "first"})
	p.enqueueDirectPacket(&packet.Text{Message: "second"})

	first, ok := p.popDirectPacket()
	if !ok || first.(*packet.Text).Message != "first" {
		t.Fatalf("first pop = %#v, %v", first, ok)
	}
	second, ok := p.popDirectPacket()
	if !ok || second.(*packet.Text).Message != "second" {
		t.Fatalf("second pop = %#v, %v", second, ok)
	}
	if _, ok := p.popDirectPacket(); ok {
		t.Fatal("empty queue reported a packet")
	}
}

func TestDirectPacketQueueWakesBlockedReader(t *testing.T) {
	p := newDirectQueueTestPlayer()
	result := make(chan packet.Packet, 1)
	go func() {
		pk, _ := p.readDirectPacket()
		result <- pk
	}()

	p.enqueueDirectPacket(&packet.Text{Message: "wake"})
	select {
	case pk := <-result:
		if pk.(*packet.Text).Message != "wake" {
			t.Fatalf("packet = %#v", pk)
		}
	case <-time.After(time.Second):
		t.Fatal("queued packet did not wake blocked reader")
	}
}

func TestDirectPacketQueueSupportsConcurrentProducerAndConsumer(t *testing.T) {
	p := newDirectQueueTestPlayer()
	const count = 1_000
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			p.enqueueDirectPacket(&packet.Text{Message: fmt.Sprint(i)})
		}
	}()
	go func() {
		defer wg.Done()
		for i := 0; i < count; i++ {
			pk, err := p.readDirectPacket()
			if err != nil {
				t.Errorf("readDirectPacket() error = %v", err)
				return
			}
			if got := pk.(*packet.Text).Message; got != fmt.Sprint(i) {
				t.Errorf("packet %d = %q", i, got)
				return
			}
		}
	}()
	wg.Wait()
}

func newDirectQueueTestPlayer() *Player {
	return &Player{
		directNotify: make(chan struct{}, 1),
		directReads:  make(chan packetRead),
	}
}

func TestApplyDirectGameDataUsesDragonflySelfRuntimeID(t *testing.T) {
	p := &Player{}
	data := minecraft.GameData{
		EntityRuntimeID: 42,
		EntityUniqueID:  84,
		PlayerGameMode:  1,
	}

	p.applyDirectGameData(data)

	if p.RuntimeId != 1 {
		t.Fatalf("RuntimeId = %d, want Dragonfly self runtime ID 1", p.RuntimeId)
	}
	if p.UniqueId != 84 || p.GameMode != 1 {
		t.Fatalf("direct game data not applied: unique=%d gamemode=%d", p.UniqueId, p.GameMode)
	}
	if p.GameDat.EntityUniqueID != 84 {
		t.Fatalf("GameDat was not retained: %#v", p.GameDat)
	}
}
