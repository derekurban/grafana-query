# Release

Releases are tag-driven.

## Requirements

- GitHub Actions enabled for this repository
- `NPM_TOKEN` configured in repository secrets to publish the `wabsignal` npm package
- Repository contents permission available to the workflow for GitHub Releases

## Release flow

1. Push a semantic version tag such as `v0.1.0`.
2. GitHub Actions runs [`.github/workflows/release.yml`](/D:/Working/Unsorted/grafana-query/.github/workflows/release.yml).
3. The workflow runs `go test ./...`.
4. GoReleaser builds and publishes release artifacts.
5. Cosign signs `dist/checksums.txt`.
6. The workflow prepares `packaging/npm-wabsignal-cli/package.json` with the tagged version and publishes `wabsignal` to npm when `NPM_TOKEN` is present.

## Verification

- `go test ./...`
- `go build ./...`
- `npm pack --dry-run` in [`packaging/npm-wabsignal-cli`](/D:/Working/Unsorted/grafana-query/packaging/npm-wabsignal-cli)
- Optional snapshot build: `goreleaser release --snapshot --clean`
