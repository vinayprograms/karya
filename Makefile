.PHONY: all clean build install install-plugins

all: build

# pattern rule to build all commands in cmd/
bin/%: cmd/%/main.go
	go build -o $@ $<

build:
	mkdir -p bin
	for cmd in cmd/*; do cmd_name=$$(basename $$cmd); $(MAKE) bin/$$cmd_name; done

# Use `go install` to install commands
install: clean
	for cmd in cmd/*; do cmd_name=$$(basename $$cmd); go install ./cmd/$$cmd_name; done

install-plugins:
	@if [ ! -d "$$HOME/.vim/pack/plugins/start" ]; then \
		echo "Error: ~/.vim/pack/plugins/start/ does not exist"; exit 1; \
	fi
	rm -rf "$$HOME/.vim/pack/plugins/start/karya.vim"
	ln -s "$(CURDIR)/editors/vim" "$$HOME/.vim/pack/plugins/start/karya.vim"
	@echo "Linked editors/vim → ~/.vim/pack/plugins/start/karya.vim"

clean:
	rm -rf bin
