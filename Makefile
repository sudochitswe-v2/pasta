MODULE := github.com/sudochitswe-v2/pasta
BIN := bin
DIST := dist

.PHONY: all build-linux build-windows test vet run clean dist

all: build-linux

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)/pasta .

build-windows:
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -H=windowsgui" -o $(BIN)/pasta.exe .
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o $(BIN)/pasta-cli.exe .

dist: build-linux build-windows
	rm -rf $(DIST)
	mkdir -p $(DIST)
	tar -czf $(DIST)/pasta-linux-amd64.tar.gz -C $(BIN) pasta -C ../deploy pasta.service
	zip -j $(DIST)/pasta-windows-amd64.zip $(BIN)/pasta.exe $(BIN)/pasta-cli.exe deploy/install.ps1
	cd $(DIST) && sha256sum pasta-linux-amd64.tar.gz pasta-windows-amd64.zip > checksums.txt

test:
	go test ./...

vet:
	go vet ./...

run:
	go run . --verbose

clean:
	rm -rf $(BIN) $(DIST)
