package scanner

import (
	"net/url"
	"regexp"
	"strings"
)

// branchRefs are common mutable branch names that must never be used as a git
// module pin — they move, so they are not reproducible.
var branchRefs = map[string]bool{
	"main": true, "master": true, "develop": true, "trunk": true, "head": true,
}

// sha1Ref matches a full 40-character git commit SHA.
var sha1Ref = regexp.MustCompile(`^[0-9a-f]{40}$`)

// moduleKind classifies a module source string.
type moduleKind int

const (
	kindLocal    moduleKind = iota // ./ or ../ — no pin required
	kindRegistry                   // Terraform registry address — needs version
	kindGit                        // git/github/bitbucket — needs a tag or SHA ref
)

func classifyModule(source string) moduleKind {
	s := strings.TrimSpace(source)
	switch {
	case strings.HasPrefix(s, "./"), strings.HasPrefix(s, "../"), s == ".":
		return kindLocal
	case isGitSource(s):
		return kindGit
	default:
		return kindRegistry
	}
}

func isGitSource(s string) bool {
	switch {
	case strings.HasPrefix(s, "git::"),
		strings.HasPrefix(s, "git@"),
		strings.HasPrefix(s, "github.com/"),
		strings.HasPrefix(s, "git::https://"),
		strings.Contains(s, "bitbucket.org/"),
		strings.HasSuffix(strings.SplitN(s, "?", 2)[0], ".git"):
		return true
	}
	return false
}

// gitRefPinned reports whether a git module source pins to a tag or commit SHA.
// A missing ref, or a ref pointing at a mutable branch, is not a pin.
func gitRefPinned(source string) bool {
	ref := extractRef(source)
	if ref == "" {
		return false
	}
	if branchRefs[strings.ToLower(ref)] {
		return false
	}
	// A full SHA or any non-branch tag (e.g. v1.2.0) counts as pinned.
	return sha1Ref.MatchString(strings.ToLower(ref)) || ref != ""
}

// extractRef pulls the ref query parameter out of a git source string.
func extractRef(source string) string {
	i := strings.Index(source, "?")
	if i < 0 {
		return ""
	}
	q := source[i+1:]
	if vals, err := url.ParseQuery(q); err == nil {
		if ref := vals.Get("ref"); ref != "" {
			return ref
		}
	}
	// Fall back to a manual scan for odd encodings.
	for _, kv := range strings.Split(q, "&") {
		if strings.HasPrefix(kv, "ref=") {
			return strings.TrimPrefix(kv, "ref=")
		}
	}
	return ""
}
