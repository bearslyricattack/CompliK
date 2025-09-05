package eventbus

import (
	"sync"
)

type Event struct {
	Payload interface{}
}

type EventChan chan Event

type EventBus struct {
	mu          sync.RWMutex
	subscribers map[string][]EventChan
	bufferSize  int
}

func NewEventBus(bufferSize int) *EventBus {
	if bufferSize <= 0 {
		bufferSize = 10000
	}
	return &EventBus{
		subscribers: make(map[string][]EventChan),
		bufferSize:  bufferSize,
	}
}

func (eb *EventBus) Publish(topic string, event Event) {
	eb.mu.RLock()
	subscribers := eb.subscribers[topic]
	eb.mu.RUnlock()

	// 非阻塞发送，避免死锁
	for _, subscriber := range subscribers {
		go func(sub chan Event) {
			sub <- event
		}(subscriber)
	}
}

func (eb *EventBus) Subscribe(topic string) EventChan {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	ch := make(EventChan, eb.bufferSize)
	eb.subscribers[topic] = append(eb.subscribers[topic], ch)
	return ch
}

func (eb *EventBus) Unsubscribe(topic string, ch EventChan) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if subscribers, ok := eb.subscribers[topic]; ok {
		for i, subscriber := range subscribers {
			if ch == subscriber {
				eb.subscribers[topic] = append(subscribers[:i], subscribers[i+1:]...)
				close(ch)
				for range ch {
				}
				return
			}
		}
	}
}
