# Watermill SQLite3 Driver Pack

Golang SQLite3 driver pack for <https://watermill.io> event dispatch framework. Drivers satisfy the following interfaces:

- [message.Publisher](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill@v1.4.6/message#Publisher)
- [message.Subscriber](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill@v1.4.6/message#Subscriber)
- [middleware.ExpiringKeyRepository](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill@v1.4.6/message/router/middleware#ExpiringKeyRepository) for message de-duplicator

SQLite3 does not support querying `FOR UPDATE`, which is used for row locking when subscribers in the same consumer group read an event batch in official Watermill SQL PubSub implementations. Current architectural decision is to lock a consumer group offset using `unixepoch()+lockTimeout` time stamp. While one consumed message is processing per group, the offset lock time is extended by `lockTimeout` periodically by `time.Ticker`. If the subscriber is unable to finish the consumer group batch, other subscribers will take over the lock as soon as the grace period runs out. A time lock fulfills the role of a traditional database network timeout that terminates transactions when its client disconnects.

- [ ] Implement SQLite connection back off manager

    A friend recommended implementing a back off manager. I think the SQLite `busy_timeout` produces a linear back off timeout. When attemping to write a row lock, SQLite will freeze the transaction until the previous one is complete up to the `busy_timeout` duration. This should prevent unneccessary waits due to polling. Perhaps this does not work like I imagine. Also, the ZombieZen variant uses immediate transactions, which may ignore the `busy_timeout`. This requires additional investigation before implementing.

    Here is an example attempt: https://github.com/sandpapersoftware/watermillsqlite

    The busy waiting loop, that polls the next batches causes a modification to the database (sets the lock)
    this causes tools like litestream to write wal files to their replicas every poll interval
    this makes a restore incredibly slow and causes a constant drain on the cpu.

    I wonder if a rollback in a batch == 0 case, wouldn't be enough to release the lock or you only set the lock, when a batch is > 0? Lock + read operation are in the same transaction; I think I can just cancel the transaction if the batch size is 0 to prevent the write.

    Implementation examples in other libraries to consider:

    - https://github.com/ThreeDotsLabs/watermill-sql/blob/master/pkg/sql/backoff_manager.go
    - https://github.com/ov2b/watermill-sqlite3/blob/main/reset_latch_backoff_manager.go
    - Comments: https://github.com/dkotik/watermillsqlite/issues/6
- [ ] Add clean up routines for removing old messages from topics.
    - [ ] wmsqlitemodernc.CleanUpTopics
    - [ ] wmsqlitezombiezen.CleanUpTopics
- [ ] ExpiringKeyRepository needs clean up sync test
- [ ] Replace SQL queries with an abstraction adaptor

      Currently, SQL queries are hard-coded into into Publisher and Subscriber. Other implementations provide query adaptors that permit overriding the queries. The author is hesitant to make this change, because it is hard to imagine a use case where this kind of adjustment would be useful. Supporting it seems like over-engineering.

      It is possible to override the table structure by manually creating the table and never setting the InitializeSchema constructor option. For rare specialty use cases, it seems cleaner to create a fork and re-run all the tests to make sure that all the SQL changes are viable and add additional tests. It seems that a query adaptor would just get in the way.

      The issue is created so that arguments can be made in favor of adding a query adaptor.
- [ ] Subscriber with poll interval lower than 10ms locks up; see BenchmarkAll; increasing the batch size also can cause a lock up
- [ ] Three-Dots Labs acceptance requests:
    - [x] may be worth adding test like (but please double check if it makes sense here - it was problematic use case for Postgres): https://github.com/ThreeDotsLabs/watermill-sql/blob/master/pkg/sql/pubsub_test.go#L466 ([won't fix, see discussion](https://github.com/dkotik/watermillsqlite/issues/10#issuecomment-2813855209))
    - [x] publish - you can get context from message (will better work with tracing etc.) - it's tricky when someone is publishing multiple messages - usually we just get context from the first ([won't fix, see discussion](https://github.com/dkotik/watermillsqlite/issues/11)
    - [x] NIT: it would be nice to add abstraction over queries (like in SQL) - so someone could customize it, but not very important ([saved to later](https://github.com/dkotik/watermillsqlite/issues/13))
    - [x] NIT: return io.ErrClosedPipe - maybe better to define custom error for that? ClosedPipe probably a bit different kind of error ([fixed](https://github.com/dkotik/watermillsqlite/commit/e09a9365230f04b14b0d63c76bc8a9c8e94436b7))
    - [x] would be nice to add benchmark - may be good thing for sqlite -> https://github.com/ThreeDotsLabs/watermill-benchmark feel free to make draft PR, we can replace repo later ([opened pull request](https://github.com/ThreeDotsLabs/watermill-benchmark/pull/10))
    - [x] does it  make sense to have two implementations -> if so, guide which to choose for people ([fixed](https://github.com/dkotik/watermillsqlite/commit/74d00ca378a4130b53676dc64a8dfeb277cabc34) and marked the first as vanilla and second as advanced)
    - [x] NewPublisher(db SQLiteDatabase -> it may be nice if it can accept just transaction like in https://github.com/ThreeDotsLabs/watermill-sql/blob/master/pkg/sql/publisher.go#L54 - it allows to add events transactionally ([fixed](https://github.com/dkotik/watermillsqlite/issues/10))
    - [x] options.LockTimeout < time.Second - rationale for second? ([explanation added](https://github.com/dkotik/watermillsqlite/commit/240ec78d2c0e85af3ba84054dbb12621a6aeeae3))
    - [x] consumer groups - it would be nice to make dynamic based on topic - usually we have closure in config that receives topic,
    - [x] ackChannel:   s.NackChannel, - typo? ([fixed](https://github.com/dkotik/watermillsqlite/commit/ae70e4c4989d07ae0d58426d623d48af342a2d10) - yes)
    - [ ] adding some logging may be useful for future - most trace or debug (everything what happens per message) - info for rare events; Robert requested that everything that happens to a message must be traced. Errors are logged. Message publishing is traced. I plan to add more trace messages in the future after benchmarks settle to avoid accidental slight performance regression. Other traces were not yet added, matching the reference implementation here: https://github.com/search?q=repo%3AThreeDotsLabs%2Fwatermill-sql+%22.logger.%22&type=code
- [x] Finish time-based lock extension when:
    - [x] sending a message to output channel
    - [x] waiting for message acknowledgement
- [x] Pass official implementation acceptance tests:
    - [x] ModernC
        - [x] tests.TestPublishSubscribe
        - [x] tests.TestConcurrentSubscribe
        - [x] tests.TestConcurrentSubscribeMultipleTopics
        - [x] tests.TestResendOnError
        - [x] tests.TestNoAck
        - [x] tests.TestContinueAfterSubscribeClose
        - [x] tests.TestConcurrentClose
        - [x] tests.TestContinueAfterErrors
        - [x] tests.TestPublishSubscribeInOrder
        - [x] tests.TestPublisherClose
        - [x] tests.TestTopic
        - [x] tests.TestMessageCtx
        - [x] tests.TestSubscribeCtx
        - [x] tests.TestConsumerGroups
    - [x] ZombieZen (passes simple tests)
        - [x] tests.TestPublishSubscribe
        - [x] tests.TestConcurrentSubscribe
        - [x] tests.TestConcurrentSubscribeMultipleTopics
        - [x] tests.TestResendOnError
        - [x] tests.TestNoAck
        - [x] tests.TestContinueAfterSubscribeClose
        - [x] tests.TestConcurrentClose
        - [x] tests.TestContinueAfterErrors
        - [x] tests.TestPublishSubscribeInOrder
        - [x] tests.TestPublisherClose
        - [x] tests.TestTopic
        - [x] tests.TestMessageCtx
        - [x] tests.TestSubscribeCtx
        - [x] tests.TestConsumerGroups

## Vanilla ModernC Driver
[![Go Reference](https://pkg.go.dev/badge/github.com/ThreeDotsLabs/watermill.svg)](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc)](https://goreportcard.com/report/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc)
[![codecov](https://codecov.io/gh/dkotik/watermillsqlite/wmsqlitemodernc/branch/master/graph/badge.svg)](https://codecov.io/gh/dkotik/watermillsqlite/wmsqlitemodernc)

```sh
go get -u github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc
```

The ModernC driver is compatible with the Golang standard library SQL package. It works without CGO. Has fewer dependencies than the ZombieZen variant.

```go
import (
	"database/sql"
	"github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc"
	_ "modernc.org/sqlite"
)

db, err := sql.Open("sqlite", ":memory:")
if err != nil {
	panic(err)
}
// limit the number of concurrent connections to one
// this is a limitation of `modernc.org/sqlite` driver
db.SetMaxOpenConns(1)
defer db.Close()

pub, err := wmsqlitemodernc.NewPublisher(db, wmsqlitemodernc.PublisherOptions{
	InitializeSchema: true, // create tables for used topics
})
if err != nil {
	panic(err)
}
sub, err := wmsqlitemodernc.NewSubscriber(db, wmsqlitemodernc.SubscriberOptions{
	InitializeSchema: true, // create tables for used topics
})
if err != nil {
	panic(err)
}
// ... follow guides on <https://watermill.io>
```

## Advanced ZombieZen Driver
[![Go Reference](https://pkg.go.dev/badge/github.com/ThreeDotsLabs/watermill.svg)](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen)
[![Go Report Card](https://goreportcard.com/badge/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen)](https://goreportcard.com/report/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen)
[![codecov](https://codecov.io/gh/dkotik/watermillsqlite/wmsqlitezombiezen/branch/master/graph/badge.svg)](https://codecov.io/gh/dkotik/watermillsqlite/wmsqlitezombiezen)

```sh
go get -u github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen
```

The ZombieZen driver abandons the standard Golang library SQL conventions in favor of [the more orthogonal API and higher performance potential](https://crawshaw.io/blog/go-and-sqlite). Under the hood, it uses ModernC SQLite3 implementation and does not need CGO. Advanced SQLite users might prefer this driver.

It is about **9 times faster** than the ModernC variant when publishing messages. It is currently more stable due to lower level control. It is faster than even the CGO SQLite variants on standard library interfaces, and with some tuning should become the absolute speed champion of persistent message brokers over time. Tuned SQLite is [~35% faster](https://sqlite.org/fasterthanfs.html) than the Linux file system.

```go
import "github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen"

// &cache=shared is critical, see: https://github.com/zombiezen/go-sqlite/issues/92#issuecomment-2052330643
connectionDSN := ":memory:")
conn, err := sqlite.OpenConn(connectionDSN)
if err != nil {
	panic(err)
}
defer conn.Close()

pub, err := wmsqlitezombiezen.NewPublisher(conn, wmsqlitezombiezen.PublisherOptions{
	InitializeSchema: true, // create tables for used topics
})
if err != nil {
	panic(err)
}
sub, err := wmsqlitezombiezen.NewSubscriber(connectionDSN, wmsqlitezombiezen.SubscriberOptions{
	InitializeSchema: true, // create tables for used topics
})
if err != nil {
	panic(err)
}
// ... follow guides on <https://watermill.io>
```

## Similar Projects

- <https://github.com/davidroman0O/watermill-comfymill>
- <https://github.com/walterwanderley/watermill-sqlite>
<!-- - <https://github.com/ov2b/watermill-sqlite3> - author requested removal of the mention, because it is a very rough draft - requires CGO for `mattn/go-sqlite3` dependency -->
