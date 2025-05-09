package tests

import (
	"context"
	"testing"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

const benchmarkTopic = "benchmark"

func NewPublishingBenchmark(p message.Publisher) func(*testing.B) {
	return func(b *testing.B) {
		if p == nil {
			b.Fatal("publisher is nil")
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			msg := message.NewMessage(uuid.New().String(), []byte("test"))
			err := p.Publish(benchmarkTopic, msg)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}

func NewSubscriptionBenchmark(s message.Subscriber) func(*testing.B) {
	return func(b *testing.B) {
		if s == nil {
			b.Fatal("subscriber is nil")
		}
		// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
		ctx, cancel := context.WithCancel(context.TODO())
		defer cancel()

		sub, err := s.Subscribe(ctx, benchmarkTopic)
		if err != nil {
			b.Fatal(err)
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			select {
			case <-ctx.Done():
				b.Fatal(ctx.Err())
			case msg := <-sub:
				msg.Ack()
			}
		}
	}
}
