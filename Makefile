PROJECT_NAME := keepalived-exporter
PKG := "github.com/cafebazaar/$(PROJECT_NAME)"
PKG_LIST := $(shell go list ${PKG}/... | grep -v /vendor/)
GO_FILES := $(shell find . -name '*.go' | grep -v /vendor/ | grep -v _test.go)
LINTER = golangci-lint
LINTER_VERSION = v1.24.0
COMMIT := $(shell git rev-parse HEAD)
VERSION := $(shell git describe --tags ${COMMIT} | cut -c2-)
ARCH ?= $(shell dpkg --print-architecture)
ARCH_RPM ?= $(shell uname --hardware-platform)
BUILD ?= 1
RELEASE_FILENAME := $(PROJECT_NAME)-$(VERSION).linux-$(ARCH)
RELEASE_FILENAME_ZIP := $(RELEASE_FILENAME).zip
RELEASE_FILENAME_TARGZ := $(RELEASE_FILENAME).tar.gz

RPM_VERSION := $(shell git describe --tags ${COMMIT} | cut -d- -f1 | cut -c2-)
RELEASE_FILENAME_RPM_TARGZ := $(PROJECT_NAME)-$(RPM_VERSION).tar.gz
RELEASE_FILENAME_RPM = $(PROJECT_NAME)-$(RPM_VERSION)-$(BUILD).$(ARCH_RPM).rpm

RPM_SPEC = $(PROJECT_NAME).spec
RPMBUILD = rpmbuild
RPMSIGN = rpmsign
DOCKER_USER ?= builder

ifndef RPM_VERBOSE
	RPM_VERBOSE := --quiet
endif

.PHONY: all dep lint build clean rpm rpm-version rpm-docker

all: build

dep: ## Get the dependencies
	@go mod tidy

lintdeps: ## golangci-lint dependencies
	curl -sfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $(GOPATH)/bin $(LINTER_VERSION)

lint: lintdeps ## to lint the files
	$(LINTER) run --config=.golangci-lint.yml ./...

build: dep ## Build the binary file
	@go build -i -v $(PKG)/cmd/$(PROJECT_NAME)

test:
	@go test -v -cover -race ./...

clean: ## Remove previous build and release files
	@rm -f $(PROJECT_NAME)
	@rm -f $(RELEASE_FILENAME_ZIP)
	@rm -f $(RELEASE_FILENAME_TARGZ)

$(RELEASE_FILENAME):
	@mkdir $(RELEASE_FILENAME)
	@cp $(PROJECT_NAME) $(RELEASE_FILENAME)/
	@cp LICENSE $(RELEASE_FILENAME)/

$(RELEASE_FILENAME_ZIP): $(RELEASE_FILENAME)
	@zip -r $(RELEASE_FILENAME_ZIP) $(RELEASE_FILENAME)

$(RELEASE_FILENAME_TARGZ): $(RELEASE_FILENAME) $(RPM_SPEC)
	@tar -czvf $(RELEASE_FILENAME_TARGZ) $(RELEASE_FILENAME)

$(RELEASE_FILENAME_RPM_TARGZ): $(RELEASE_FILENAME) $(RPM_SPEC) Makefile
	@rm -rf $(PROJECT_NAME)-$(RPM_VERSION)
	@cp -r $(RELEASE_FILENAME) $(PROJECT_NAME)-$(RPM_VERSION)
	@cp $(RPM_SPEC) $(PROJECT_NAME)-$(RPM_VERSION)/
	@cp Makefile $(PROJECT_NAME)-$(RPM_VERSION)/
	@cp -r lib $(PROJECT_NAME)-$(RPM_VERSION)/
	@tar -czvf $(RELEASE_FILENAME_RPM_TARGZ) $(PROJECT_NAME)-$(RPM_VERSION)

tar: $(RELEASE_FILENAME_TARGZ)

release: $(RELEASE_FILENAME_ZIP) $(RELEASE_FILENAME_TARGZ)
	@rm -rf $(RELEASE_FILENAME)

$(RPM_SPEC): $(RPM_SPEC).in
	sed -e 's/@@VERSION@@/$(RPM_VERSION)/g' < $< > $@+
	mv $@+ $@

rpm-version:
	@echo $(RPM_VERSION)

rpm: $(RELEASE_FILENAME_RPM) $(RPM_SPEC)

rpm-docker:
	docker run --user $(DOCKER_USER) --rm --volume $(PWD)/rpm-pkg:/srv/pkg --env ARCH=$(ARCH) --volume $(PWD):/home/builder/app:z --workdir /home/builder/app rpmbuild/centos7

rpm-sign: $(RELEASE_FILENAME_RPM)
	$(RPMSIGN) --resign --key-id "$(RPM_SIGN_KEY)" "$(RELEASE_FILENAME_RPM)"

$(RELEASE_FILENAME_RPM): $(RELEASE_FILENAME_RPM_TARGZ)
	$(RPMBUILD) \
		$(RPM_VERBOSE) \
		--clean \
		--target $(ARCH_RPM) \
		--define "Release $(BUILD)" \
		-tb $(RELEASE_FILENAME_RPM_TARGZ) \
		--define "dist .R" \
		--define "_rpmdir ." \
		--define "_build_name_fmt %%{NAME}-%%{VERSION}-%%{RELEASE}.%%{ARCH}.rpm"

install: $(DESTDIR)$(PREFIX)/bin $(DESTDIR)/lib/systemd/system
	$(INSTALL) -m 755  $(PROJECT_NAME) $(DESTDIR)$(PREFIX)/bin/
	$(INSTALL) -m 644  lib/systemd/system/$(PROJECT_NAME).service $(DESTDIR)/lib/systemd/system/$(PROJECT_NAME).service
