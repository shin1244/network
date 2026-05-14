package network

import (
	"fmt"
	"net"
	"sync"
)

type Client struct {
	addr   string
	conn   net.Conn
	mu     sync.Mutex
	Events chan Event
}

type Event struct {
	Data []byte
}

func NewClient(addr string) *Client {
	return &Client{
		addr:   addr,
		Events: make(chan Event, 1000),
	}
}

func (c *Client) Connect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		return nil
	}

	conn, err := net.Dial("tcp", c.addr)
	if err != nil {
		return err
	}

	c.conn = conn
	return nil
}

func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return nil
	}

	err := c.conn.Close()
	c.conn = nil
	return err
}

func (c *Client) IsConnected() bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	return c.conn != nil
}

func (c *Client) WritePacket(packet []byte) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		return fmt.Errorf("network client is not connected")
	}

	_, err := c.conn.Write(packet)
	return err
}

// [Scene] [SceneCommand] [data]
func (c *Client) Readloop() {
	buffer := make([]byte, 1024)

	for {
		n, err := c.readPacket(buffer)
		if err != nil {
			c.Events <- Event{Data: nil}
			return
		}

		data := make([]byte, n)
		copy(data, buffer[:n])

		c.Events <- Event{Data: data}
	}
}

func (c *Client) readPacket(buffer []byte) (int, error) {
	if c.conn == nil {
		return 0, fmt.Errorf("network client is not connected")
	}

	return c.conn.Read(buffer)
}
