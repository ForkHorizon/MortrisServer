.PHONY: fmt lint test build dashboard

fmt:
	gofmt -l -w .

# internal/httpapi imports the dashboard package (dashboard/embed.go),
# whose //go:embed target must contain at least one file to compile at
# all — go vet/build/test all fail on a totally fresh checkout without
# this. dashboard/dist is gitignored (real builds go through Vite, which
# empties the directory on every run — a tracked placeholder there would
# just get wiped), so this rule creates a cheap one instead. `make build`
# (via the `dashboard` target below) overwrites it with the real thing;
# once that's happened this rule is a no-op (the file already exists).
dashboard/dist/index.html:
	mkdir -p dashboard/dist
	printf '<!doctype html><title>Mortris</title><body>dashboard not built yet — run `make dashboard`</body>' > dashboard/dist/index.html

lint: dashboard/dist/index.html
	go vet ./...
	gofmt -l . | (! grep .)

test: dashboard/dist/index.html
	go test ./...

# Builds the real Vite frontend into dashboard/dist, which Go embeds
# (dashboard/embed.go, section 13.1).
dashboard:
	cd dashboard && npm ci && npm run build

build: dashboard
	go build -o bin/analytics-server ./cmd/analytics-server
