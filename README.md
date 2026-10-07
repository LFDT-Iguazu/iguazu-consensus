# iguazu-consensus

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