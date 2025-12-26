.PHONY: all clean build install

all: build

# pattern rule to build all commands in cmd/
bin/%: cmd/%/main.go
	go build -o $@ $<

build:
	mkdir -p bin
	for cmd in cmd/*; do cmd_name=$$(basename $$cmd); GOEXPERIMENT=greenteagc $(MAKE) bin/$$cmd_name; done

# Use `go install` to install commands
install: clean
	for cmd in cmd/*; do cmd_name=$$(basename $$cmd); GOEXPERIMENT=greenteagc go install ./cmd/$$cmd_name; done

clean:
	rm -rf bin
