package pubsub_test

import (
	"testing"
	"time"

	"ci.zevix.uk/moment-go/pkg/pubsub"
)

func TestPublishSubscribe(t *testing.T) {
	bus := pubsub.NewBus[string]()
	sub := bus.Subscribe()
	defer sub.Close()

	bus.Publish("hello")
	select {
	case got := <-sub.Events():
		if got != "hello" {
			t.Fatalf("got %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("no event received")
	}
}

func TestSubscribeClose(t *testing.T) {
	bus := pubsub.NewBus[int]()
	sub := bus.Subscribe()
	sub.Close()
	select {
	case <-sub.Done():
	case <-time.After(time.Second):
		t.Fatal("Done not closed after Close")
	}
	// 重复 Close 应安全。
	sub.Close()
}

func TestSlowConsumerDropped(t *testing.T) {
	bus := pubsub.NewBus[int]()
	sub := bus.Subscribe()
	defer sub.Close()

	// 通道容量 64，超过后慢消费者的事件被丢弃而非阻塞发布者。
	for i := 0; i < 128; i++ {
		bus.Publish(i)
	}
	if len(sub.Events()) > 64 {
		t.Fatalf("slow consumer should be capped, len=%d", len(sub.Events()))
	}
}
