.PHONY: test vet fmt run

test:
	go test ./...

vet:
	go vet ./...

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

run:
	go run ./cmd/bot -config config/config.yaml
