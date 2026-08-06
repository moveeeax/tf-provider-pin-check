package scanner

import "testing"

func TestClassifyModule(t *testing.T) {
	cases := []struct {
		src  string
		want moduleKind
	}{
		{"./modules/vpc", kindLocal},
		{"../shared", kindLocal},
		{"terraform-aws-modules/vpc/aws", kindRegistry},
		{"git::https://github.com/org/repo.git", kindGit},
		{"github.com/org/repo", kindGit},
		{"git@github.com:org/repo.git", kindGit},
	}
	for _, c := range cases {
		if got := classifyModule(c.src); got != c.want {
			t.Errorf("classifyModule(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}

func TestGitRefPinned(t *testing.T) {
	cases := []struct {
		src  string
		want bool
	}{
		{"git::https://github.com/org/repo.git", false},                                             // no ref
		{"git::https://github.com/org/repo.git?ref=main", false},                                    // branch
		{"git::https://github.com/org/repo.git?ref=master", false},                                  // branch
		{"git::https://github.com/org/repo.git?ref=v1.2.0", true},                                   // tag
		{"git::https://github.com/org/repo.git?ref=0123456789abcdef0123456789abcdef01234567", true}, // sha
		{"git::https://github.com/org/repo.git//modules/vpc?ref=v0.1.0", true},
	}
	for _, c := range cases {
		if got := gitRefPinned(c.src); got != c.want {
			t.Errorf("gitRefPinned(%q) = %v, want %v", c.src, got, c.want)
		}
	}
}
