// Package dashboard embeds the built Vite frontend (section 13.1: "CI
// builds the dashboard, then Go embeds its dist directory with
// //go:embed"). dist/ is entirely gitignored — Vite empties it on every
// build, so a tracked placeholder inside it would just get wiped. The
// //go:embed directive below still needs at least one file to exist at
// compile time, so `make lint`/`make test`/`make build` all depend on a
// Makefile rule that creates a cheap placeholder if nothing's been built
// yet; `make build`'s real `npm run build` overwrites it. Compiling this
// package directly (bypassing make) requires dashboard/dist to already
// exist — run `make dashboard` or `npm run build` in dashboard/ first.
package dashboard

import "embed"

//go:embed all:dist
var DistFS embed.FS
