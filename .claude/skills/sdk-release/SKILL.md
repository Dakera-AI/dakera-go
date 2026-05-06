---
name: sdk-release
description: Release the Dakera Go SDK. Use when publishing a new module version.
disable-model-invocation: true
allowed-tools: Bash(gh *) Bash(go *)
---

## Go SDK Release

### Pre-release checks
```bash
go vet ./...
go test ./...
golangci-lint run
```

### Version bump
Go modules use git tags — no version file to update.

### Release process
1. Update `CHANGELOG.md`
2. Commit: `git commit -m "chore: bump to vX.Y.Z"`
3. Tag: `git tag vX.Y.Z`
4. Push: `git push origin main --tags`
5. Go proxy auto-indexes new tags

### Batching rules
- All 4 SDKs (py, js, rs, go) sync in a single coordinated batch
- Do NOT release for a single trivial change — batch until 2+ changes or security fix
