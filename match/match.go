package match

import (
	"fmt"
	"net"
	"sync"
)

type Match struct {
	queue map[net.Conn]bool
	lock  *sync.Mutex
	cond  *sync.Cond
}

func NewMatch() *Match {
	lock := &sync.Mutex{}
	return &Match{
		queue: make(map[net.Conn]bool),
		lock:  lock,
		cond:  sync.NewCond(lock),
	}
}

func (m *Match) Toggle(conn net.Conn) bool {
	m.lock.Lock()
	result := false
	if _, ok := m.queue[conn]; ok {
		delete(m.queue, conn)
	} else {
		m.queue[conn] = true
		result = true
	}
	m.cond.Signal()
	m.lock.Unlock()
	return result
}

func (m *Match) MatchMaker() {
	for {
		m.lock.Lock()
		for len(m.queue) < 2 {
			m.cond.Wait()
		}
		var p1, p2 net.Conn
		for conn := range m.queue {
			if p1 == nil {
				p1 = conn
			} else {
				p2 = conn
				break
			}
		}
		fmt.Println("match!", p1, p2)
		delete(m.queue, p1)
		delete(m.queue, p2)

		p1.Write([]byte{2})
		p1.Write([]byte("Matched! You are Player 1.\n"))
		p2.Write([]byte{2})
		p2.Write([]byte("Matched! You are Player 2.\n"))
		m.lock.Unlock()
	}
}
