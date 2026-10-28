# Watermill SQLite Pub/Sub
<img align="right" width="200" src="https://watermill.io/img/gopher.svg">

[![CI Status](https://github.com/ThreeDotsLabs/watermill-sqlite/actions/workflows/master.yml/badge.svg)](https://github.com/ThreeDotsLabs/watermill-sqlite/actions/workflows/master.yml)

This is Pub/Sub for the [Watermill](https://watermill.io/) project. The implementation provides two CGO-free driver variants optimized for different use cases.

**Beta Version Warning: this Pub/Sub is stable, but it has not been widely tested in production environments. It may be sensitive to certain edge cases and combinations of configuration parameters.**

1. ModernC [![Go Reference](https://pkg.go.dev/badge/github.com/ThreeDotsLabs/watermill.svg)](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc) [![Go Report Card](https://goreportcard.com/badge/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc)](https://goreportcard.com/report/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitemodernc)
2. ZombieZen [![Go Reference](https://pkg.go.dev/badge/github.com/ThreeDotsLabs/watermill.svg)](https://pkg.go.dev/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen) [![Go Report Card](https://goreportcard.com/badge/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen)](https://goreportcard.com/report/github.com/ThreeDotsLabs/watermill-sqlite/wmsqlitezombiezen)

See [DEVELOPMENT.md](./DEVELOPMENT.md) for more information about running and testing.

Watermill is a Go library for working efficiently with message streams. It is intended
for building event driven applications, enabling event sourcing, RPC over messages,
sagas and basically whatever else comes to your mind. You can use conventional pub/sub
implementations like Kafka or RabbitMQ, but also HTTP or MySQL binlog if that fits your use case.

All Pub/Sub implementations can be found at [https://watermill.io/pubsubs/](https://watermill.io/pubsubs/).

Documentation: https://watermill.io/

Getting started guide: https://watermill.io/docs/getting-started/

Issues: https://github.com/ThreeDotsLabs/watermill/issues

## Contributing

All contributions are very much welcome. If you'd like to help with Watermill development,
please see [open issues](https://github.com/ThreeDotsLabs/watermill/issues?utf8=%E2%9C%93&q=is%3Aissue+is%3Aopen+)
and submit your pull request via GitHub.

## Support

If you didn't find the answer to your question in [the documentation](https://watermill.io/), feel free to ask us directly!

Please join us on the `#watermill` channel on the [Three Dots Labs Discord](https://discord.gg/QV6VFg4YQE).

## License

[MIT License](./LICENSE)
