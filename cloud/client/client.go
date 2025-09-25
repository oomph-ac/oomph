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
	log     *slog.Logger
	conn    net.Conn
	netOpts NetworkOpts

	batched  []packet.Packet
	wPackets chan packet.Packet
	rPackets chan packet.Packet

	closed atomic.Bool

	done chan struct{}
	once sync.Once
}

func New(log *slog.Logger, conn net.Conn, cd NetworkOpts) *Client {
	if cd.FlushRate < 50*time.Millisecond {
		cd.FlushRate = 50 * time.Millisecond
	}
	c := &Client{
		log:     log,
		conn:    conn,
		netOpts: cd,

		batched:  make([]packet.Packet, 0, 64),
		wPackets: make(chan packet.Packet, 128),
		rPackets: make(chan packet.Packet, 32),

		done: make(chan struct{}),
	}
	c.closed.Store(false)
	go c.readLoop()
	go c.writeLoop()
	return c
}

func (c *Client) Conn() net.Conn {
	return c.conn
}

func (c *Client) Closed() bool {
	return c.closed.Load()
}

func (c *Client) Close() {
	c.once.Do(func() {
		c.closed.Store(true)
		close(c.done)
		if conn := c.conn; conn != nil {
			_ = conn.Close()
			c.conn = nil
		}
	})
}
