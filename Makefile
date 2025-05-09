up:
	docker compose up -d

test:
	go test ./...

test_v:
	go test -v ./...

test_short:
	go test ./... -short

test_race:
	go test ./... -short -race

test_stress:
	go test -tags=stress -timeout=45m ./...

test_reconnect:
	go test -tags=reconnect ./...

test_codecov: up wait
	go test -coverprofile=coverage.out -covermode=atomic ./...

wait:
	go run github.com/ThreeDotsLabs/wait-for@latest localhost:4566

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

default:
	(cd wmsqlitemodernc && go test -short -failfast ./...)
	(cd wmsqlitezombiezen && go test -short -failfast ./...)
test:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=15m ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=15m ./...)
test_race:
	(cd wmsqlitemodernc && go test -v -count=5 -failfast -timeout=18m -race ./...)
	(cd wmsqlitezombiezen && go test -v -count=5 -failfast -timeout=18m -race ./...)
benchmark:
	(cd wmsqlitemodernc && go test -bench=. -run=^BenchmarkAll$$ -timeout=15s)
	(cd wmsqlitezombiezen && go test -bench=. -run=^BenchmarkAll$$ -timeout=15s)
