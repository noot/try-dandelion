package main

import (
	"sync"
	"time"
)

type messageLatencyTracker struct {
	sync.Mutex
	sentTime     map[string]time.Time
	receivedTime map[string][]time.Time
}

func newMessageLatencyTracker(numNodes int) *messageLatencyTracker {
	return &messageLatencyTracker{
		sentTime:     make(map[string]time.Time),
		receivedTime: make(map[string][]time.Time),
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
	var durationSum time.Duration
	count := int64(0)
	durations := []time.Duration{}
	for id := range t.receivedTime {
		log.Infof("msg ID %s\n\tsent time %s\n\trcvd times %s", id, t.sentTime[id], t.receivedTime[id])
		if _, has := t.sentTime[id]; !has {
			continue
		}
		lastIdx := len(t.receivedTime[id]) - 1
		duration := t.receivedTime[id][lastIdx].Sub(t.sentTime[id])
		log.Infof("time between sending and last node receiving: %s", duration)
		durations = append(durations, duration)
		durationSum += duration
		count++
	}
	log.Infof("all durations: %s", durations)
	log.Infof("average total duration: %s", durationSum/time.Duration(count))
}
