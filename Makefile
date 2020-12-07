BIN=$(BINDIR)/selinuxdctl
BINDIR=bin
POLICYDIR=/etc/selinux.d

SRC=$(shell find . -name "*.go")

.PHONY: all
all: build

.PHONY: build
build: $(BIN)

$(BIN): $(BINDIR) $(SRC) pkg/semodule/semanage/callbacks.c
	go build -o $(BIN) .

.PHONY: test
test:
	go test github.com/JAORMX/selinuxd/...

.PHONY: run
run: $(BIN) $(POLICYDIR)
	sudo $(BIN) daemon

$(BINDIR):
	mkdir -p $(BINDIR)

$(POLICYDIR):
	mkdir -p $(POLICYDIR)
