GO ?= $(firstword $(shell command -v go) $(wildcard /usr/local/go/bin/go $(HOME)/.local/go/bin/go $(HOME)/go/bin/go) go)
TEST_ENV = PATH="$(dir $(GO)):$$PATH"

.PHONY: install test test-live claude-dangerous

install:
	npm ci
	$(GO) mod download

test:
	$(TEST_ENV) $(GO) test ./...

test-live:
	$(TEST_ENV) PILOTAB_LIVE=1 $(GO) test ./e2e/ -count=1 -v

claude-dangerous:
	PILOTAB_DANGEROUSLY_SKIP_CONFIRMATIONS=1 claude $(ARGS)
