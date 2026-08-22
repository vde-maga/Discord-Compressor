.PHONY: build test clean

build:
	go build -ldflags="-s -w" -o discord-compress .

test:
	go test -v ./...

clean:
	rm -f discord-compress ffmpeg2pass*.log*
