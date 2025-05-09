package tests

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/google/uuid"
)

func TestBasicSendRecieve(setup PubSubFixture) func(t *testing.T) {
	topic := "test-basic-publish-subscribe"
	return func(t *testing.T) {
		pub, sub := setup(t, "testConsumerGroup")
		t.Run("publish 20 messages", func(t *testing.T) {
			t.Parallel()

			for i := 0; i < 10; i++ {
				msg := message.NewMessage(uuid.New().String(), []byte("test"))
				msg2 := message.NewMessage(uuid.New().String(), []byte("test"))
				if err := pub.Publish(topic, msg, msg2); err != nil {
					t.Fatal(err)
				}
			}
		})

		t.Run("collect 20 messages", func(t *testing.T) {
			t.Parallel()
			// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
			ctx, cancel := context.WithTimeout(context.TODO(), time.Second*5)
			defer cancel()
			// ctx, cancel := context.WithTimeout(t.Context(), time.Second*5)
			// defer cancel()

			msgs, err := sub.Subscribe(ctx, topic)
			if err != nil {
				t.Fatal(err)
			}

			select {
			case <-time.After(time.Second):
				t.Fatal("first message took longer than a second")
			case msg := <-msgs:
				if msg == nil {
					t.Fatal("message is nil")
				}
				msg.Ack()
				if string(msg.Payload) != "test" {
					t.Fatalf("unexpected message payload: %s", string(msg.Payload))
				}
			}

			end := time.After(time.Second * 4)
			processed := 0

		loop:
			for {
				select {
				case <-end:
					break loop
				case msg := <-msgs:
					if msg == nil {
						t.Fatal("message is nil")
					}
					msg.Ack()
					if string(msg.Payload) != "test" {
						t.Fatalf("unexpected message payload: %s", string(msg.Payload))
					}
					processed++
				}
			}
			if processed != 19 {
				t.Fatalf("expected 19 messages, got %d", processed)
			}
		})
	}
}

func TestOnePublisherThreeSubscribers(setup PubSubFixture, messageCount int) func(t *testing.T) {
	topic := "one-publisher-three-subscribers"
	return func(t *testing.T) {
		t.Parallel()

		pub, sub := setup(t, "testConsumerGroup")
		t.Run("publish messages", func(t *testing.T) {
			t.Parallel()
			for i := 0; i < messageCount; i++ {
				msg := message.NewMessage(uuid.New().String(), []byte("test"))
				if err := pub.Publish(topic, msg); err != nil {
					t.Fatal(err)
				}
			}
		})

		t.Run("collect messages", func(t *testing.T) {
			t.Parallel()
			// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
			ctx, cancel := context.WithCancel(context.TODO())
			defer cancel()

			first, err := sub.Subscribe(ctx, topic)
			if err != nil {
				t.Fatal(err)
			}
			second, err := sub.Subscribe(ctx, topic)
			if err != nil {
				t.Fatal(err)
			}
			third, err := sub.Subscribe(ctx, topic)
			if err != nil {
				t.Fatal(err)
			}

			discoveredMessageUUIDs := map[string]struct{}{}
			discoveredMessageDuplicates := 0
			collectMessage := func(m *message.Message) {
				m.Ack()
				if _, ok := discoveredMessageUUIDs[m.UUID]; ok {
					discoveredMessageDuplicates++
				} else {
					discoveredMessageUUIDs[m.UUID] = struct{}{}
				}
			}

			done := time.After(time.Second * 2)
			for {
				select {
				case <-done:
					if len(discoveredMessageUUIDs) != messageCount {
						t.Fatalf("expected %d messages, got %d", messageCount, len(discoveredMessageUUIDs))
					}
					if discoveredMessageDuplicates > 1001 {
						t.Fatalf("collected more than one batch of duplicates, got %d", discoveredMessageDuplicates)
					} else if discoveredMessageDuplicates > 0 {
						// occasionally an extra batch gets re-delivered when multiple subscribers are processing messages
						// behavior is expected, but curious // TODO: investigate
						t.Logf("collected %d duplicates", discoveredMessageDuplicates)
					}
					return
				case msg := <-first:
					collectMessage(msg)
				case msg := <-second:
					collectMessage(msg)
				case msg := <-third:
					collectMessage(msg)
				}
			}
		})
	}
}
