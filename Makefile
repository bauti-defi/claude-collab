.PHONY: build clean test

build:
	cd src && go build -o ../bin/claude-collab .

test:
	cd src && go test -v -race -count=1 ./...

clean:
	rm -f bin/claude-collab
