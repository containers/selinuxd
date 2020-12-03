BIN=$(BINDIR)/selinuxdctl
BINDIR=bin
POLICYDIR=/etc/selinux.d

SRC=$(shell find . -name "*.go")

.PHONY: all
all: build

.PHONY: build
build: $(BIN)

$(BIN): $(BINDIR) $(SRC) pkg/semanage/callbacks.c
	go build -o $(BIN) .

.PHONY: run
run: $(BIN) $(POLICYDIR)
	sudo $(BIN) daemon

$(BINDIR):
	mkdir -p $(BINDIR)

$(POLICYDIR):
	mkdir -p $(POLICYDIR)
