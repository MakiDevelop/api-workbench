build:
	go build -o bin/apiw ./cmd/apiw

install:
	go install ./cmd/apiw

test:
	go test ./...
