package service

import (
	"strings"
	"sync"
)

const liveFeedTopic = "__live_feed__"

// EventBroker is a lightweight fan-out pub/sub for player score-update events.
// Each subscriber gets a buffered channel that receives a signal whenever new
// scores are ingested for a given player handle.
type EventBroker struct {
	mu   sync.RWMutex
	subs map[string][]chan struct{}
}

func NewEventBroker() *EventBroker {
	return &EventBroker{subs: make(map[string][]chan struct{})}
}

// Subscribe returns a channel that receives a signal when scores are updated
// for handle.  Call unsub when done to release the channel.
func (b *EventBroker) Subscribe(handle string) (ch chan struct{}, unsub func()) {
	handle = normalizeTopic(handle)
	ch = make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[handle] = append(b.subs[handle], ch)
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		defer b.mu.Unlock()
		chs := b.subs[handle]
		for i, c := range chs {
			if c == ch {
				b.subs[handle] = append(chs[:i], chs[i+1:]...)
				break
			}
		}
		if len(b.subs[handle]) == 0 {
			delete(b.subs, handle)
		}
	}
}

// Publish signals all subscribers of handle that scores have been updated.
// Non-blocking: slow subscribers are skipped rather than held.
func (b *EventBroker) Publish(handle string) {
	handle = normalizeTopic(handle)
	if handle == "" {
		return
	}
	b.mu.RLock()
	chs := b.subs[handle]
	b.mu.RUnlock()
	for _, ch := range chs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func (b *EventBroker) SubscribeLiveFeed() (chan struct{}, func()) {
	return b.Subscribe(liveFeedTopic)
}

func (b *EventBroker) PublishLiveFeed() {
	b.Publish(liveFeedTopic)
}

func normalizeTopic(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}
