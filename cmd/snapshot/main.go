// Command snapshot refreshes data/snapshot.json, the copy of the star history
// and the contributors that ships inside the binary.
//
// It pages through every stargazer, so it needs a token:
//
//	GITHUB_TOKEN=$(gh auth token) go run ./cmd/snapshot > data/snapshot.json
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/Gaurav-Gosain/tuios-5k/internal/gh"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	c := &gh.Client{Token: os.Getenv("GITHUB_TOKEN")}
	d, err := c.Snapshot(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	if err := enc.Encode(d); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
}
