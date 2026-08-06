package scanner

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/moveeeax/tf-provider-pin-check/internal/config"
)

func writeTF(t *testing.T, dir, name, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestScanFlagsUnpinned(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "bad.tf", `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = ">= 5.0"
    }
    random = {
      source = "hashicorp/random"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
  }
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
}

module "sg" {
  source  = "terraform-aws-modules/security-group/aws"
  version = ">= 4.0"
}

module "app" {
  source = "git::https://github.com/org/app.git?ref=main"
}

module "db" {
  source = "git::https://github.com/org/db.git?ref=v1.4.0"
}

module "local" {
  source = "./modules/thing"
}
`)

	findings, err := New(dir, &config.Config{}).Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}

	byName := map[string]Finding{}
	for _, f := range findings {
		byName[f.Name] = f
	}

	wantFlagged := []struct {
		name string
		rule Rule
	}{
		{"aws", RuleProviderUnpinned},
		{"random", RuleProviderUnpinned},
		{"vpc", RuleModuleUnpinned},
		{"sg", RuleModuleUnpinned},
		{"app", RuleModuleGitRef},
	}
	for _, w := range wantFlagged {
		f, ok := byName[w.name]
		if !ok {
			t.Errorf("expected %q to be flagged, but it was not", w.name)
			continue
		}
		if f.Rule != w.rule {
			t.Errorf("%q flagged with rule %q, want %q", w.name, f.Rule, w.rule)
		}
		if f.Line == 0 {
			t.Errorf("%q flagged without a line number", w.name)
		}
	}

	wantClean := []string{"tls", "db", "local"}
	for _, name := range wantClean {
		if _, ok := byName[name]; ok {
			t.Errorf("%q should be pinned but was flagged", name)
		}
	}

	if len(findings) != len(wantFlagged) {
		t.Errorf("got %d findings, want %d: %+v", len(findings), len(wantFlagged), findings)
	}
}

func TestScanCleanTree(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "main.tf", `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "5.31.0"
    }
  }
}

module "vpc" {
  source  = "terraform-aws-modules/vpc/aws"
  version = "5.5.1"
}
`)
	findings, err := New(dir, &config.Config{}).Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected no findings, got %+v", findings)
	}
}

func TestConfigExceptions(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "main.tf", `
terraform {
  required_providers {
    random = {
      source = "hashicorp/random"
    }
  }
}

module "vpc" {
  source = "terraform-aws-modules/vpc/aws"
}
`)
	cfg := &config.Config{
		Providers: []string{"random"},
		Modules:   []string{"terraform-aws-modules/vpc/aws"},
	}
	findings, err := New(dir, cfg).Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("allowlisted references should be skipped, got %+v", findings)
	}
}

func TestSkipsTerraformDir(t *testing.T) {
	dir := t.TempDir()
	sub := filepath.Join(dir, ".terraform", "modules")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	writeTF(t, sub, "vendored.tf", `
terraform {
  required_providers {
    aws = { source = "hashicorp/aws" }
  }
}
`)
	findings, err := New(dir, &config.Config{}).Scan()
	if err != nil {
		t.Fatalf("scan error: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("files under .terraform should be ignored, got %+v", findings)
	}
}

func TestStrictRejectsPessimistic(t *testing.T) {
	dir := t.TempDir()
	writeTF(t, dir, "main.tf", `
terraform {
  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 5.0"
    }
  }
}
`)
	loose, _ := New(dir, &config.Config{}).Scan()
	if len(loose) != 0 {
		t.Errorf("~> should pass without strict, got %+v", loose)
	}
	strict, _ := New(dir, &config.Config{Strict: true}).Scan()
	if len(strict) != 1 {
		t.Errorf("~> should fail under strict, got %+v", strict)
	}
}
