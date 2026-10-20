// Package version reports the git commit this process was built or deployed from.
package version

import (
	"os"
	"strings"
)

// commit is the build-time SHA. Set it with
// -ldflags "-X affilio-api/internal/version.commit=<sha>".
// An empty value means the binary was built without a SHA.
var commit = "unknown"

// Commit is the deployed SHA. A runtime host value wins, then the value
// baked in at build time, then "unknown".
func Commit() string {
	return Resolve(commit, os.Getenv("RENDER_GIT_COMMIT"), os.Getenv("RAILWAY_GIT_COMMIT_SHA"))
}

// Resolve picks the first non-empty value: Render's runtime SHA, Railway's
// runtime SHA, then the build-time value. A blank result is "unknown".
func Resolve(build, render, railway string) string {
	for _, raw := range []string{render, railway, build} {
		if value := strings.TrimSpace(raw); value != "" {
			return value
		}
	}
	return "unknown"
}
