# iguazu-consensus

![GitHub License](https://img.shields.io/github/license/LFDT-Iguazu/iguazu-consensus)
[![OpenSSF Scorecard](https://api.scorecard.dev/projects/github.com/LFDT-Iguazu/iguazu-consensus/badge)](https://scorecard.dev/viewer/?uri=github.com/LFDT-Iguazu/iguazu-consensus)
[![OpenSSF Best Practices](https://www.bestpractices.dev/projects/16197/badge)](https://www.bestpractices.dev/projects/16197)

Apache 2.0 Go library with the Iguazu consensus logic (DAG consensus, prevote, spine, block voting, deposits, withdrawals, fork choice scoring).

Part of the Iguazu protocol separation effort: extracts the Iguazu-specific consensus code out of the Prysm-based coordinator (GPL-3.0) into a clean, reusable Apache 2.0 module. The library has no Prysm/coordinator imports and operates on abstract interfaces (`iface.BeaconState`, `iface.ExecutionEngine`, `iface.ForkChoice`, ...), so it can be consumed both in-process by the coordinator and by the `iguazu-coordinator` sidecar.

## Scope

- DAG consensus processing
- Deposit, voluntary exit, withdrawal processing
- Spine data and block voting (including SSZ roots)
- Prevote pool, aggregation, validation
- Fork choice scoring (optimistic spines)
- Consensus and prevote helpers
- Shared types and interfaces (depends on `iguazu-types`)

## Releases

Releases are published as [GitHub Releases](https://github.com/LFDT-Iguazu/iguazu-consensus/releases) with signed provenance;
see [RELEASE.md](RELEASE.md) for the process and how to verify them, and [CHANGELOG.md](CHANGELOG.md)
for the changes. Use the module with:

```shell
go get github.com/LFDT-Iguazu/iguazu-consensus@latest
```

## Documentation

API documentation is available on [pkg.go.dev](https://pkg.go.dev/github.com/LFDT-Iguazu/iguazu-consensus).

## Adopters

If you use Iguazu, please add yourself to [ADOPTERS.md](ADOPTERS.md).

## Contributing

Iguazu is an [LF Decentralized Trust](https://www.lfdecentralizedtrust.org/) project. See
[CONTRIBUTING.md](CONTRIBUTING.md) for how to contribute, [MAINTAINERS.md](MAINTAINERS.md) for
the maintainers, [SECURITY.md](SECURITY.md) for reporting vulnerabilities and
[CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md) for the Code of Conduct.

## License

Apache License 2.0 — see [LICENSE](LICENSE).

## Related repositories

- `iguazu-types` — shared types and interfaces
- `iguazu-engine` / `gwat` — execution layer
- `iguazu-coordinator` — Apache 2.0 sidecar for asynchronous Iguazu components
- `coordinator` — Prysm fork (GPL-3.0) that imports this library

See `plugin_plan/` in the parent workspace for the full separation plan.