package main 

import (
	"sync"
	"time"
)

type messageLatencyTracker struct {
	sync.Mutex
	sentTime map[string]time.Time
	receivedTime map[string][]time.Time
	ch chan string
}

func newMessageLatencyTracker(numNodes int) *messageLatencyTracker {
	return &messageLatencyTracker{
		sentTime: make(map[string]time.Time),
		receivedTime: make(map[string][]time.Time),
		ch: make(chan string, numNodes),
	}
}

func (t *messageLatencyTracker) trackRecieved() {
	for id := range t.ch {
		t.logReceived(id)
	}
}

func (t *messageLatencyTracker) logSent(id string) {
	t.Lock()
	defer t.Unlock()
	t.sentTime[id] = time.Now()
}

func (t *messageLatencyTracker) logReceived(id string) {
	t.Lock()
	defer t.Unlock()
	existing, has := t.receivedTime[id]
	if !has {
		t.receivedTime[id] = []time.Time{time.Now()}
	} else {
		t.receivedTime[id] = append(existing, time.Now())
	}
}

func (t *messageLatencyTracker) log() {
	t.Lock()
	defer t.Unlock()
	for id := range t.receivedTime {
		log.Infof("msg ID %s\n\tsent time %s\n\trcvd times %s", id, t.sentTime[id], t.receivedTime[id])
	}
}