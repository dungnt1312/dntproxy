package version

// Version is the current application version.
// Override at build time: go build -ldflags "-X github.com/dungnt/dntproxy/internal/version.Version=0.3.0"
var Version = "dev"
