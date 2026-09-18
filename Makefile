.PHONY: fmt lint test build dashboard

fmt:
	gofmt -l -w .

# internal/httpapi imports the dashboard package (dashboard/embed.go),
# whose //go:embed target must contain at least one file to compile at
# all — go vet/build/test all fail on a totally fresh checkout without
# this. dashboard/dist/index.html is now a tracked placeholder (needed by
# the unified CI Scope go-quality check, which runs bare go vet/gofmt with
# no bootstrap step) — the rest of dashboard/dist stays gitignored, and
# Vite's build still empties/overwrites the directory on every real build.
# This rule is now a no-op on a fresh checkout; it only matters if the
# tracked placeholder is ever deleted without a real `make dashboard` run.
dashboard/dist/index.html:
	mkdir -p dashboard/dist
	printf '<!doctype html><title>Mortris</title><body>dashboard not built yet — run `make dashboard`</body>' > dashboard/dist/index.html

lint: dashboard/dist/index.html
	test -x deploy/backup/sync-to-drive.sh
	test -x deploy/smoke-test.sh
	go vet ./cmd/... ./internal/...
	gofmt -l . | (! grep .)

test: dashboard/dist/index.html
	go test ./cmd/... ./internal/...
	cd dashboard && npm test

# Builds the real Vite frontend into dashboard/dist, which Go embeds
# (dashboard/embed.go, section 13.1).
dashboard:
	cd dashboard && npm ci && npm run build

build: dashboard
	chmod 0755 deploy/backup/sync-to-drive.sh
	go build -o bin/analytics-server ./cmd/analytics-server
	go build -o bin/mcp-analytics ./cmd/mcp-analytics
