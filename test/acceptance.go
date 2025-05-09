package tests

import (
	"testing"

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
	}
}
