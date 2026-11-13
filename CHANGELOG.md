# Changelog

Release notes for every version are published with its
[GitHub Release](https://github.com/LFDT-Iguazu/iguazu-consensus/releases).

## v0.2.0

### Changed

- Requires Go 1.26 (`go 1.26.0`, toolchain `go1.26.11`); modules depending on `iguazu-consensus` must use Go 1.26 or later.
- Depends on `iguazu-types` v0.4.0 instead of the retracted v0.1.0.
- Dependencies updated to versions with known security fixes: `golang.org/x/crypto` v0.59.0, `golang.org/x/sys` v0.50.0.

### Added

- GitHub Actions: CodeQL static analysis (security and quality queries).
- GitHub Actions: CI (build, vet, test), OpenSSF Scorecard and signed releases with SLSA provenance (see [RELEASE.md](RELEASE.md)); Dependabot.
- Repository files required by LF Decentralized Trust: `RELEASE.md`, `NOTICE`, `ADOPTERS.md`, issue and pull request templates.

## v0.1.0

- Initial release of `iguazu-consensus` in the LF Decentralized Trust Iguazu project.
