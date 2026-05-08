package compressor

// ContentType identifies the kind of content detected in a tool result.
type ContentType int

const (
	ContentGeneric   ContentType = iota
	ContentCodeFile              // priority 1 — skip compression
	ContentGitDiff               // priority 2
	ContentGitStatus             // priority 3
	ContentGitLog                // priority 4
	ContentGoTest                // priority 5
	ContentCargoTest             // priority 6
	ContentPytest                // priority 7
	ContentLS                    // priority 8
	ContentLog                   // priority 9
	ContentJSON                  // priority 10
)

func (ct ContentType) String() string {
	switch ct {
	case ContentCodeFile:
		return "code-file"
	case ContentGitDiff:
		return "git-diff"
	case ContentGitStatus:
		return "git-status"
	case ContentGitLog:
		return "git-log"
	case ContentGoTest:
		return "go-test"
	case ContentCargoTest:
		return "cargo-test"
	case ContentPytest:
		return "pytest"
	case ContentLS:
		return "ls"
	case ContentLog:
		return "log"
	case ContentJSON:
		return "json"
	default:
		return "generic"
	}
}

// Options configures compressor behaviour.
type Options struct {
	Enabled          bool
	MinContentLength int  // default 500
	LogSavings       bool
}

// Stats carries per-request compression statistics.
type Stats struct {
	OriginalBytes   int
	CompressedBytes int
	TokensSaved     int            // (orig-comp)/4
	Detections      map[string]int // content-type name → count
	Skipped         int
}
