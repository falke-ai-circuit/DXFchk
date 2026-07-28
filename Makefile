.PHONY: build vet test cross run web clean

GOCMD=/opt/data/go/bin/go
GOBUILD=$(GOCMD) build
GOVET=$(GOCMD) vet
VERSION=v0.1.0
LDFLAGS=-X main.version=$(VERSION)

build:
	$(GOBUILD) -ldflags "$(LDFLAGS)" -o ./cmd/dxfchk/dxfchk ./cmd/dxfchk/

vet:
	$(GOVET) ./...

test:
	$(GOCMD) test ./... -v -count=1

cross:
	GOOS=linux GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ./build/dxfchk-linux-amd64 ./cmd/dxfchk/
	GOOS=linux GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ./build/dxfchk-linux-arm64 ./cmd/dxfchk/
	GOOS=windows GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ./build/dxfchk-windows-amd64.exe ./cmd/dxfchk/
	GOOS=darwin GOARCH=amd64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ./build/dxfchk-darwin-amd64 ./cmd/dxfchk/
	GOOS=darwin GOARCH=arm64 $(GOBUILD) -ldflags "$(LDFLAGS)" -o ./build/dxfchk-darwin-arm64 ./cmd/dxfchk/

run: build
	./cmd/dxfchk/dxfchk --port 8643 --db-path dxfchk-data

web:
	cd web && npm install && npm run build

clean:
	rm -rf ./build/
	rm -f ./cmd/dxfchk/dxfchk