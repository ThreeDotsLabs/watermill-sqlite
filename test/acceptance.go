package test

import (
	"context"
	"testing"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"github.com/ThreeDotsLabs/watermill/pubsub/tests"
)

func OfficialImplementationAcceptance(fixture PubSubFixture) func(t *testing.T) {
	return func(t *testing.T) {
		features := tests.Features{
			// ConsumerGroups should be true, if consumer groups are supported.
			ConsumerGroups: true,

			// ExactlyOnceDelivery should be true, if exactly-once delivery is supported.
			ExactlyOnceDelivery: false,

			// GuaranteedOrder should be true, if order of messages is guaranteed.
			GuaranteedOrder: true,

			// Some Pub/Subs guarantee the order only when one subscriber is subscribed at a time.
			GuaranteedOrderWithSingleSubscriber: true,

			// GuaranteedTotalOrder should be true if all messages published to a topic are received
			// in the order they were published. Every topic is a single table with an AUTOINCREMENT offset,
			// SQLite serializes writers so offsets are committed in order, and a consumer group reads
			// by ascending offset while holding the group's row lock.
			GuaranteedTotalOrder: true,

			// Persistent should be true, if messages are persistent between multiple instances of a Pub/Sub
			// (in practice, only GoChannel doesn't support that).
			Persistent: true,

			// RequireSingleInstance must be true,if a PubSub requires a single instance to work properly
			// (for example: GoChannel implementation).
			RequireSingleInstance: true,

			// NewSubscriberReceivesOldMessages should be set to true if messages are persisted even
			// if they are already consumed (for example, like in Kafka).
			NewSubscriberReceivesOldMessages: false,

			// GenerateTopicFunc overrides standard topic name generation.
			// GenerateTopicFunc func(tctx TestContext) string
		}

		// tCtx := tests.TestContext{
		// 	TestID:   tests.NewTestID(),
		// 	Features: features,
		// }
		// tests.TestPublishSubscribe(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestConcurrentSubscribe(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestConcurrentSubscribeMultipleTopics(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestResendOnError(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestNoAck(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestContinueAfterSubscribeClose(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestConcurrentClose(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestContinueAfterErrors(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestPublishSubscribeInOrder(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestPublisherClose(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestTopic(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestMessageCtx(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestSubscribeCtx(t, tCtx, fixture.WithConsumerGroup("test"))
		// tests.TestConsumerGroups(t, tCtx, tests.ConsumerGroupPubSubConstructor(fixture)) // requires features.ConsumerGroups=true

		// OMIT THOSE ACCEPTANCE TESTS
		//
		// tests.TestNewSubscriberReceivesOldMessages(t, tCtx, fixture.WithConsumerGroup("test"))
		// exactly once delivery

		tests.TestPubSub(t,
			features,
			fixture.WithConsumerGroup("test"),
			tests.ConsumerGroupPubSubConstructor(fixture),
		)

		t.Run("ensure nil messages can be processed", TestNilPayloadMessagePublishingAndReceiving(fixture))
	}
}

// TestNilPayloadMessagePublishingAndReceiving ensures that a publisher may publish
// messages without any payload, that is a nil payload.
//
// Clarification: https://github.com/ThreeDotsLabs/watermill/issues/565#issuecomment-2885938295
func TestNilPayloadMessagePublishingAndReceiving(fixture PubSubFixture) func(t *testing.T) {
	return func(t *testing.T) {
		topic := "testNilMessageTopic"
		pub, sub := fixture(t, "testNilMessage")

		// TODO: replace with t.Context() after Watermill bumps to Golang 1.24
		in, err := sub.Subscribe(context.TODO(), topic)
		if err != nil {
			t.Fatal("unable to subscribe to topic:", err)
		}

		nilPayload := message.NewMessage("nilMessage", nil)
		if err = pub.Publish(topic, nilPayload); err != nil {
			t.Fatal("unable to publish message with nil payload:", err)
		}

		select {
		case msg := <-in:
			msg.Ack()
			if msg.UUID != nilPayload.UUID {
				t.Fatal("UUIDs do not match")
			}
			if msg.Payload == nil {
				t.Fatal("nil payload, but should be an empty byte list instead")
			}
		case <-time.After(time.Second):
			t.Fatal("no message was delivered within one second")
		}
	}
}
