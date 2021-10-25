NAME=ci-downloader
VERSION?=$$(git rev-parse HEAD)

default: release

version:
	@echo ${VERSION}

.PHONY: build
build:
	# The -w turns off DWARF debugging information
	# The -s turns off generation of the Go symbol table
	GOOS=${GOOS} GOARCH=${GOARCH} CGO_ENABLED=0 \
		go build \
			-mod vendor \
			-ldflags="-w -s -X main.Version=${VERSION}" \
			-o bin/${NAME} \
			./cmd/main.go

.PHONY: vendor
vendor:
	go mod vendor

run:
	go run -mod vendor cmd/main.go

release: build
