package main

import (
	"bufio"
	"net"
	"sync"
)

const (
	MsgChat  uint8 = 1
	MsgMatch uint8 = 2
)

type matchQueue struct {
	matchMap  map[net.Conn]bool
	queueLock *sync.Mutex
	queueCond *sync.Cond
}

func main() {
	ln, _ := net.Listen("tcp", ":9909")
	defer ln.Close()

	mq := newMatchQueue()
	go matchMaker(mq)

	for {
		conn, _ := ln.Accept()
		go handleConn(conn, mq)
	}
}

func handleConn(conn net.Conn, mq *matchQueue) {
	defer func() {
		mq.queueLock.Lock()
		delete(mq.matchMap, conn)
		mq.queueLock.Unlock()
		conn.Close()
	}()

	reader := bufio.NewReader(conn)
	for {
		head, _ := reader.ReadByte()

		switch head {
		case MsgMatch:
			handleMatch(conn, mq)
		}
	}
}

func handleMatch(conn net.Conn, mq *matchQueue) {
	mq.queueLock.Lock()
	if _, exists := mq.matchMap[conn]; exists {
		delete(mq.matchMap, conn)
	} else {
		mq.matchMap[conn] = true
	}
	mq.queueCond.Signal()
	mq.queueLock.Unlock()
}
func newMatchQueue() *matchQueue {
	lock := &sync.Mutex{}
	mq := &matchQueue{
		matchMap:  make(map[net.Conn]bool),
		queueLock: lock,
		queueCond: sync.NewCond(lock),
	}
	return mq
}
func matchMaker(mq *matchQueue) {
	for {
		mq.queueLock.Lock()
		for len(mq.matchMap) < 2 {
			mq.queueCond.Wait()
		}

		var p1, p2 net.Conn

		for conn := range mq.matchMap {
			if p1 == nil {
				p1 = conn
			} else {
				p2 = conn
				break
			}
		}
		delete(mq.matchMap, p1)
		delete(mq.matchMap, p2)
		mq.queueLock.Unlock()
	}
}
