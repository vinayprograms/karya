.PHONY: all clean

all: build

# pattern rule to build all commands in cmd/
bin/%: cmd/%/*.go
	go build -o $@ $<

# use pattern rule to build all commands in cmd/
build:
	mkdir -p bin
	for cmd in cmd/*; do cmd_name=$$(basename $$cmd); $(MAKE) bin/$$cmd_name; done

clean:
	rm -rf bin
