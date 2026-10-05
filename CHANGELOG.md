# Changelog

## [0.195.0](https://github.com/zeroroot-ai/sdk/compare/v0.194.0...v0.195.0) (2026-10-05)


### Features

* **proto:** the wire has a checkpoints setting and a rewind request ([#222](https://github.com/zeroroot-ai/sdk/issues/222)) ([43c9c08](https://github.com/zeroroot-ai/sdk/commit/43c9c081d952f134f2aaf004041b0d4a4b940150)), closes [#216](https://github.com/zeroroot-ai/sdk/issues/216)


### Bug Fixes

* **proto:** field 4 (remote) of the plugin registration request is reserved ([#224](https://github.com/zeroroot-ai/sdk/issues/224)) ([9ff2496](https://github.com/zeroroot-ai/sdk/commit/9ff2496edec4595bd58c472080c4651599b088ce)), closes [#181](https://github.com/zeroroot-ai/sdk/issues/181)

## [0.194.0](https://github.com/zeroroot-ai/sdk/compare/v0.193.2...v0.194.0) (2026-10-05)


### Features

* **proto:** a mission node and an origination name a start state ([#220](https://github.com/zeroroot-ai/sdk/issues/220)) ([aeff37d](https://github.com/zeroroot-ai/sdk/commit/aeff37da647443f700c95f271d855455a8229d0e))

## [0.193.2](https://github.com/zeroroot-ai/sdk/compare/v0.193.1...v0.193.2) (2026-10-05)


### Bug Fixes

* **boundary:** the boundary check reads the require list of go.mod ([#218](https://github.com/zeroroot-ai/sdk/issues/218)) ([94c7946](https://github.com/zeroroot-ai/sdk/commit/94c79463dd4e6d53fde18a58350917b18bac196e))

## [0.193.1](https://github.com/zeroroot-ai/sdk/compare/v0.193.0...v0.193.1) (2026-10-05)


### Bug Fixes

* **proto:** enroll_command is reserved, because the verb it named is gone ([#209](https://github.com/zeroroot-ai/sdk/issues/209)) ([992cd75](https://github.com/zeroroot-ai/sdk/commit/992cd754c4e3409293baa75f830ff769a1ec7eb0))
* **proto:** the proof request carries no destructive flag ([#211](https://github.com/zeroroot-ai/sdk/issues/211)) ([3361448](https://github.com/zeroroot-ai/sdk/commit/3361448572561d0a00a8502c241e1f8e656b3a2d))

## [0.193.0](https://github.com/zeroroot-ai/sdk/compare/v0.192.3...v0.193.0) (2026-10-05)


### Features

* **proto:** a proof names its recorded tool calls, and a node can be a research node ([#204](https://github.com/zeroroot-ai/sdk/issues/204)) ([d0f8aad](https://github.com/zeroroot-ai/sdk/commit/d0f8aad72cc96bb95f433e6b25e6070c4174cfdf)), closes [#203](https://github.com/zeroroot-ai/sdk/issues/203)

## [0.192.3](https://github.com/zeroroot-ai/sdk/compare/v0.192.2...v0.192.3) (2026-10-05)


### Bug Fixes

* **boundary:** the sdk imports no platform back-end client ([#201](https://github.com/zeroroot-ai/sdk/issues/201)) ([9d2df26](https://github.com/zeroroot-ai/sdk/commit/9d2df260e6e7c8b8849cb9a61b7b07665b99a341))
* **ci:** ci runs the wire contract check and the whole boundary check ([#194](https://github.com/zeroroot-ai/sdk/issues/194)) ([9a1db8f](https://github.com/zeroroot-ai/sdk/commit/9a1db8ff6656b053a3664ac9b7507d2ed6c4f5b5)), closes [#177](https://github.com/zeroroot-ai/sdk/issues/177) [#178](https://github.com/zeroroot-ai/sdk/issues/178)
* **lint:** the pagination allowlist fails on an entry that exempts nothing ([#196](https://github.com/zeroroot-ai/sdk/issues/196)) ([f94e72d](https://github.com/zeroroot-ai/sdk/commit/f94e72de23af6b7a5afd391008d999de64a38c00)), closes [#183](https://github.com/zeroroot-ai/sdk/issues/183)
* **secrets:** a late resolve caller reads the cache and does not fetch again ([#199](https://github.com/zeroroot-ai/sdk/issues/199)) ([85fa9cf](https://github.com/zeroroot-ai/sdk/commit/85fa9cf627467a757632798f2b752a1c7ac60524)), closes [#198](https://github.com/zeroroot-ai/sdk/issues/198)

## [0.192.2](https://github.com/zeroroot-ai/sdk/compare/v0.192.1...v0.192.2) (2026-10-05)


### Bug Fixes

* **authz-registry-gen:** the go registry entry drops method, which nothing read ([#169](https://github.com/zeroroot-ai/sdk/issues/169)) ([9be2cb8](https://github.com/zeroroot-ai/sdk/commit/9be2cb8a067e38691f62ee197c5acf92f3e8adb5)), closes [#145](https://github.com/zeroroot-ai/sdk/issues/145)
* **ci:** the fan-out reads no unset matrix key, and the workflows are linted ([#172](https://github.com/zeroroot-ai/sdk/issues/172)) ([99848bb](https://github.com/zeroroot-ai/sdk/commit/99848bb0a13e12d8ae2f56b0c245bcf92a86dbc8)), closes [#171](https://github.com/zeroroot-ai/sdk/issues/171)

## [0.192.1](https://github.com/zeroroot-ai/sdk/compare/v0.192.0...v0.192.1) (2026-10-04)


### Bug Fixes

* **sdk:** the unread declarations of [#111](https://github.com/zeroroot-ai/sdk/issues/111) with no reader anywhere are deleted ([27d419f](https://github.com/zeroroot-ai/sdk/commit/27d419ff562d57dd7237e90b7b150b1c7f974b0a))
* **sdk:** the unread declarations with no reader anywhere are deleted ([#168](https://github.com/zeroroot-ai/sdk/issues/168)) ([27d419f](https://github.com/zeroroot-ai/sdk/commit/27d419ff562d57dd7237e90b7b150b1c7f974b0a))
* **serve:** a tool reads no component.yaml, because the parser is gone ([#166](https://github.com/zeroroot-ai/sdk/issues/166)) ([b89a06a](https://github.com/zeroroot-ai/sdk/commit/b89a06adb1d0c65c39f0d4d3f44ec159a6f5ce06)), closes [#165](https://github.com/zeroroot-ai/sdk/issues/165)

## [0.192.0](https://github.com/zeroroot-ai/sdk/compare/v0.191.1...v0.192.0) (2026-10-04)


### Features

* **daemon:** the mission catalog is readable by a person ([#163](https://github.com/zeroroot-ai/sdk/issues/163)) ([ca28851](https://github.com/zeroroot-ai/sdk/commit/ca288512282e7f0c7ae2dc3cdd0ed51b0fda167d))

## [0.191.1](https://github.com/zeroroot-ai/sdk/compare/v0.191.0...v0.191.1) (2026-10-04)


### Bug Fixes

* **deps:** grpc 1.84.0 reopens the server-panic advisory, go back to 1.83.2 ([#159](https://github.com/zeroroot-ai/sdk/issues/159)) ([031475c](https://github.com/zeroroot-ai/sdk/commit/031475c6846ce327fcd23dea3aedf0228fe91842))

## [0.191.0](https://github.com/zeroroot-ai/sdk/compare/v0.190.0...v0.191.0) (2026-10-04)


### Features

* **mission:** a mission declares which secrets its components may receive ([#156](https://github.com/zeroroot-ai/sdk/issues/156)) ([8f2f352](https://github.com/zeroroot-ai/sdk/commit/8f2f352ad73600877cb77689c8ec212b5394503a))
* **secretenv:** one rule for the environment name a declared secret arrives in ([#158](https://github.com/zeroroot-ai/sdk/issues/158)) ([75f0bcf](https://github.com/zeroroot-ai/sdk/commit/75f0bcf2dd2252c1eafb0aa519fdd77788b143dc))

## [0.190.0](https://github.com/zeroroot-ai/sdk/compare/v0.189.1...v0.190.0) (2026-10-02)


### Features

* **plugin:** a method's description moves into Go, beside its handler ([#131](https://github.com/zeroroot-ai/sdk/issues/131)) ([1c871ff](https://github.com/zeroroot-ai/sdk/commit/1c871ff7fe3fe37ddc69bdaf5e4ff651c80c9b3b))
* **serve:** two environment variables are the whole boot path ([#144](https://github.com/zeroroot-ai/sdk/issues/144)) ([1c80526](https://github.com/zeroroot-ai/sdk/commit/1c805262a9a1b1bd71d16cae831397635defbe37)), closes [#128](https://github.com/zeroroot-ai/sdk/issues/128)
* **taxonomy-gen:** enum numbers are declared, so a type can be retired ([#137](https://github.com/zeroroot-ai/sdk/issues/137)) ([156e08a](https://github.com/zeroroot-ai/sdk/commit/156e08a00265b036d76ba6789070668d604c445d)), closes [#132](https://github.com/zeroroot-ai/sdk/issues/132) [#111](https://github.com/zeroroot-ai/sdk/issues/111)


### Bug Fixes

* **auth:** the identity freshness window is not an operator knob ([#136](https://github.com/zeroroot-ai/sdk/issues/136)) ([502c07d](https://github.com/zeroroot-ai/sdk/commit/502c07df0d35567571d98c9b57c49b87e422cf25))
* **capabilitygrant:** the no-token error never named GIBSON_BOOTSTRAP_TOKEN ([#140](https://github.com/zeroroot-ai/sdk/issues/140)) ([86881fc](https://github.com/zeroroot-ai/sdk/commit/86881fc8a1e95ef0a1e5d27757b017ea0bd79286))
* **graphrag:** compliance_signals is the seam ADR-0013 forbids leaving behind ([#133](https://github.com/zeroroot-ai/sdk/issues/133)) ([b23197d](https://github.com/zeroroot-ai/sdk/commit/b23197d4631a9eb0c0e8309e584ccad3db8e972f))
* **graphrag:** DiscoveryResult.compliance_signals is the seam ADR-0013 forbids ([b23197d](https://github.com/zeroroot-ai/sdk/commit/b23197d4631a9eb0c0e8309e584ccad3db8e972f))
* **serve:** the documented enrol variables are not the ones serve reads ([#134](https://github.com/zeroroot-ai/sdk/issues/134)) ([447bca8](https://github.com/zeroroot-ai/sdk/commit/447bca890836b41dc929e32c7e9f8ea48a869752))
* **taxonomy:** deprecate the compliance catalog, whose two consumers never existed ([#139](https://github.com/zeroroot-ai/sdk/issues/139)) ([959f519](https://github.com/zeroroot-ai/sdk/commit/959f519b4b15fff686e57ae922ce87f194a6a8b2))

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
