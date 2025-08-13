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
		bufferSize = 5000
	}
	return &EventBus{
		subscribers: make(map[string][]EventChan),
		bufferSize:  bufferSize,
	}
}

func (eb *EventBus) Publish(topic string, event Event) {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	subscribers := append([]EventChan{}, eb.subscribers[topic]...)

	var wg sync.WaitGroup
	for _, subscriber := range subscribers {
		wg.Add(1)
		go func(ch EventChan) {
			defer wg.Done()
			// 直接发送，会阻塞直到通道有空间
			ch <- event
		}(subscriber)
	}
	wg.Wait() // 等待所有订阅者都收到事件
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
