package client

import "time"

func (c *Client) checkConnStatus() {
	t := time.NewTicker(10 * time.Second)
	defer t.Stop()

	wasConnected := c.conn != nil
	for {
		select {
		case <-c.done:
			return
		case <-t.C:
			if c.Conn() == nil {
				if wasConnected {
					c.log.Warn("connection to cloud lost, attempting to re-establish")
				}
				wasConnected = false
				if fn := c.resolveConn; fn != nil {
					fn(c)
				}
				if c.Conn() != nil {
					c.log.Info("re-connected to oomph cloud")
					wasConnected = true
				}
			} else {
				wasConnected = true
			}
		}
	}
}
