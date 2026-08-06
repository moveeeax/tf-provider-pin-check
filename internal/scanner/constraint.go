package scanner

import (
	"regexp"
	"strings"
)

// exactVersion matches a bare version like 1.2.3 or 1.2 (no operator).
var exactVersion = regexp.MustCompile(`^[0-9]+(\.[0-9]+){0,2}$`)

// pinnedConstraint reports whether a Terraform version constraint pins to a
// concrete release. The rules are deliberately opinionated:
//
//   - An empty constraint is never pinned.
//   - "latest" or a bare "*" wildcard is never pinned.
//   - An exact comparison ("= 1.2.3" or a bare "1.2.3") is always pinned.
//   - The pessimistic operator "~> 1.2" is pinned unless strict is set.
//   - Open-ended ranges (">=", ">", "<", "<=", "!=") on their own are unpinned.
//
// A constraint may be a comma-separated list; it is pinned only when at least
// one comparison provides a concrete floor and none is an open wildcard.
func pinnedConstraint(raw string, strict bool) bool {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return false
	}
	if strings.EqualFold(raw, "latest") {
		return false
	}
	anchored := false
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if part == "*" || strings.Contains(part, "latest") {
			return false
		}
		op, ver := splitConstraint(part)
		switch op {
		case "=", "":
			if exactVersion.MatchString(ver) {
				anchored = true
			}
		case "~>":
			if !strict && ver != "" {
				anchored = true
			}
		case ">=", ">", "<=", "<", "!=":
			// Open-ended: contributes no concrete pin on its own.
		}
	}
	return anchored
}

// splitConstraint separates the leading operator from the version in one
// comparison such as ">= 1.2.0".
func splitConstraint(part string) (op, ver string) {
	for _, candidate := range []string{"~>", ">=", "<=", "!=", ">", "<", "="} {
		if strings.HasPrefix(part, candidate) {
			return candidate, strings.TrimSpace(strings.TrimPrefix(part, candidate))
		}
	}
	return "", strings.TrimSpace(part)
}
