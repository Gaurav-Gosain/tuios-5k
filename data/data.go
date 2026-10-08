// Package data embeds the snapshot the app falls back to when GitHub cannot
// be reached. Refresh it with `go run ./cmd/snapshot > data/snapshot.json`.
package data

import _ "embed"

// Snapshot is the JSON form of gh.Data, taken by cmd/snapshot.
//
//go:embed snapshot.json
var Snapshot []byte
