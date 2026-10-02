# Changelog

## [0.189.1](https://github.com/zeroroot-ai/sdk/compare/v0.189.0...v0.189.1) (2026-10-02)


### Bug Fixes

* **proto:** proto-breaking compares against the remote base, not a local branch ([#124](https://github.com/zeroroot-ai/sdk/issues/124)) ([e7a4d9d](https://github.com/zeroroot-ai/sdk/commit/e7a4d9decfce871b28cf2f3bd589757ed043eacd)), closes [#108](https://github.com/zeroroot-ai/sdk/issues/108)

## [0.189.0](https://github.com/zeroroot-ai/sdk/compare/v0.188.0...v0.189.0) (2026-10-01)


### Features

* **mission:** for_each nodes, one template run once per target ([#120](https://github.com/zeroroot-ai/sdk/issues/120)) ([015e677](https://github.com/zeroroot-ai/sdk/commit/015e677e4686ec320bef26bb74d192f44417f130))


### Bug Fixes

* **ci:** govulncheck that can read Go 1.27, in both of sdk's call sites ([#122](https://github.com/zeroroot-ai/sdk/issues/122)) ([a9430a7](https://github.com/zeroroot-ai/sdk/commit/a9430a77a2c8e57b250ff54a01010b96e4f86975))
* **cueschemas:** the freshness gate now checks freshness ([#121](https://github.com/zeroroot-ai/sdk/issues/121)) ([6db92db](https://github.com/zeroroot-ai/sdk/commit/6db92dbd3b11c111c47940c76d764f326d0ed44a))
* **eval:** the partial-score test raced the worker it was measuring ([#117](https://github.com/zeroroot-ai/sdk/issues/117)) ([2ab0329](https://github.com/zeroroot-ai/sdk/commit/2ab03291b0acb160f545bca978866260ca9ab73f))
* **target:** reserve 9, 10 and 19 — a Target names no credential ([#119](https://github.com/zeroroot-ai/sdk/issues/119)) ([0c196b6](https://github.com/zeroroot-ai/sdk/commit/0c196b64687f5ac0eb17b1152ebeea39b957bf2d))

## [0.188.0](https://github.com/zeroroot-ai/sdk/compare/v0.187.0...v0.188.0) (2026-10-01)


### Features

* **secrets:** the SDK gains SecretsService as gibson.secrets.v1 ([#116](https://github.com/zeroroot-ai/sdk/issues/116)) ([98a502a](https://github.com/zeroroot-ai/sdk/commit/98a502a43339c2c4dcff819fa7e43a00cc421af2))

## [0.187.0](https://github.com/zeroroot-ai/sdk/compare/v0.186.0...v0.187.0) (2026-10-01)


### Features

* **go:** move the toolchain floor to 1.27.1 ([#113](https://github.com/zeroroot-ai/sdk/issues/113)) ([7eba7d6](https://github.com/zeroroot-ai/sdk/commit/7eba7d698e9b75189e9c09c4d6a3b8795f15a407))

## [0.186.0](https://github.com/zeroroot-ai/sdk/compare/v0.185.0...v0.186.0) (2026-10-01)


### Features

* **target:** replace credential_id with name-shaped secret_name ([#107](https://github.com/zeroroot-ai/sdk/issues/107)) ([94c9ccb](https://github.com/zeroroot-ai/sdk/commit/94c9ccbeba65b5cc314a7484bdca71837c4041c8))

## [0.185.0](https://github.com/zeroroot-ai/sdk/compare/v0.184.0...v0.185.0) (2026-10-01)


### Features

* **identity:** add PRINCIPAL_KIND_USER for a person ([#106](https://github.com/zeroroot-ai/sdk/issues/106)) ([130510d](https://github.com/zeroroot-ai/sdk/commit/130510d684165455ce5c5b7d482646441c91d714))


### Bug Fixes

* **release:** name the repository on every publisher dispatch ([#103](https://github.com/zeroroot-ai/sdk/issues/103)) ([7980cc9](https://github.com/zeroroot-ai/sdk/commit/7980cc9cadff635578876d3e507d1980b62ddc75))

## [0.184.0](https://github.com/zeroroot-ai/sdk/compare/v0.183.1...v0.184.0) (2026-09-30)


### Features

* **daemon:** missions record who created them ([#102](https://github.com/zeroroot-ai/sdk/issues/102)) ([55d165a](https://github.com/zeroroot-ai/sdk/commit/55d165ab7fcf49b73d9ed20a7ee89758a8142af4))


### Bug Fixes

* **ci:** link-check checks only the Markdown a PR touched (.github v0.7.2) ([#100](https://github.com/zeroroot-ai/sdk/issues/100)) ([f6ed267](https://github.com/zeroroot-ai/sdk/commit/f6ed267dcd41d4d195a859c9cbfa849e83c615f0))

## [0.183.1](https://github.com/zeroroot-ai/sdk/compare/v0.183.0...v0.183.1) (2026-09-29)


### Bug Fixes

* **authz:** a Viewer reads and never changes tenant state, enforced at registry generation ([#97](https://github.com/zeroroot-ai/sdk/issues/97)) ([4bbcb6f](https://github.com/zeroroot-ai/sdk/commit/4bbcb6f11c7ab9f74b949ab8adc57e867f5a8733))

## [0.183.0](https://github.com/zeroroot-ai/sdk/compare/v0.182.0...v0.183.0) (2026-09-29)


### Features

* **harness:** add ProposeOntologyExtension RPC ([#94](https://github.com/zeroroot-ai/sdk/issues/94)) ([8b7776f](https://github.com/zeroroot-ai/sdk/commit/8b7776f38736eda473feb3682592070e6fb193a4))

## [0.182.0](https://github.com/zeroroot-ai/sdk/compare/v0.181.0...v0.182.0) (2026-09-29)


### Features

* **harness:** add SubmitProof and RequestDestructiveAuthorization RPCs ([#92](https://github.com/zeroroot-ai/sdk/issues/92)) ([ddd9457](https://github.com/zeroroot-ai/sdk/commit/ddd94571a5fa79631ba7c58f8bc039b1f5804f83))

## [0.181.0](https://github.com/zeroroot-ai/sdk/compare/v0.180.0...v0.181.0) (2026-09-28)


### Features

* **agent:** intelligence-layer SDK primitives ([#90](https://github.com/zeroroot-ai/sdk/issues/90)) ([e6af40e](https://github.com/zeroroot-ai/sdk/commit/e6af40e297db3f493f03d32e0b2f2288f3c1f3d3))

## [0.180.0](https://github.com/zeroroot-ai/sdk/compare/v0.179.1...v0.180.0) (2026-09-28)


### Features

* add hypothesis and bet primitives to the sdk ([#77](https://github.com/zeroroot-ai/sdk/issues/77)) ([aea2761](https://github.com/zeroroot-ai/sdk/commit/aea2761e1f512c22ff09713eb594d9f4dd3d9ef9))

## [0.179.1](https://github.com/zeroroot-ai/sdk/compare/v0.179.0...v0.179.1) (2026-09-21)


### Bug Fixes

* **plugin:** the heartbeat reports the lifecycle state, not a constant ([#61](https://github.com/zeroroot-ai/sdk/issues/61)) ([c8d38ed](https://github.com/zeroroot-ai/sdk/commit/c8d38ed64501f0df027030c89b003563a3e37f0b))

## [0.179.0](https://github.com/zeroroot-ai/sdk/compare/v0.178.0...v0.179.0) (2026-09-18)


### Features

* **plugin:** deliver secret events over WatchComponentEvents ([#58](https://github.com/zeroroot-ai/sdk/issues/58)) ([ae4aa3e](https://github.com/zeroroot-ai/sdk/commit/ae4aa3e800c9e4e0138fb3e5a6181b6bee065c18))

## [0.178.0](https://github.com/zeroroot-ai/sdk/compare/v0.177.4...v0.178.0) (2026-09-18)


### Features

* **proto:** add WatchComponentEvents to stream secret events to a component ([#57](https://github.com/zeroroot-ai/sdk/issues/57)) ([fb413ab](https://github.com/zeroroot-ai/sdk/commit/fb413ab675399159cc72daa3b1dcc333d8aa418e))


### Bug Fixes

* **capabilitygrant:** require https and pin endpoints to the platform origin ([#54](https://github.com/zeroroot-ai/sdk/issues/54)) ([d39bae7](https://github.com/zeroroot-ai/sdk/commit/d39bae7d10dc4ed05a0ad1d4cf30c2a8aec46b9b))
* **ci:** pin every zeroroot-ai/.github reference to v0.5.1 ([#47](https://github.com/zeroroot-ai/sdk/issues/47)) ([6d5da77](https://github.com/zeroroot-ai/sdk/commit/6d5da77101e4baa660a16ef45e18c32ffd526fec))
* **ci:** pin the org guard reusables to .github v0.4.1 ([#46](https://github.com/zeroroot-ai/sdk/issues/46)) ([022a9fa](https://github.com/zeroroot-ai/sdk/commit/022a9fa1bb9e997794866da26c5781c4fabc7537))
* **ci:** pin the org tree guards to a commit SHA ([#42](https://github.com/zeroroot-ai/sdk/issues/42)) ([3099000](https://github.com/zeroroot-ai/sdk/commit/3099000f41330ea32bf99e660088e83bc2dd26b6))
* **codegen:** keep git credentials out of shell source, argv, and config ([#53](https://github.com/zeroroot-ai/sdk/issues/53)) ([07209ea](https://github.com/zeroroot-ai/sdk/commit/07209eae918bf64d842e47819e6da5c67eeedf75))
* **codegen:** refuse editor file paths outside the workspace ([#52](https://github.com/zeroroot-ai/sdk/issues/52)) ([0b16174](https://github.com/zeroroot-ai/sdk/commit/0b16174973b77b39b64ea33aad995a0125eb36a2))
* **plugin:** refuse to start a plugin with secrets while the event stream is a stub ([#55](https://github.com/zeroroot-ai/sdk/issues/55)) ([7d361b6](https://github.com/zeroroot-ai/sdk/commit/7d361b66fcd302550bd2ce2ecb3569ede7c31128))

## [0.177.4](https://github.com/zeroroot-ai/sdk/compare/v0.177.3...v0.177.4) (2026-09-09)


### Bug Fixes

* **proto:** name repository-relative paths in published comments ([#28](https://github.com/zeroroot-ai/sdk/issues/28)) ([9cb69be](https://github.com/zeroroot-ai/sdk/commit/9cb69befeeae7a20bba010d9a8752ea833385124))

## Changelog

This repository restarted from a fresh baseline on 2026-09-06. Release notes before that date are archived offline and do not resolve on GitHub. release-please adds each release below this line.
