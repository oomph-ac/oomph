package client

import (
	"log/slog"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/oomph-ac/oomph/cloud/packet"
)

type Client struct {
	conn      *net.TCPConn
	connSetAt time.Time
	connMu    sync.RWMutex

	log *slog.Logger

	flushRate    time.Duration
	cmpThreshold uint32

	batched  []packet.Packet
	wPackets chan packet.Packet
	rPackets chan packet.Packet

	// resolveConn is a function that makes a new connection to the cloud server in the case the current one is closed.
	resolveConn func(c *Client)

	closed atomic.Bool
	done   chan struct{}
	once   sync.Once
}

func New(log *slog.Logger, flushRate time.Duration, cmpThreshold uint32, resolveConn func(c *Client)) *Client {
	if flushRate < 50*time.Millisecond {
		flushRate = 50 * time.Millisecond
	}
	c := &Client{
		flushRate:    flushRate,
		cmpThreshold: cmpThreshold,
		resolveConn:  resolveConn,

		wPackets: make(chan packet.Packet, 128),
		rPackets: make(chan packet.Packet, 32),

		log:  log,
		done: make(chan struct{}, 1),
	}
	resolveConn(c)
	c.closed.Store(false)
	go c.readLoop()
	go c.writeLoop()
	go c.checkConnStatus()
	return c
}

func (c *Client) Conn() *net.TCPConn {
	c.connMu.RLock()
	defer c.connMu.RUnlock()

	return c.conn
}

func (c *Client) SetConn(conn *net.TCPConn) {
	c.connMu.Lock()
	defer c.connMu.Unlock()

	c.conn = conn
	c.connSetAt = time.Now()
}

func (c *Client) Close() {
	c.once.Do(func() {
		close(c.done)
		c.closeConn()
	})
}
