# tf-provider-pin-check

[![ci](https://github.com/moveeeax/tf-provider-pin-check/actions/workflows/ci.yml/badge.svg)](https://github.com/moveeeax/tf-provider-pin-check/actions/workflows/ci.yml)
[![Go](https://img.shields.io/badge/go-1.26%2B-00ADD8?logo=go&logoColor=white)](go.mod)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

A fast CI check that fails when any Terraform **provider** or **module** reference
is not pinned to an explicit version. Floating versions (`>= 5.0`, `latest`, a git
`?ref=main`, or a module with no `version` at all) make `terraform init` produce a
different dependency set tomorrow than it did today — this catches them before they
land on `main`.

## What it flags

| Rule | Fires when |
| --- | --- |
| `provider-unpinned` | a `required_providers` entry has no `version`, or a constraint that isn't anchored to a release (`>= 5.0`, `latest`, `*`) |
| `module-unpinned` | a registry module has no `version` or an unanchored constraint |
| `module-git-ref` | a `git::` / `github.com/` module source has no `?ref=`, or the ref is a mutable branch (`main`, `master`, `develop`) |

An exact version (`5.31.0`, `= 5.31.0`) and the pessimistic operator (`~> 5.0`)
count as pinned. Local module sources (`./`, `../`) are ignored. Pass `-strict`
to reject `~>` too and demand exact versions everywhere.

## Install

```bash
go install github.com/moveeeax/tf-provider-pin-check@latest
```

Or build from source:

```bash
git clone https://github.com/moveeeax/tf-provider-pin-check
cd tf-provider-pin-check
go build -o tf-provider-pin-check .
```

## Usage

```console
$ tf-provider-pin-check ./examples/bad
main.tf:3:5: [provider-unpinned] provider "aws" version ">= 5.0" is not pinned to an explicit release
main.tf:7:5: [provider-unpinned] provider "random" has no version constraint in required_providers
main.tf:14:3: [module-unpinned] module "vpc" (terraform-aws-modules/vpc/aws) has no version constraint
main.tf:18:3: [module-git-ref] module "app" git source "git::https://github.com/acme/app.git?ref=main" must pin a tag or commit SHA via ?ref=

4 unpinned reference(s) found
```

Exit code is `0` when everything is pinned and `1` when there is at least one
finding — drop it straight into CI. `-format json` emits machine-readable output.

## Exceptions

Drop a `.tf-pin-check.yml` in the directory you scan (or point at one with
`-config`):

```yaml
# Providers and modules allowed to stay unpinned, plus paths to skip.
providers:
  - random
modules:
  - git::https://git.internal/tf-shared.git
paths:
  - "**/.terraform/**"
strict: false
```

## As a GitHub Action

```yaml
- uses: moveeeax/tf-provider-pin-check@v0
  with:
    path: infra
    strict: "false"
```

## As a pre-commit hook

```yaml
repos:
  - repo: https://github.com/moveeeax/tf-provider-pin-check
    rev: v0.1.0
    hooks:
      - id: tf-provider-pin-check
```

## How it works

Each `.tf` file is parsed with the HashiCorp HCL v2 parser (`hclsyntax`), so
findings carry a real `file:line:column` from the source range rather than a
regex guess. The scanner walks `terraform → required_providers`, legacy
`provider` blocks, and `module` blocks, classifies each reference, and applies
the constraint rules above. `.terraform/` and `.git/` directories are skipped.

## License

MIT — see [LICENSE](LICENSE).

---

**Archived on 2026-11-05.** tflint's terraform ruleset already ships all three checks (`terraform_required_providers`, `terraform_module_version`, `terraform_module_pinned_source`), so the current feature set is redundant. The repo is only worth keeping if it adds lock-file checks (feature 1), which tflint does not do.
