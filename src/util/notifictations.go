package util

import (
	"errors"
	"sync"
)

type Notifications struct {
	mtx  sync.RWMutex
	nots map[string]chan error
}

func NewNotifications() *Notifications {
	return &Notifications{mtx: sync.RWMutex{}, nots: make(map[string]chan error)}
}

var ErrNotificationAlreadyExists = errors.New("notification with this ID already exists")

func (n *Notifications) AddPromiseChan(reqID string) (chan error, error) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	c := make(chan error, 1)
	if _, ok := n.nots[reqID]; ok {
		return nil, ErrNotificationAlreadyExists
	}
	n.nots[reqID] = c
	return c, nil
}

func (n *Notifications) Notify(reqID string, err error) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	if c, ok := n.nots[reqID]; ok {
		c <- err
	}
}

func (n *Notifications) Delete(reqID string) {
	n.mtx.Lock()
	defer n.mtx.Unlock()
	delete(n.nots, reqID)
}
