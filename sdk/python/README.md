# `kestrai` — Python agent SDK

The Python SDK is the first-class way to write Kestrai agents. It is shipped from this monorepo and published to PyPI as the `kestrai` package.

This package is **Phase 0 stub**. The real agent surface (decorators, tool gateway client, model router client, lifecycle hooks) lands in Phase 1.

## Install

```bash
uv add kestrai
# or, from a clone:
uv pip install -e .
```

## Workspace layout

This package is a member of the repo-root [uv workspace](../../pyproject.toml). To work on it from a clone:

```bash
# From repo root
uv sync
uv run pytest
```
