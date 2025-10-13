.PHONY: all build clean

all: build

build:
	mkdir -p bin
	for cmd in cmd/*; do \
		name=$$(basename $$cmd); \
		go build -o bin/$$name ./$$cmd; \
	done

clean:
	rm -rf bin