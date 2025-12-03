package users

import (
	"net"
	"sync"
)

type Users struct {
	conns map[net.Conn]bool
	lock  sync.Mutex
}

func NewUsers() *Users {
	return &Users{
		conns: make(map[net.Conn]bool),
		lock:  sync.Mutex{},
	}
}

func (u *Users) Add(conn net.Conn) {
	u.lock.Lock()
	u.conns[conn] = true
	u.lock.Unlock()
}

func (u *Users) Remove(conn net.Conn) {
	u.lock.Lock()
	delete(u.conns, conn)
	u.lock.Unlock()
}

func (u *Users) Len() int {
	u.lock.Lock()
	defer u.lock.Unlock()

	return len(u.conns)
}

func (u *Users) Broadcast(msg []byte) {
	u.lock.Lock()
	for c := range u.conns {
		c.Write(msg)
	}
	u.lock.Unlock()
}
