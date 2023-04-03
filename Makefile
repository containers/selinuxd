PROJECT=selinuxd
BIN=$(BINDIR)/selinuxdctl
BINDIR=bin
POLICYDIR=/etc/selinux.d

SRC=$(shell find . -name "*.go")

GO?=go

GO_PROJECT := github.com/containers/$(PROJECT)

# External Helper variables

GOLANGCI_LINT_VERSION=1.50.1
GOLANGCI_LINT_OS=linux
ifeq ($(OS_NAME), Darwin)
    GOLANGCI_LINT_OS=darwin
endif
GOLANGCI_LINT_URL=https://github.com/golangci/golangci-lint/releases/download/v$(GOLANGCI_LINT_VERSION)/golangci-lint-$(GOLANGCI_LINT_VERSION)-$(GOLANGCI_LINT_OS)-amd64.tar.gz

CONTAINER_RUNTIME?=podman

IMAGE_NAME=$(PROJECT)
IMAGE_TAG=latest

IMAGE_REF=$(IMAGE_NAME):$(IMAGE_TAG)

IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_REF)
EL8_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-el8:$(IMAGE_TAG)
EL9_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-el9:$(IMAGE_TAG)
# Tag centos the same as EL8 for now, remove after some SPO releases pass
CENTOS_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-centos:$(IMAGE_TAG)
FEDORA_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-fedora:$(IMAGE_TAG)

TEST_OS?=fedora

DATE_FMT = +'%Y-%m-%dT%H:%M:%SZ'
BUILD_DATE ?= $(shell date -u "$(DATE_FMT)")

# By default the version is the latest tag in the current branch,
# if there is none, then the commit hash
VERSION := $(shell git describe --tag)
ifeq ($(VERSION),)
	VERSION := $(shell git rev-parse --short --verify HEAD)
endif

LDVARS := \
	-X $(GO_PROJECT)/pkg/version.buildDate=$(BUILD_DATE) \
	-X $(GO_PROJECT)/pkg/version.version=$(VERSION)

SEMODULE_BACKEND?=policycoreutils
ifeq ($(SEMODULE_BACKEND), semanage)
	BUILDTAGS:=semanage
endif
ifeq ($(SEMODULE_BACKEND), policycoreutils)
	BUILDTAGS:=policycoreutils
endif

# Targets

.PHONY: all
all: build

.PHONY: build
build: $(BIN)

$(BIN): $(BINDIR) $(SRC) pkg/semodule/semanage/callbacks.c
	$(GO) build -ldflags "$(LDVARS)" -tags '$(BUILDTAGS)' -o $(BIN) .

.PHONY: test
test:
	$(GO) test -tags '$(BUILDTAGS)' -race $(GO_PROJECT)/pkg/...

.PHONY: e2e
e2e:
	$(GO) test -tags '$(BUILDTAGS)' ./tests/e2e -timeout 40m -v --ginkgo.v


.PHONY: run
run: $(BIN) $(POLICYDIR)
	sudo $(BIN) daemon

.PHONY: runc
runc: image $(POLICYDIR)
	sudo $(CONTAINER_RUNTIME) run -ti \
		--privileged \
		-v /sys/fs/selinux:/sys/fs/selinux \
		-v /var/lib/selinux:/var/lib/selinux \
		-v /etc/selinux.d:/etc/selinux.d \
		$(IMAGE_REPO) daemon

$(BINDIR):
	mkdir -p $(BINDIR)

$(POLICYDIR):
	mkdir -p $(POLICYDIR)

.PHONY: verify
verify: mod-verify verify-go-lint ## Run code lint checks

.PHONY: mod-verify
mod-verify:
	@$(GO) mod verify

.PHONY: verify-go-lint
verify-go-lint: golangci-lint ## Verify the golang code by linting
	GOLANGCI_LINT_CACHE=/tmp/golangci-cache $(GOPATH)/bin/golangci-lint run

# Install external dependencies
.PHONY: golangci-lint
golangci-lint: $(GOPATH)/bin/golangci-lint

$(GOPATH)/bin/golangci-lint:
	curl -L --output - $(GOLANGCI_LINT_URL) | \
		tar xz --strip-components 1 -C $(GOPATH)/bin/ golangci-lint-$(GOLANGCI_LINT_VERSION)-$(GOLANGCI_LINT_OS)-amd64/golangci-lint || \
		(echo "curl returned $$? trying to fetch golangci-lint. please install golangci-lint and try again"; exit 1); \
	GOLANGCI_LINT_CACHE=/tmp/golangci-cache $(GOPATH)/bin/golangci-lint version
	GOLANGCI_LINT_CACHE=/tmp/golangci-cache $(GOPATH)/bin/golangci-lint linters

.PHONY: image
image: default-image centos-image fedora-image

.PHONY: default-image
default-image:
	$(MAKE) el8-image

# backwards compatibility
.PHONY: centos-image
centos-image:
	$(MAKE) el8-image
	$(CONTAINER_RUNTIME) tag $(EL8_IMAGE_REPO) $(CENTOS_IMAGE_REPO)

.PHONY: el8-image
el8-image:
	$(CONTAINER_RUNTIME) build -f images/el8/Dockerfile -t $(EL8_IMAGE_REPO) .

.PHONY: el9-image
el9-image:
	$(CONTAINER_RUNTIME) build -f images/el9/Dockerfile -t $(EL9_IMAGE_REPO) .

.PHONY: fedora-image
fedora-image:
	$(CONTAINER_RUNTIME) build -f images/fedora/Dockerfile -t $(FEDORA_IMAGE_REPO) .

.PHONY: push
push:
	$(CONTAINER_RUNTIME) push $(IMAGE_REPO)

image.tar:
	$(MAKE) $(TEST_OS)-image && \
	$(CONTAINER_RUNTIME) save -o image.tar quay.io/security-profiles-operator/$(IMAGE_NAME)-$(TEST_OS):$(IMAGE_TAG); \

.PHONY: vagrant-up
vagrant-up: image.tar ## Boot the vagrant based test VM
	ln -sf hack/ci/Vagrantfile-$(TEST_OS) ./Vagrantfile
	# Retry in case provisioning failed because of some temporarily unavailable
	# remote resource (like the VM image)
	vagrant up || vagrant up || vagrant up
