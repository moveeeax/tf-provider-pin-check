// Package scanner walks Terraform files and reports provider and module
// references that are not pinned to an explicit version.
package scanner

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"

	"github.com/moveeeax/tf-provider-pin-check/internal/config"
)

// Rule identifies the category of a finding.
type Rule string

const (
	RuleProviderUnpinned Rule = "provider-unpinned"
	RuleModuleUnpinned   Rule = "module-unpinned"
	RuleModuleGitRef     Rule = "module-git-ref"
)

// Finding is a single unpinned reference located in a file.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Rule    Rule   `json:"rule"`
	Name    string `json:"name"`
	Message string `json:"message"`
}

// String renders a finding as "file:line:col: [rule] message".
func (f Finding) String() string {
	return fmt.Sprintf("%s:%d:%d: [%s] %s", f.File, f.Line, f.Column, f.Rule, f.Message)
}

// Scanner scans a directory tree of Terraform files.
type Scanner struct {
	cfg  *config.Config
	root string
}

// New builds a Scanner rooted at root using cfg for exceptions.
func New(root string, cfg *config.Config) *Scanner {
	if cfg == nil {
		cfg = &config.Config{}
	}
	return &Scanner{cfg: cfg, root: root}
}

// Scan walks root and returns findings sorted by file then line. Parse errors on
// individual files are returned as a combined error but do not stop the walk.
func (s *Scanner) Scan() ([]Finding, error) {
	var findings []Finding
	var parseErrs []string

	err := filepath.WalkDir(s.root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Never descend into a local Terraform working directory.
			if d.Name() == ".terraform" || d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".tf" {
			return nil
		}
		rel, relErr := filepath.Rel(s.root, path)
		if relErr != nil {
			rel = path
		}
		if s.cfg.SkipsPath(rel) {
			return nil
		}
		fileFindings, ferr := s.scanFile(path, rel)
		if ferr != nil {
			parseErrs = append(parseErrs, ferr.Error())
			return nil
		}
		findings = append(findings, fileFindings...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].File != findings[j].File {
			return findings[i].File < findings[j].File
		}
		return findings[i].Line < findings[j].Line
	})

	if len(parseErrs) > 0 {
		return findings, fmt.Errorf("parse errors:\n%s", strings.Join(parseErrs, "\n"))
	}
	return findings, nil
}

func (s *Scanner) scanFile(path, rel string) ([]Finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, diags := hclsyntax.ParseConfig(src, rel, hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, fmt.Errorf("%s: %s", rel, diags.Error())
	}
	body, ok := f.Body.(*hclsyntax.Body)
	if !ok {
		return nil, nil
	}

	var out []Finding
	for _, block := range body.Blocks {
		switch block.Type {
		case "terraform":
			out = append(out, s.scanTerraform(block, rel)...)
		case "provider":
			out = append(out, s.scanLegacyProvider(block, rel)...)
		case "module":
			out = append(out, s.scanModule(block, rel)...)
		}
	}
	return out, nil
}

// scanTerraform inspects the required_providers map inside a terraform block.
func (s *Scanner) scanTerraform(block *hclsyntax.Block, rel string) []Finding {
	var out []Finding
	for _, inner := range block.Body.Blocks {
		if inner.Type != "required_providers" {
			continue
		}
		names := make([]string, 0, len(inner.Body.Attributes))
		for name := range inner.Body.Attributes {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			attr := inner.Body.Attributes[name]
			if s.cfg.AllowsProvider(name) {
				continue
			}
			constraint, hasVersion := providerConstraint(attr)
			pos := attr.SrcRange.Start
			if !hasVersion {
				out = append(out, Finding{
					File: rel, Line: pos.Line, Column: pos.Column,
					Rule: RuleProviderUnpinned, Name: name,
					Message: fmt.Sprintf("provider %q has no version constraint in required_providers", name),
				})
				continue
			}
			if !pinnedConstraint(constraint, s.cfg.Strict) {
				out = append(out, Finding{
					File: rel, Line: pos.Line, Column: pos.Column,
					Rule: RuleProviderUnpinned, Name: name,
					Message: fmt.Sprintf("provider %q version %q is not pinned to an explicit release", name, constraint),
				})
			}
		}
	}
	return out
}

// providerConstraint reads the version out of a required_providers entry, which
// may be a bare string constraint or an object with source/version keys.
func providerConstraint(attr *hclsyntax.Attribute) (constraint string, hasVersion bool) {
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() {
		return "", false
	}
	if val.Type() == cty.String {
		return val.AsString(), true
	}
	if val.Type().IsObjectType() && val.Type().HasAttribute("version") {
		v := val.GetAttr("version")
		if v.Type() == cty.String {
			return v.AsString(), true
		}
	}
	return "", false
}

// scanLegacyProvider checks the deprecated inline "version" on a provider block.
func (s *Scanner) scanLegacyProvider(block *hclsyntax.Block, rel string) []Finding {
	name := ""
	if len(block.Labels) > 0 {
		name = block.Labels[0]
	}
	if s.cfg.AllowsProvider(name) {
		return nil
	}
	attr, ok := block.Body.Attributes["version"]
	if !ok {
		return nil // version belongs in required_providers; nothing to pin here.
	}
	val, diags := attr.Expr.Value(nil)
	if diags.HasErrors() || val.Type() != cty.String {
		return nil
	}
	if pinnedConstraint(val.AsString(), s.cfg.Strict) {
		return nil
	}
	pos := attr.SrcRange.Start
	return []Finding{{
		File: rel, Line: pos.Line, Column: pos.Column,
		Rule: RuleProviderUnpinned, Name: name,
		Message: fmt.Sprintf("provider %q inline version %q is not pinned", name, val.AsString()),
	}}
}

// scanModule checks a module block's source and version.
func (s *Scanner) scanModule(block *hclsyntax.Block, rel string) []Finding {
	name := ""
	if len(block.Labels) > 0 {
		name = block.Labels[0]
	}
	srcAttr, ok := block.Body.Attributes["source"]
	if !ok {
		return nil
	}
	srcVal, diags := srcAttr.Expr.Value(nil)
	if diags.HasErrors() || srcVal.Type() != cty.String {
		return nil
	}
	source := srcVal.AsString()
	if s.cfg.AllowsModule(source) {
		return nil
	}
	pos := srcAttr.SrcRange.Start

	switch classifyModule(source) {
	case kindLocal:
		return nil
	case kindGit:
		if gitRefPinned(source) {
			return nil
		}
		return []Finding{{
			File: rel, Line: pos.Line, Column: pos.Column,
			Rule: RuleModuleGitRef, Name: name,
			Message: fmt.Sprintf("module %q git source %q must pin a tag or commit SHA via ?ref=", name, source),
		}}
	case kindRegistry:
		verAttr, hasVer := block.Body.Attributes["version"]
		if !hasVer {
			return []Finding{{
				File: rel, Line: pos.Line, Column: pos.Column,
				Rule: RuleModuleUnpinned, Name: name,
				Message: fmt.Sprintf("module %q (%s) has no version constraint", name, source),
			}}
		}
		verVal, vdiags := verAttr.Expr.Value(nil)
		if vdiags.HasErrors() || verVal.Type() != cty.String {
			return nil
		}
		if pinnedConstraint(verVal.AsString(), s.cfg.Strict) {
			return nil
		}
		vpos := verAttr.SrcRange.Start
		return []Finding{{
			File: rel, Line: vpos.Line, Column: vpos.Column,
			Rule: RuleModuleUnpinned, Name: name,
			Message: fmt.Sprintf("module %q version %q is not pinned to an explicit release", name, verVal.AsString()),
		}}
	}
	return nil
}
