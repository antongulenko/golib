package golib

import (
	"sync"
	"sync/atomic"
	"time"
	"unsafe"
)

type BoolCondition struct {
	*sync.Cond
	Val bool
}

func NewBoolCondition() *BoolCondition {
	return &BoolCondition{
		Cond: sync.NewCond(new(sync.Mutex)),
	}
}

func (cond *BoolCondition) Broadcast() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = true
	cond.Cond.Broadcast()
}

func (cond *BoolCondition) Signal() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = true
	cond.Cond.Signal()
}

func (cond *BoolCondition) Unset() {
	cond.L.Lock()
	defer cond.L.Unlock()
	cond.Val = false
}

func (cond *BoolCondition) Wait() {
	cond.L.Lock()
	defer cond.L.Unlock()
	for !cond.Val {
		cond.Cond.Wait()
		if cond.Val {
			return
		}
	}
}

func (cond *BoolCondition) WaitAndUnset() {
	cond.L.Lock()
	defer cond.L.Unlock()
	for {
		if cond.Val {
			cond.Val = false
			return
		}
		cond.Cond.Wait()
		if cond.Val {
			cond.Val = false
			return
		}
	}
}

// Inspired from: https://gist.github.com/zviadm/c234426882bfc8acba88f3503edaaa36#file-cond2-go

// Like sync.Cond, but supports WaitTimeout() instead of Signal()
type TimeoutCond struct {
	L sync.Locker
	n unsafe.Pointer
}

func NewTimeoutCond(l sync.Locker) *TimeoutCond {
	c := &TimeoutCond{L: l}
	n := make(chan struct{})
	c.n = unsafe.Pointer(&n)
	return c
}

func (c *TimeoutCond) Wait() {
	n := c.notifyChan()
	c.L.Unlock()
	<-n
	c.L.Lock()
}

func (c *TimeoutCond) WaitTimeout(t time.Duration) {
	n := c.notifyChan()
	c.L.Unlock()
	select {
	case <-n:
	case <-time.After(t):
	}
	c.L.Lock()
}

func (c *TimeoutCond) notifyChan() <-chan struct{} {
	ptr := atomic.LoadPointer(&c.n)
	return *((*chan struct{})(ptr))
}

func (c *TimeoutCond) Broadcast() {
	n := make(chan struct{})
	ptrOld := atomic.SwapPointer(&c.n, unsafe.Pointer(&n))
	close(*(*chan struct{})(ptrOld))
}
