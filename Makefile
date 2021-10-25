NAME=ci-downloader

build:
	GOOS=${GOOS} GOARCH=${GOARCH} \
	go build \
		-mod vendor \
		-o bin/${NAME} \
		./cmd/main.go

run:
	go run -mod vendor cmd/main.go