set default-list
set quiet

[doc('Run golangci-lint'), no-cd]
lint:
  golangci-lint run ./...

[doc('Run local goreleaser'), no-cd]
dist:
  goreleaser release --clean --snapshot

[doc('Update go.mod dependencies'), no-cd]
update:
  go get -u ./...

[doc('Build binary'), no-cd]
build:
  just _go build

[doc('Build and install binary'), no-cd]
install:
  just _go install

[env('CGO_ENABLED', '0'), no-cd]
_go cmd:
  go {{ cmd }} -trimpath -ldflags "-s -w -buildid=" ./cmd/yfs
