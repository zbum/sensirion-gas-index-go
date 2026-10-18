GO ?= go

.PHONY: test test-cgo test-purego

test: test-cgo test-purego

test-cgo:
	CGO_ENABLED=1 $(GO) test ./...

test-purego:
	CGO_ENABLED=0 $(GO) test ./...
