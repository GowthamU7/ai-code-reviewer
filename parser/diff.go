package parser

import (
	"strings"
)

// FileDiff represents the changes in a single file
type FileDiff struct {
	Filename string
	Content  string // the raw diff chunk for this file
	Language string // detected from file extension
}

// ParseDiff takes a raw git diff string and splits it into per-file diffs
// A git diff looks like:
//
//	diff --git a/main.go b/main.go
//	index abc..def 100644
//	--- a/main.go
//	+++ b/main.go
//	@@ -1,5 +1,8 @@
//	 package main
//	+
//	+import "fmt"
func ParseDiff(rawDiff string) []FileDiff {
	var files []FileDiff
	var current *FileDiff

	lines := strings.Split(rawDiff, "\n")

	for _, line := range lines {
		// "diff --git a/foo.go b/foo.go" marks start of a new file
		if strings.HasPrefix(line, "diff --git") {
			// Save the previous file if there was one
			if current != nil {
				files = append(files, *current)
			}
			// Start a new file diff
			current = &FileDiff{}
			// Extract filename from "diff --git a/foo.go b/foo.go"
			// We want the "b/" version which is the new file path
			parts := strings.Split(line, " b/")
			if len(parts) == 2 {
				current.Filename = parts[1]
				current.Language = detectLanguage(parts[1])
			}
		} else if current != nil {
			// Accumulate all lines for this file
			current.Content += line + "\n"
		}
	}

	// Don't forget the last file
	if current != nil {
		files = append(files, *current)
	}

	// Filter out files we can't meaningfully review
	return filterReviewable(files)
}

// filterReviewable removes binary files, lock files, generated files etc
// Sending these to an LLM wastes tokens and produces useless reviews
func filterReviewable(files []FileDiff) []FileDiff {
	skip := map[string]bool{
		"":      true,
		"lock":  true,
		"sum":   true,
		"mod":   true,
		"pb.go": true, // generated protobuf
	}

	skipPrefixes := []string{
		"vendor/",
		"node_modules/",
		".github/",
		"dist/",
	}

	var reviewable []FileDiff
	for _, f := range files {
		// Skip if extension is in blocklist
		ext := fileExtension(f.Filename)
		if skip[ext] {
			continue
		}

		// Skip vendor/generated directories
		blocked := false
		for _, prefix := range skipPrefixes {
			if strings.HasPrefix(f.Filename, prefix) {
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		// Skip very large diffs — LLMs have context limits
		if len(f.Content) > 8000 {
			f.Content = f.Content[:8000] + "\n... (truncated, diff too large)"
		}

		reviewable = append(reviewable, f)
	}
	return reviewable
}

// detectLanguage maps file extensions to language names
// This goes into the prompt so the LLM knows what it's reviewing
func detectLanguage(filename string) string {
	ext := fileExtension(filename)
	langs := map[string]string{
		"go":   "Go",
		"py":   "Python",
		"js":   "JavaScript",
		"ts":   "TypeScript",
		"tsx":  "TypeScript React",
		"jsx":  "JavaScript React",
		"java": "Java",
		"rs":   "Rust",
		"cs":   "C#",
		"cpp":  "C++",
		"c":    "C",
		"rb":   "Ruby",
		"php":  "PHP",
		"sql":  "SQL",
		"yaml": "YAML",
		"yml":  "YAML",
		"json": "JSON",
		"md":   "Markdown",
		"sh":   "Shell",
	}
	if lang, ok := langs[ext]; ok {
		return lang
	}
	return "unknown"
}

func fileExtension(filename string) string {
	parts := strings.Split(filename, ".")
	if len(parts) < 2 {
		return ""
	}
	return parts[len(parts)-1]
}
