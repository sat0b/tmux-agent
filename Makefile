.PHONY: build test install clean

build:
	go build -o tmux-agent .

test:
	go test -v ./...

install:
	go install .

clean:
	rm -f tmux-agent
