package pubsub

import "sync"

type Subscriber[T any] struct {
	ch    chan T
	close chan struct{}
	once  sync.Once
}

// Events 返回消息通道，供订阅者消费。
func (s *Subscriber[T]) Events() <-chan T { return s.ch }

// Close 注销订阅并释放资源。可安全重复调用。
func (s *Subscriber[T]) Close() { s.once.Do(func() { close(s.close) }) }

// Done 在订阅关闭时返回一个结束信号，常用于 select 操作。
func (s *Subscriber[T]) Done() <-chan struct{} { return s.close }

// Bus 是泛型的订阅/发布总线，管理一组订阅者。
type Bus[T any] struct {
	mu   sync.Mutex
	subs map[*Subscriber[T]]struct{}
}

// NewBus 构造一个针对消息类型 T 的总线。
func NewBus[T any]() *Bus[T] {
	return &Bus[T]{subs: make(map[*Subscriber[T]]struct{})}
}

// Subscribe 注册一个新订阅者。调用方随后应从其 Events() 通道读取消息。
func (b *Bus[T]) Subscribe() *Subscriber[T] {
	sub := &Subscriber[T]{
		ch:    make(chan T, 64),
		close: make(chan struct{}),
	}

	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()

	return sub
}

// Publish 向所有订阅者投递一条消息。某个订阅者消费过慢时会丢弃该消息，
// 以保证发布者不被阻塞（适用于流式事件的尽力投递语义）。
func (b *Bus[T]) Publish(ev T) {
	b.mu.Lock()
	defer b.mu.Unlock()

	for sub := range b.subs {
		select {
		case sub.ch <- ev:
		default:
		}
	}
}

// Unsubscribe 移除一个订阅者。
func (b *Bus[T]) Unsubscribe(sub *Subscriber[T]) {
	b.mu.Lock()
	delete(b.subs, sub)
	b.mu.Unlock()
}
