MODULE := github.com/pasta/pasta
BIN := bin

.PHONY: all build-linux build-windows test vet run clean

all: build-linux

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)/pasta .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $(BIN)/pasta.exe .

test:
	go test ./...

vet:
	go vet ./...

run:
	go run . --verbose

clean:
	rm -rf $(BIN)
