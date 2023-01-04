package dlock

import (
	"context"
	"github.com/sirupsen/logrus"
	"net"
	"sync"
	"time"
)

type internalStatus byte

const (
	onlineStatus internalStatus = iota
	offlineStatus
)

var store *Store

type Store struct {
	list map[string]*Lock
	mx   sync.Mutex
}

func StartLockStore() {
	store = &Store{
		list: make(map[string]*Lock),
		mx:   sync.Mutex{},
	}
}

func (s *Store) getLock(key string) (*Lock, bool) {
	s.mx.Lock()
	defer s.mx.Unlock()
	lock, ok := s.list[key]
	return lock, ok
}

func (s *Store) isFindLock(key string) bool {
	_, ok := s.list[key]
	return ok
}

func (s *Store) NewLock(n *node) {
	s.mx.Lock()
	defer s.mx.Unlock()
	if _, ok := s.list[n.lockKey]; ok {
		return
	}

	lc := &Lock{
		key:     n.lockKey,
		nodeKey: byConnGenerKey(n.conn),
		nodes:   map[string]byte{},
		queue:   make([]*node, 0),
	}
	s.list[lc.key] = lc
	n.isLock = true
	lc.push(n)
	n.write(LockOKEvent)
}

func (s *Store) deleteLock(lock *Lock) {
	if lock == nil {
		return
	}
	s.mx.Lock()
	defer s.mx.Unlock()
	delete(s.list, lock.key)
}

type Lock struct {
	key     string
	nodeKey string
	mx      sync.Mutex
	nodes   map[string]byte
	queue   []*node
}

func (l *Lock) Lock(n *node) {
	if l.isNode(n) {
		n.write(alreadyLockEvent)
		return
	}

	l.push(n)
}

func (l *Lock) UnLock(n *node) {
	if len(l.queue) == 0 {
		return
	}
	if l.nodeKey != byConnGenerKey(n.conn) {
		if n.discard {
			l.discardExpireNode(n)
		}
		return
	}
	l.mx.Lock()
	defer l.mx.Unlock()

	l.updateNodeKey()
	last := l.queue[0]
	result := make([]*node, len(l.queue)-1)
	copy(result, l.queue[1:])
	l.queue = result
	l.removeNodeKey(last)
	last.write(UnLockOKEvent)
	if !n.discard {
		last.changeStatusToNormal()
	}

	if len(l.queue) == 0 {
		store.deleteLock(l)
		return
	}

	next := l.queue[0]
	l.nodeKey = byConnGenerKey(next.conn)
	next.nextTime = time.Now().Add(time.Second * 20)
	next.isNext = true
	next.isLock = true
	next.write(LockOKEvent)
}

func (l *Lock) push(n *node) {
	l.mx.Lock()
	defer l.mx.Unlock()
	l.nodes[byConnGenerKey(n.conn)] = 0x10
	l.queue = append(l.queue, n)
}

func (l *Lock) discardExpireNode(n *node) {
	l.mx.Lock()
	defer l.mx.Unlock()
	newSlice := make([]*node, len(l.queue)-1)
	isFind := false
	index := 0
	for i, kv := range l.queue {
		if kv == n {
			isFind = true
		} else {
			newSlice[index] = l.queue[i]
			index++
		}
	}
	if isFind {
		l.queue = newSlice
		delete(l.nodes, byConnGenerKey(n.conn))
	}
}

func (l *Lock) isNode(n *node) bool {
	_, ok := l.nodes[byConnGenerKey(n.conn)]
	return ok
}

func (l *Lock) updateNodeKey() {
	l.nodeKey = ""
}

func (l *Lock) removeNodeKey(n *node) {
	delete(l.nodes, byConnGenerKey(n.conn))
}

type node struct {
	conn     net.Conn
	lockKey  string
	status   internalStatus
	discard  bool
	ctx      context.Context
	cancel   context.CancelFunc
	nextTime time.Time
	isLock   bool
	isNext   bool
	isQuit   bool
	signal   chan struct{}
}

func newNode(conn net.Conn) *node {
	return &node{
		conn: conn,
	}
}

func (n *node) Handler(body []byte) {
	event, body, err := decode(body)
	if err != nil {
		return
	}
	if event == LockEvent {
		if len(body) == 0 {
			return
		}
		n.lockKey = "LOCK/" + string(body)
		n.lockProcess()
	}
	if event == UnLockEvent {
		n.unLockProcess()
	}
}

func (n *node) lockProcess() {
	lock, ok := store.getLock(n.lockKey)
	if ok && lock.isNode(n) {
		n.write(alreadyLockEvent)
		return
	}

	n.ctx, n.cancel = context.WithTimeout(context.TODO(), time.Second*20)
	defer n.cancel()
	n.signal = make(chan struct{}, 1)
	if ok {
		lock.Lock(n)
	} else {
		store.NewLock(n)
	}
	for {
		select {
		case <-n.ctx.Done():
			if n.isNext {
				n.cancel()
				n.ctx, n.cancel = context.WithDeadline(context.TODO(), n.nextTime)
				n.isNext = false
				break
			}
			//The lock has been obtained but not unlocked in time
			if n.isLock {
				n.write(OperateTimeoutEvent)
			} else {
				//Didn't even get the lock
				n.write(LockFailEvent)
			}
			n.discard = true
			n.unLockProcess()
			n.quit()
			return
		case <-n.signal:
			n.quit()
			return
		}
	}
}

func (n *node) unLockProcess() {
	lock, ok := store.getLock(n.lockKey)
	if !ok || !lock.isNode(n) {
		n.write(notFindLockEvent)
		return
	}
	lock.UnLock(n)
}

func (n *node) quit() {
	if n.isQuit {
		return
	}
	n.isQuit = true
	err := n.conn.Close()
	if err != nil {
		logger.WithFields(logrus.Fields{"type": "runtime", "theme": "close conn", "err": err}).Error()
	}
}

func (n *node) write(event eventType) {
	if n.status == offlineStatus {
		return
	}
	body, err := encode(event)
	if err != nil {
		return
	}
	length, err := n.conn.Write(body)
	if err != nil {
		logger.WithFields(logrus.Fields{"type": "runtime", "theme": "node write", "node": byConnGenerKey(n.conn), "len": length, "err": err}).Error()
	}
}

func (n *node) changeStatusToNormal() {
	n.signal <- struct{}{}
}

func byConnGenerKey(conn net.Conn) string {
	return conn.RemoteAddr().String()
}
