.PHONY: test test_v test_short test_race test_stress test_reconnect test_codecov up wait build fmt update_watermill

up:
	# nothing to do - for compatibility with other makefiles

test:
	(cd wmsqlitezombiezen && go test -count=8 -failfast -timeout=30m ./...)
	(cd wmsqlitemodernc && go test -count=8 -failfast -timeout=30m ./...)

test_v:
	(cd wmsqlitemodernc && go test -v -count=2 -failfast -timeout=30m ./...)
	(cd wmsqlitezombiezen && go test -v -count=2 -failfast -timeout=30m ./...)

test_short:
	(cd wmsqlitemodernc && go test -short -count=5 -failfast -timeout=30m ./...)
	(cd wmsqlitezombiezen && go test -short -count=5 -failfast -timeout=30m ./...)

test_race:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=50m -race ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=50m -race ./...)

test_stress:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=50m ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=50m ./...)

test_reconnect:
	# nothing to do - for compatibility with other makefiles

test_codecov: up wait
	(cd wmsqlitemodernc && go test -coverprofile=coverage.out -covermode=atomic ./...)
	(cd wmsqlitezombiezen && go test -coverprofile=coverage.out -covermode=atomic ./...)


benchmark:
	(cd wmsqlitemodernc && go test -bench=. -run=^BenchmarkAll$$ -timeout=15s)
	(cd wmsqlitezombiezen && go test -bench=. -run=^BenchmarkAll$$ -timeout=15s)

wait:
	# nothing to do - for compatibility with other makefiles

build:
  # nothing to do - for compatibility with other makefiles

fmt:
	(cd wmsqlitemodernc && go fmt ./... && goimports -l -w .)
	(cd wmsqlitezombiezen && go fmt ./... && goimports -l -w .)
	(cd test && go fmt ./... && goimports -l -w .)

update_watermill:
	(cd test && go get -u github.com/ThreeDotsLabs/watermill@latest && go mod tidy)
	(cd wmsqlitemodernc && go get -u github.com/ThreeDotsLabs/watermill-sqlite/test@latest && go get -u github.com/ThreeDotsLabs/watermill@latest && go mod tidy)
	(cd wmsqlitezombiezen && go get -u github.com/ThreeDotsLabs/watermill-sqlite/test@latest && go get -u github.com/ThreeDotsLabs/watermill@latest && go mod tidy)

	sed -i '|^go 1\.|d' test/go.mod wmsqlitemodernc/go.mod wmsqlitezombiezen/go.mod
	(cd test && go mod edit -fmt)
	(cd wmsqlitemodernc && go mod edit -fmt)
	(cd wmsqlitezombiezen && go mod edit -fmt)

reflex_modernc:
		(cd wmsqlitemodernc && reflex --inverse-regex=testdata -- sh -c 'clear; echo "[00] ---\n";go test ./... | grep -v -E "^(\?|ok)\s+"; echo "---"')

reflex_zombiezen:
		(cd wmsqlitezombiezen && reflex --inverse-regex=testdata -- sh -c 'clear; echo "[00] ---\n";go test ./... | grep -v -E "^(\?|ok)\s+"; echo "---"')

default: test
