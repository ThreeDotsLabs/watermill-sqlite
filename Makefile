.PHONY: test test_v test_short test_race test_stress test_reconnect test_codecov up wait build fmt update_watermill

up:
	# nothing to do - for compatibility with other makefiles

test:
	(cd wmsqlitezombiezen && go test -count=5 -failfast -timeout=15m ./...)
	(cd wmsqlitemodernc && go test -count=5 -failfast -timeout=15m ./...)

test_v:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=15m ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=15m ./...)

test_short:
	(cd wmsqlitemodernc && go test -short -count=5 -failfast -timeout=15m ./...)
	(cd wmsqlitezombiezen && go test -short -count=5 -failfast -timeout=15m ./...)

test_race:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=18m -race ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=18m -race ./...)

test_stress:
	(cd wmsqlitemodernc && go test -v -count=15 -failfast -timeout=30m ./...)
	(cd wmsqlitezombiezen && go test -v -count=15 -failfast -timeout=30m ./...)

test_reconnect:
	# nothing to do - for compatibility with other makefiles

test_codecov: up wait
	(cd wmsqlitemodernc && go test -coverprofile=coverage.out -covermode=atomic ./...)
	(cd wmsqlitezombiezen && go test -coverprofile=coverage.out -covermode=atomic ./...)


benchmark:
	(cd test && go wmsqlitemodernc -bench=. -run=^BenchmarkAll$$ -timeout=15s)
	(cd test && go wmsqlitezombiezen -bench=. -run=^BenchmarkAll$$ -timeout=15s)

wait:
	# nothing to do - for compatibility with other makefiles

build:
	go build ./...

fmt:
	go fmt ./...
	goimports -l -w .

update_watermill:
	go get -u github.com/ThreeDotsLabs/watermill
	go mod tidy

	sed -i '\|go 1\.|d' go.mod
	go mod edit -fmt

default: test