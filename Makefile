PROJECT=selinuxd
BIN=$(BINDIR)/selinuxdctl
BINDIR=bin
POLICYDIR=/etc/selinux.d

SRC=$(shell find . -name "*.go")

GO?=go

GO_PROJECT := github.com/containers/$(PROJECT)

# External Helper variables

GOLANGCI_LINT_VERSION=1.33.0
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
CENTOS_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-centos:$(IMAGE_TAG)
FEDORA_IMAGE_REPO?=quay.io/security-profiles-operator/$(IMAGE_NAME)-fedora:$(IMAGE_TAG)

TEST_OS?=fedora

DATE_FMT = +'%Y-%m-%dT%H:%M:%SZ'
BUILD_DATE ?= $(shell date -u "$(DATE_FMT)")
VERSION := $(shell cat VERSION)

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

.PHONY: set-release-tag
set-release-tag:
	$(eval IMAGE_TAG = $(VERSION))

.PHONY: image
image: default-image centos-image fedora-image

.PHONY: release-image
release-image: set-release-tag default-image centos-image fedora-image push push-fedora
	# This will ensure that we also push to the latest tag
	$(eval IMAGE_TAG = latest)
	$(MAKE) push

.PHONY: default-image
default-image:
	$(CONTAINER_RUNTIME) build -f images/Dockerfile.centos -t $(IMAGE_REPO) .

.PHONY: centos-image
centos-image:
	$(CONTAINER_RUNTIME) build -f images/Dockerfile.centos -t $(CENTOS_IMAGE_REPO) .

.PHONY: fedora-image
fedora-image:
	$(CONTAINER_RUNTIME) build -f images/Dockerfile.fedora -t $(FEDORA_IMAGE_REPO) .

.PHONY: push
push: default-image
	$(CONTAINER_RUNTIME) push $(IMAGE_REPO)

.PHONY: push-fedora
push-fedora: fedora-image
	$(CONTAINER_RUNTIME) push $(FEDORA_IMAGE_REPO)

image.tar:
	$(MAKE) $(TEST_OS)-image && \
	$(CONTAINER_RUNTIME) save -o image.tar quay.io/security-profiles-operator/$(IMAGE_NAME)-$(TEST_OS):$(IMAGE_TAG); \

.PHONY: vagrant-up
vagrant-up: image.tar ## Boot the vagrant based test VM
	ln -sf hack/ci/Vagrantfile-$(TEST_OS) ./Vagrantfile
	# Retry in case provisioning failed because of some temporarily unavailable
	# remote resource (like the VM image)
	vagrant up || vagrant up || vagrant up

.PHONY: check-release-version
check-release-version:
ifndef RELEASE_VERSION
	$(error RELEASE_VERSION must be defined)
endif

.PHONY: commit-release-version
commit-release-version: check-release-version
	echo $(RELEASE_VERSION) > VERSION
	git add VERSION
	git commit -s -m "Release v$(RELEASE_VERSION)"
	$(eval VERSION = $(RELEASE_VERSION))

.PHONY: next-version
next-version:
	@grep '^[0-9]\+.[0-9]\+.[0-9]\+$$' VERSION
	sed -i "s/\([0-9]\+\.[0-9]\+\.[0-9]\+\)/\1.99/" VERSION
	git add VERSION
	git commit -s -m "Prepare VERSION for the next release"

.PHONY: tag-release
tag-release:
	git tag "v$(VERSION)"
	git push origin "v$(VERSION)"

.PHONY: release
release: commit-release-version tag-release release-image next-version
