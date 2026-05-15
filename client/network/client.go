package network

import (
	"encoding/binary"
	"fmt"
	"io"
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
	Scene        byte
	SceneCommand byte
	Data         []byte
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

// 기본적으로 [1 바이트] [1 바이트] [4 바이트] [? 바이트] 형태로 패킷이 전송됩니다.
// [Scene] [SceneCommand] [DataLength] [Data...]
func (c *Client) Readloop() {
	for {
		// 1단계: 헤더 6bytes 무조건 확보
		header := make([]byte, 6)
		_, err := io.ReadFull(c.conn, header)
		if err != nil {
			c.Events <- Event{Data: nil}
			return
		}

		scene := header[0]
		sceneCommand := header[1]
		dataLength := binary.BigEndian.Uint32(header[2:6])

		// 2단계: Data 부분 확보
		data := make([]byte, dataLength)
		_, err = io.ReadFull(c.conn, data)
		if err != nil {
			c.Events <- Event{Data: nil}
			return
		}

		c.Events <- Event{
			Data:         data,
			Scene:        scene,
			SceneCommand: sceneCommand,
		}
	}
}
