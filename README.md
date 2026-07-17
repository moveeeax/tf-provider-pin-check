# tf-provider-pin-check

> No floating versions: enforce reproducible Terraform across the repo.

**Status:** 🚧 In development

## Overview

CI check that every Terraform provider and module reference is pinned to an explicit version.

## Features

- Scans `.tf` for provider blocks and module sources without version pins
- Flags `latest`, unpinned git refs, and missing `required_providers` constraints
- Configurable allowlist
- Clear file:line diagnostics
- Fast recursive scan

## Stack

Go + HCL parser.

## Usage

```bash
tf-provider-pin-check ./infra
```

## License

MIT
