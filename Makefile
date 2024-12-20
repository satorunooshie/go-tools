.PHONY: all
all: gopls cmd

.PHONY: gopls
install:
	cd golang.org/x/tools/gopls; go install

.PHONY: cmd
cmd:
	go install ./cmd/...

.PHONY: test
test:
	cd golang.org/x/tools; go test ./custom/...

.PHONY: sync
sync:
	git subtree pull --prefix=golang.org/x/tools https://go.googlesource.com/tools master --squash -m 'make sync'
	go mod tidy
	@if ! git diff --quiet; then \
	  git add -u; \
	  git commit -m 'update go.mod'; \
	fi
