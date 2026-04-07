PREFIX  ?= $(HOME)/bin
VERSION ?= 0.1.0

.PHONY: build install completions clean fmt vet

build:
	go build -ldflags "-X gloss/cmd.Version=$(VERSION)" -o gloss .

install: build
	mkdir -p $(PREFIX)
	cp gloss $(PREFIX)/gloss
	codesign --force --sign - $(PREFIX)/gloss
	@echo "Installed gloss to $(PREFIX)/gloss"

completions: install
	mkdir -p $(HOME)/.config/fish/completions
	$(PREFIX)/gloss completion fish > $(HOME)/.config/fish/completions/gloss.fish

fmt:
	gofmt -w .

vet:
	go vet ./...

clean:
	rm -f gloss
