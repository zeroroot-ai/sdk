# Gibson SDK Changelog

## [0.177.0](https://github.com/zeroroot-ai/sdk/compare/v0.176.0...v0.177.0) (2026-09-01)


### Features

* **proto:** gibson.bank.v1 and gibson.job.v1 with harness inbox RPCs ([#550](https://github.com/zeroroot-ai/sdk/issues/550)) ([dc86ece](https://github.com/zeroroot-ai/sdk/commit/dc86ecedc828fd1fd432423287e4dd030a044f9f))
* **proto:** Task.job, the job node, the dispatch target and member status ([#552](https://github.com/zeroroot-ai/sdk/issues/552)) ([be63e32](https://github.com/zeroroot-ai/sdk/commit/be63e32ad1a137ba77dbd1465dde2c6f136799ff)), closes [#546](https://github.com/zeroroot-ai/sdk/issues/546) [#547](https://github.com/zeroroot-ai/sdk/issues/547)

## [0.176.0](https://github.com/zeroroot-ai/sdk/compare/v0.175.0...v0.176.0) (2026-08-30)


### Features

* **harness:** name a checked-in catalog mission on CreateMission ([#542](https://github.com/zeroroot-ai/sdk/issues/542)) ([7f8d857](https://github.com/zeroroot-ai/sdk/commit/7f8d8576891c0a9d2d31df6ac0ccd988c798b58b))

## [0.175.0](https://github.com/zeroroot-ai/sdk/compare/v0.174.0...v0.175.0) (2026-08-30)


### Features

* **harness:** carry an agent-written priority back on the findings read ([#540](https://github.com/zeroroot-ai/sdk/issues/540)) ([6a69805](https://github.com/zeroroot-ai/sdk/commit/6a69805104e0ae9784f07aae9373870b4c43230c))

## [0.174.0](https://github.com/zeroroot-ai/sdk/compare/v0.173.1...v0.174.0) (2026-08-30)


### Features

* **observe:** carry lifecycle entities and read what actually runs them ([#538](https://github.com/zeroroot-ai/sdk/issues/538)) ([fdf25dd](https://github.com/zeroroot-ai/sdk/commit/fdf25dd8f74dd1887d51711ceaf9d8822e6e915b)), closes [#537](https://github.com/zeroroot-ai/sdk/issues/537)

## [0.173.1](https://github.com/zeroroot-ai/sdk/compare/v0.173.0...v0.173.1) (2026-08-29)


### Bug Fixes

* **auth:** WithTenant keeps the caller identity and changes only the tenant ([#535](https://github.com/zeroroot-ai/sdk/issues/535)) ([971e7ea](https://github.com/zeroroot-ai/sdk/commit/971e7ead940dcbfcf828b85e55c5f18fed03f3fc))

## [0.173.0](https://github.com/zeroroot-ai/sdk/compare/v0.172.0...v0.173.0) (2026-08-29)


### Features

* **agentidentity:** capability_ceiling on CreateAgentIdentityRequest ([#533](https://github.com/zeroroot-ai/sdk/issues/533)) ([4157f93](https://github.com/zeroroot-ai/sdk/commit/4157f933230f26347715d890bbd2a3cbf0787270))

## [0.172.0](https://github.com/zeroroot-ai/sdk/compare/v0.171.0...v0.172.0) (2026-08-28)


### Features

* **harness:** MemoryObservation variant on ObserveRequest ([#531](https://github.com/zeroroot-ai/sdk/issues/531)) ([fb153b0](https://github.com/zeroroot-ai/sdk/commit/fb153b06e8bc7a847daaabb095ffd7d2aa814c38)), closes [#530](https://github.com/zeroroot-ai/sdk/issues/530)

## [0.171.0](https://github.com/zeroroot-ai/sdk/compare/v0.170.0...v0.171.0) (2026-08-23)


### Features

* **plugin:** send declared secrets in RegisterComponent for can_resolve binding ([#522](https://github.com/zeroroot-ai/sdk/issues/522)) ([57599be](https://github.com/zeroroot-ai/sdk/commit/57599bea650f80316e1c4cf16c7ea7699b82b5b7))

## [0.170.0](https://github.com/zeroroot-ai/sdk/compare/v0.169.0...v0.170.0) (2026-08-23)


### Features

* **component:** gate RegisterComponent + Heartbeat on can_poll_work ([#521](https://github.com/zeroroot-ai/sdk/issues/521)) ([5665780](https://github.com/zeroroot-ai/sdk/commit/5665780fdc748d019712f9db8a68345c65b6955f))


### Bug Fixes

* **deps:** mark go-jose/v4 as a direct dependency ([#519](https://github.com/zeroroot-ai/sdk/issues/519)) ([832223d](https://github.com/zeroroot-ai/sdk/commit/832223d6e1d3623075739414835bdddf443f831d))

## [0.169.0](https://github.com/zeroroot-ai/sdk/compare/v0.168.0...v0.169.0) (2026-08-23)


### Features

* **capabilitygrant:** present a SPIFFE JWT-SVID as the enrollment credential when available ([#517](https://github.com/zeroroot-ai/sdk/issues/517)) ([57a55c3](https://github.com/zeroroot-ai/sdk/commit/57a55c3ef0d7d9ddf40c37464ac87d9214edae77))

## [0.168.0](https://github.com/zeroroot-ai/sdk/compare/v0.167.0...v0.168.0) (2026-08-22)


### Features

* **plugin:** go-first authoring with derived schema; delete proto-first and mcp-bridge ([#515](https://github.com/zeroroot-ai/sdk/issues/515)) ([b88792b](https://github.com/zeroroot-ai/sdk/commit/b88792b280f87b38e57f628557be6d1fadec3138))

## [0.167.0](https://github.com/zeroroot-ai/sdk/compare/v0.166.0...v0.167.0) (2026-08-21)


### Features

* **mcpbridge:** a generic auth block so http connectors can authenticate ([#502](https://github.com/zeroroot-ai/sdk/issues/502)) ([7f627c5](https://github.com/zeroroot-ai/sdk/commit/7f627c53e44d57d02e2cf44238f0614b2ba1026a))


### Bug Fixes

* **mcpbridge:** declare the auth secret so an http connector can resolve it ([#514](https://github.com/zeroroot-ai/sdk/issues/514)) ([af7fc68](https://github.com/zeroroot-ai/sdk/commit/af7fc68296bf3fba69e59343d25d46ee6b10bb28))

## [0.166.0](https://github.com/zeroroot-ai/sdk/compare/v0.165.0...v0.166.0) (2026-08-17)


### Features

* **agent:** give Harness a KnowledgeReader group and wire it over the callback ([#498](https://github.com/zeroroot-ai/sdk/issues/498)) ([4d2f9a4](https://github.com/zeroroot-ai/sdk/commit/4d2f9a48818d6f05201494462f0074bc2050e22a))

## [0.165.0](https://github.com/zeroroot-ai/sdk/compare/v0.164.0...v0.165.0) (2026-08-17)


### ⚠ BREAKING CHANGES

* **harness:** FindSimilarAttacksResponse, GetAttackChainsResponse, FindSimilarFindingsResponse, GetRelatedFindingsResponse and GetFindingsResponse change field 1 from bytes to a repeated message. These RPCs were added hours ago and nothing consumes them, so this is a clean break rather than a dead results_json parked at field 1 forever.

### Features

* **harness:** type the knowledge payloads and add the two missing reads ([#496](https://github.com/zeroroot-ai/sdk/issues/496)) ([dcd3ec6](https://github.com/zeroroot-ai/sdk/commit/dcd3ec68a9be694191e5b9f522421b700da2a3c7)), closes [#491](https://github.com/zeroroot-ai/sdk/issues/491)

## [0.164.0](https://github.com/zeroroot-ai/sdk/compare/v0.163.1...v0.164.0) (2026-08-17)


### Features

* **harness:** carry the knowledge-graph reads on HarnessCallbackService ([#486](https://github.com/zeroroot-ai/sdk/issues/486)) ([d015560](https://github.com/zeroroot-ai/sdk/commit/d015560f15d90620742d0670aeef69298093057d))

## [0.163.1](https://github.com/zeroroot-ai/sdk/compare/v0.163.0...v0.163.1) (2026-08-16)


### Bug Fixes

* **ci:** check the BSR credential weekly so buf-push cannot rot silently ([#481](https://github.com/zeroroot-ai/sdk/issues/481)) ([1e9dc9e](https://github.com/zeroroot-ai/sdk/commit/1e9dc9ece00822426824788d81ed9c92170cc7b3))

## [0.163.0](https://github.com/zeroroot-ai/sdk/compare/v0.162.0...v0.163.0) (2026-08-15)


### Features

* **ci:** notify docs-site to refresh api-spec on release ([#474](https://github.com/zeroroot-ai/sdk/issues/474)) ([87fe8f1](https://github.com/zeroroot-ai/sdk/commit/87fe8f117d7e0b5258556b6ac8305374b474d77f))
* **component:** let LLMMessage carry tool-call history ([#473](https://github.com/zeroroot-ai/sdk/issues/473)) ([1f7c0c6](https://github.com/zeroroot-ai/sdk/commit/1f7c0c6212501faaeb69f29b3fbc0c2781037511))


### Bug Fixes

* **ci:** file a tracked issue when buf-push fails ([#462](https://github.com/zeroroot-ai/sdk/issues/462)) ([09eccea](https://github.com/zeroroot-ai/sdk/commit/09ecceacfab6a06458e76e90ae51278648f9bf8f))
* **ci:** move CodeQL off merge_group — SARIF upload always fails there ([#479](https://github.com/zeroroot-ai/sdk/issues/479)) ([4996c09](https://github.com/zeroroot-ai/sdk/commit/4996c0974ae65799faa4e27a258ff75d235d1490))
* **ci:** pin GitHub Actions to SHAs, close workflow-permissions alerts ([#478](https://github.com/zeroroot-ai/sdk/issues/478)) ([f5dd516](https://github.com/zeroroot-ai/sdk/commit/f5dd516d094de0d5c2a84fafd86af2d83c7d4dde))

## [0.162.0](https://github.com/zeroroot-ai/sdk/compare/v0.161.0...v0.162.0) (2026-08-13)


### Features

* **harness:** session Devbox exec + session-context store RPCs ([#460](https://github.com/zeroroot-ai/sdk/issues/460)) ([b8899a8](https://github.com/zeroroot-ai/sdk/commit/b8899a8020ed0f6847c3667d5f6058635ddd3908))

## [0.161.0](https://github.com/zeroroot-ai/sdk/compare/v0.160.0...v0.161.0) (2026-08-11)


### Features

* **harness:** add the agent-facing WorldView read half of the emit-only contract ([#457](https://github.com/zeroroot-ai/sdk/issues/457)) ([ee966cf](https://github.com/zeroroot-ai/sdk/commit/ee966cf3925d6423d28396ec6664d09a7473d6d7)), closes [#341](https://github.com/zeroroot-ai/sdk/issues/341)

## [0.160.0](https://github.com/zeroroot-ai/sdk/compare/v0.159.0...v0.160.0) (2026-08-09)


### Features

* **capabilitygrant:** bind agent+jwt to its gRPC method and a stable audience ([#456](https://github.com/zeroroot-ai/sdk/issues/456)) ([0e93fdc](https://github.com/zeroroot-ai/sdk/commit/0e93fdce4813bb6d543b69c639747541c436485e))


### Bug Fixes

* **plugin:** resolve daemon address from the platform URL port instead of hardcoding :50051 ([#453](https://github.com/zeroroot-ai/sdk/issues/453)) ([382504f](https://github.com/zeroroot-ai/sdk/commit/382504f3a690c17acfcfeb00e7ee92850840b67f)), closes [#452](https://github.com/zeroroot-ai/sdk/issues/452)

## [0.159.0](https://github.com/zeroroot-ai/sdk/compare/v0.158.0...v0.159.0) (2026-08-08)


### ⚠ BREAKING CHANGES

* remove the ComponentService StoreNode RPC ([#451](https://github.com/zeroroot-ai/sdk/issues/451))

### Features

* remove the ComponentService StoreNode RPC ([#451](https://github.com/zeroroot-ai/sdk/issues/451)) ([09395d2](https://github.com/zeroroot-ai/sdk/commit/09395d28e2ab9a6030d67b4ab0dc791284c82f72))


### Bug Fixes

* **ci:** make buf a Go tool dependency — hermetic go test, drop npm-buf ([#420](https://github.com/zeroroot-ai/sdk/issues/420) root cause) ([#421](https://github.com/zeroroot-ai/sdk/issues/421)) ([99905ce](https://github.com/zeroroot-ai/sdk/commit/99905ce599b3eb52fca9d4eb2e1375f175821e7e))
* **codegen/lsp:** switch diagnostic severity tally on codegen enum members ([#424](https://github.com/zeroroot-ai/sdk/issues/424)) ([2700546](https://github.com/zeroroot-ai/sdk/commit/27005469e718416b22117362d9542eb01a0a20ed)), closes [#404](https://github.com/zeroroot-ai/sdk/issues/404)
* **fan-out:** correct stale consumer matrix (gibson-executor rename, adk path, drop absorbed repos) ([#411](https://github.com/zeroroot-ai/sdk/issues/411)) ([e34b8e1](https://github.com/zeroroot-ai/sdk/commit/e34b8e1c0659fae093074ae95f68959733f39906))
* pipe authz-registry-gen Go output through go/format ([#423](https://github.com/zeroroot-ai/sdk/issues/423)) ([988ecc7](https://github.com/zeroroot-ai/sdk/commit/988ecc737191c2fd86c232bc002eac76bb4d48b1)), closes [#414](https://github.com/zeroroot-ai/sdk/issues/414)

## [0.158.0](https://github.com/zeroroot-ai/sdk/compare/v0.157.1...v0.158.0) (2026-06-29)


### Features

* **identity:** add can_revoke_sessions to WhoAmIResponse (gibson[#628](https://github.com/zeroroot-ai/sdk/issues/628)) ([#407](https://github.com/zeroroot-ai/sdk/issues/407)) ([3bdc058](https://github.com/zeroroot-ai/sdk/commit/3bdc058366a785802a920d242b20b56ddc7d9716))

## [0.157.1](https://github.com/zeroroot-ai/sdk/compare/v0.157.0...v0.157.1) (2026-06-26)


### Bug Fixes

* **plugin:** forward the plugin:* registration metadata keys the daemon reads (gibson[#997](https://github.com/zeroroot-ai/sdk/issues/997)) ([#399](https://github.com/zeroroot-ai/sdk/issues/399)) ([d141e3c](https://github.com/zeroroot-ai/sdk/commit/d141e3c2d3d11724ce8ff1c108373174ce1050d9))

## [0.157.0](https://github.com/zeroroot-ai/sdk/compare/v0.156.0...v0.157.0) (2026-06-26)


### Features

* **plugin:** manifest content_trust + forward to daemon at register (gibson[#997](https://github.com/zeroroot-ai/sdk/issues/997)) ([#397](https://github.com/zeroroot-ai/sdk/issues/397)) ([92d6e24](https://github.com/zeroroot-ai/sdk/commit/92d6e2446fdf4d4f609a694ffd0a6c3e37776be9))

## [0.156.0](https://github.com/zeroroot-ai/sdk/compare/v0.155.0...v0.156.0) (2026-06-25)


### Features

* **capability:** add IsolationMode enum + isolation field (gibson[#998](https://github.com/zeroroot-ai/sdk/issues/998)) ([#392](https://github.com/zeroroot-ai/sdk/issues/392)) ([71bbf75](https://github.com/zeroroot-ai/sdk/commit/71bbf752d4e5250023542a38419cd7b65c6bfbfd))

## [0.155.0](https://github.com/zeroroot-ai/sdk/compare/v0.154.0...v0.155.0) (2026-06-23)


### ⚠ BREAKING CHANGES

* **proto:** narrow the SDK — re-home enrollment protos, drop 9 admin services ([#390](https://github.com/zeroroot-ai/sdk/issues/390))

### Features

* pin Go 1.26.4 toolchain, uniform make contract, blocking lint + digest-pinned mirror images ([#386](https://github.com/zeroroot-ai/sdk/issues/386)) ([51328bd](https://github.com/zeroroot-ai/sdk/commit/51328bdfa6641da3d0e2aa9e2f325f3d8f3462ea))
* **proto:** freeze narrowed SDK on buf breaking WIRE ([#391](https://github.com/zeroroot-ai/sdk/issues/391)) ([b751898](https://github.com/zeroroot-ai/sdk/commit/b751898098fe723e506869eabee23bf97bd0234b))
* **proto:** narrow the SDK — re-home enrollment protos, drop 9 admin services ([#390](https://github.com/zeroroot-ai/sdk/issues/390)) ([7e46de0](https://github.com/zeroroot-ai/sdk/commit/7e46de06c1be6b0952486434ccb1fa61b3adb7be))

## [0.154.0](https://github.com/zeroroot-ai/sdk/compare/v0.153.0...v0.154.0) (2026-06-21)


### ⚠ BREAKING CHANGES

* **tenant:** ProviderService no longer exposes {Get,Set,Delete}TenantLangfuseCredentials. The feature is retired; no customer integration uses these owner-relation admin RPCs.
* **component:** remove the memory RPCs from ComponentService (gibson#756) ([#380](https://github.com/zeroroot-ai/sdk/issues/380))

### Features

* **component:** remove the memory RPCs from ComponentService (gibson[#756](https://github.com/zeroroot-ai/sdk/issues/756)) ([#380](https://github.com/zeroroot-ai/sdk/issues/380)) ([5268430](https://github.com/zeroroot-ai/sdk/commit/5268430ef6fbf450a1ee400e2d70e67e25f1700c))
* **tenant:** remove the *TenantLangfuseCredentials provider RPCs ([#382](https://github.com/zeroroot-ai/sdk/issues/382)) ([a50e0b8](https://github.com/zeroroot-ai/sdk/commit/a50e0b8ff765fb944b2df8cd9b1b775939960d1c))

## [0.153.0](https://github.com/zeroroot-ai/sdk/compare/v0.152.0...v0.153.0) (2026-06-20)


### Features

* **mission:** add optional decider_slot to MissionDefinition ([#850](https://github.com/zeroroot-ai/sdk/issues/850)) ([#377](https://github.com/zeroroot-ai/sdk/issues/377)) ([74eb4cb](https://github.com/zeroroot-ai/sdk/commit/74eb4cb7f637e6c1021f111fa34f1e8e5512d34d))

## [0.152.0](https://github.com/zeroroot-ai/sdk/compare/v0.151.0...v0.152.0) (2026-06-19)


### ⚠ BREAKING CHANGES

* remove the orphaned intelligence/v1 proto ([#819](https://github.com/zeroroot-ai/sdk/issues/819)) (#375)

### Features

* remove the orphaned intelligence/v1 proto ([#819](https://github.com/zeroroot-ai/sdk/issues/819)) ([#375](https://github.com/zeroroot-ai/sdk/issues/375)) ([9019fb6](https://github.com/zeroroot-ai/sdk/commit/9019fb63970ea3cb3b429bbf90eba18bab7c6b94))

## [0.151.0](https://github.com/zeroroot-ai/sdk/compare/v0.150.0...v0.151.0) (2026-06-19)


### ⚠ BREAKING CHANGES

* rip StoreNode direct-emit surface — Observe is the only emit ([#775](https://github.com/zeroroot-ai/sdk/issues/775)) (#373)

### Features

* rip StoreNode direct-emit surface — Observe is the only emit ([#775](https://github.com/zeroroot-ai/sdk/issues/775)) ([#373](https://github.com/zeroroot-ai/sdk/issues/373)) ([7025ab9](https://github.com/zeroroot-ai/sdk/commit/7025ab9eab88c7afd867b6432b63f21bd992bde7))

## [0.150.0](https://github.com/zeroroot-ai/sdk/compare/v0.149.0...v0.150.0) (2026-06-19)


### Features

* **agent:** credential/account typed observations ([#825](https://github.com/zeroroot-ai/sdk/issues/825)) ([#371](https://github.com/zeroroot-ai/sdk/issues/371)) ([1d94d02](https://github.com/zeroroot-ai/sdk/commit/1d94d02fb6c2dd0e0a54040e36ba4d5266382a07))

## [0.149.0](https://github.com/zeroroot-ai/sdk/compare/v0.148.0...v0.149.0) (2026-06-19)


### Features

* **agent:** endpoint/technology/certificate service-detail observations ([#824](https://github.com/zeroroot-ai/sdk/issues/824)) ([#369](https://github.com/zeroroot-ai/sdk/issues/369)) ([5ad5600](https://github.com/zeroroot-ai/sdk/commit/5ad5600e28125e6722d4747e6f5b769c7cd44138))

## [0.148.0](https://github.com/zeroroot-ai/sdk/compare/v0.147.0...v0.148.0) (2026-06-19)


### Features

* **agent:** domain/subdomain typed observations ([#823](https://github.com/zeroroot-ai/sdk/issues/823)) ([#367](https://github.com/zeroroot-ai/sdk/issues/367)) ([ba496f3](https://github.com/zeroroot-ai/sdk/commit/ba496f38e55917db28fbe5e77c99cc44079d377b))

## [0.147.0](https://github.com/zeroroot-ai/sdk/compare/v0.146.0...v0.147.0) (2026-06-19)


### Features

* **agent:** typed Observe(Observation) emit surface ([#773](https://github.com/zeroroot-ai/sdk/issues/773)) ([#365](https://github.com/zeroroot-ai/sdk/issues/365)) ([1e55720](https://github.com/zeroroot-ai/sdk/commit/1e557204087ee8f579525aff5f42a32f1fd95f16))

## [0.146.0](https://github.com/zeroroot-ai/sdk/compare/v0.145.0...v0.146.0) (2026-06-19)


### ⚠ BREAKING CHANGES

* delete graphrag query-types (emit-only graphrag) ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) (#354)
* remove recall/memory/query RPCs from harness-callback proto ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) (#352)

### Features

* delete graphrag query-types (emit-only graphrag) ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) ([#354](https://github.com/zeroroot-ai/sdk/issues/354)) ([15a1b23](https://github.com/zeroroot-ai/sdk/commit/15a1b2342510e1937a6bd345e16a43333957c1a8))
* remove recall/memory/query RPCs from harness-callback proto ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) ([#352](https://github.com/zeroroot-ai/sdk/issues/352)) ([f86d73c](https://github.com/zeroroot-ai/sdk/commit/f86d73cfe3ecefb8358d41e33b81c3473b8db222))

## [0.145.0](https://github.com/zeroroot-ai/sdk/compare/v0.144.0...v0.145.0) (2026-06-19)


### ⚠ BREAKING CHANGES

* rip recall Go API from serve harnesses + delete sdk/memory ([#350](https://github.com/zeroroot-ai/sdk/issues/350)) (#351)
* make agent.Harness an emit-only worker contract ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) (#349)

### Features

* make agent.Harness an emit-only worker contract ([#341](https://github.com/zeroroot-ai/sdk/issues/341)) ([#349](https://github.com/zeroroot-ai/sdk/issues/349)) ([5dab39d](https://github.com/zeroroot-ai/sdk/commit/5dab39d68c2c885706fa9e1df2e5cc94d6821bbf))
* rip recall Go API from serve harnesses + delete sdk/memory ([#350](https://github.com/zeroroot-ai/sdk/issues/350)) ([#351](https://github.com/zeroroot-ai/sdk/issues/351)) ([b775f59](https://github.com/zeroroot-ai/sdk/commit/b775f597b212c0e0280e46a7657ed84fe7b7eaf7))
* **taxonomy:** add scope/credential/account node types + volatile marker ([#347](https://github.com/zeroroot-ai/sdk/issues/347)) ([fd3b040](https://github.com/zeroroot-ai/sdk/commit/fd3b04039ef6d3bbd1f3660d8a7d44ac9626f696))

## [0.144.0](https://github.com/zeroroot-ai/sdk/compare/v0.143.0...v0.144.0) (2026-06-12)


### ⚠ BREAKING CHANGES

* **connector:** the connector.gibson.zeroroot.ai/v1 manifest schema and the github.com/zeroroot-ai/sdk/mcpbridge/connector package are removed. A connector is authored as a plugin.gibson.zeroroot.ai/v1 manifest with runtime: mcp-bridge and a spec.mcp_bridge block.

### Miscellaneous Chores

* **connector:** retire the connector.gibson.zeroroot.ai/v1 lineage; the mcp-bridge plugin builds from the plugin manifest ([#338](https://github.com/zeroroot-ai/sdk/issues/338)) ([82431bd](https://github.com/zeroroot-ai/sdk/commit/82431bd203d87326cc397cb9997f2ecb1f18acc2))

## [0.143.0](https://github.com/zeroroot-ai/sdk/compare/v0.142.0...v0.143.0) (2026-06-12)


### Features

* **manifest:** add runtime: mcp-bridge + spec.mcp_bridge block to the plugin manifest ([#336](https://github.com/zeroroot-ai/sdk/issues/336)) ([5ff9121](https://github.com/zeroroot-ai/sdk/commit/5ff9121f3c384e6dbb08efeb0b04722e04697eb5))

## [0.142.0](https://github.com/zeroroot-ai/sdk/compare/v0.141.0...v0.142.0) (2026-06-12)


### Features

* support TLS dial + authority override for the daemon gRPC connection ([#329](https://github.com/zeroroot-ai/sdk/issues/329)) ([07cd6be](https://github.com/zeroroot-ai/sdk/commit/07cd6be871241890245e5869d7f0c68ca5ad29ad))
* **tenant:** add MembershipService.SetCatalogPublished for BYO connectors ([#683](https://github.com/zeroroot-ai/sdk/issues/683)) ([#333](https://github.com/zeroroot-ai/sdk/issues/333)) ([d8c796a](https://github.com/zeroroot-ai/sdk/commit/d8c796a568445016f83a130f338189740f1bd04e))


### Bug Fixes

* honor GIBSON_BOOTSTRAP_TOKEN and ported platform URLs in plugin bootstrap ([#327](https://github.com/zeroroot-ai/sdk/issues/327)) ([d97f181](https://github.com/zeroroot-ai/sdk/commit/d97f181e2715025e14de20c6df91ba6c25da1dce)), closes [#325](https://github.com/zeroroot-ai/sdk/issues/325) [#326](https://github.com/zeroroot-ai/sdk/issues/326)

## [0.141.0](https://github.com/zeroroot-ai/sdk/compare/v0.140.0...v0.141.0) (2026-06-09)


### Features

* **tenant:** add RegisterPluginRequest.remote for customer-network connectors ([#323](https://github.com/zeroroot-ai/sdk/issues/323)) ([f3814ae](https://github.com/zeroroot-ai/sdk/commit/f3814aecdccfb334a7941c0612fcc70ce8519711))

## [0.140.0](https://github.com/zeroroot-ai/sdk/compare/v0.139.0...v0.140.0) (2026-06-09)


### Features

* **harness:** add SearchTools RPC for agent tool discovery ([#321](https://github.com/zeroroot-ai/sdk/issues/321)) ([c35f5b9](https://github.com/zeroroot-ai/sdk/commit/c35f5b939cd41fa49e9ff4fde152d9349cacc49c))

## [0.139.0](https://github.com/zeroroot-ai/sdk/compare/v0.138.0...v0.139.0) (2026-06-09)


### Features

* **component:** carry per-method descriptions through RegisterComponent ([#318](https://github.com/zeroroot-ai/sdk/issues/318)) ([7fbfba9](https://github.com/zeroroot-ai/sdk/commit/7fbfba9dca62675bfdecc7fb9c266af6405d1c8a))
* **mcpbridge:** add egress declarations to the connector manifest ([#319](https://github.com/zeroroot-ai/sdk/issues/319)) ([5a5541d](https://github.com/zeroroot-ai/sdk/commit/5a5541d88d0e941a4c29b5aae8959b15e6ae71c7))

## [0.138.0](https://github.com/zeroroot-ai/sdk/compare/v0.137.0...v0.138.0) (2026-06-09)


### ⚠ BREAKING CHANGES

* **tenant:** CreateAgentIdentityResponse no longer carries client_id / client_secret. Consumers use bootstrap_token (gibson, adk, dashboard already updated; the SDK fan-out bumps land clean).

### Features

* **plugin:** add MCP-bridge plugin with dynamic method registration ([#317](https://github.com/zeroroot-ai/sdk/issues/317)) ([e0833f6](https://github.com/zeroroot-ai/sdk/commit/e0833f68e4b1604707a3222f4db367ed88ed1f7d)), closes [#316](https://github.com/zeroroot-ai/sdk/issues/316)
* **tenant:** remove vestigial OAuth client_id/client_secret from CreateAgentIdentityResponse ([#309](https://github.com/zeroroot-ai/sdk/issues/309)) ([16a964f](https://github.com/zeroroot-ai/sdk/commit/16a964f9015311c18a0e4791921a108ee3654297))


### Bug Fixes

* **lint:** correct grandfather paths and allow-list wire-locked UserService List RPCs ([#315](https://github.com/zeroroot-ai/sdk/issues/315)) ([e01b2cd](https://github.com/zeroroot-ai/sdk/commit/e01b2cd11f9ab44cea20a61fc5ec1e7605f45d14)), closes [#250](https://github.com/zeroroot-ai/sdk/issues/250)

## [0.137.0](https://github.com/zeroroot-ai/sdk/compare/v0.136.0...v0.137.0) (2026-06-05)


### Features

* **agent:** authenticate Connect with the capability-grant JWT (sdk[#302](https://github.com/zeroroot-ai/sdk/issues/302)) ([#305](https://github.com/zeroroot-ai/sdk/issues/305)) ([655fa1c](https://github.com/zeroroot-ai/sdk/commit/655fa1cd0cbd2dc5ef6f07514c7718f486c3f0bd))


### Bug Fixes

* **fan-out:** close superseded bump PRs + loud auto-merge arming (sdk[#301](https://github.com/zeroroot-ai/sdk/issues/301)) ([#303](https://github.com/zeroroot-ai/sdk/issues/303)) ([08f3293](https://github.com/zeroroot-ai/sdk/commit/08f3293c000ba1fb18bf7327fe8883ce832070f6))

## [0.136.0](https://github.com/zeroroot-ai/sdk/compare/v0.135.0...v0.136.0) (2026-06-05)


### Features

* **capabilitygrant:** persistable runtime credential + dedicated CG header (sdk[#292](https://github.com/zeroroot-ai/sdk/issues/292)) ([#300](https://github.com/zeroroot-ai/sdk/issues/300)) ([fd18dd4](https://github.com/zeroroot-ai/sdk/commit/fd18dd4cafcfeecb2597398869245db52acd923f))


### Bug Fixes

* **capabilitygrant:** set kid=agentID on the per-RPC agent+jwt (gibson[#648](https://github.com/zeroroot-ai/sdk/issues/648)) ([#298](https://github.com/zeroroot-ai/sdk/issues/298)) ([b354c91](https://github.com/zeroroot-ai/sdk/commit/b354c9127da3c104c7434d2d327574ec769f6b0b))

## [0.135.0](https://github.com/zeroroot-ai/sdk/compare/v0.134.0...v0.135.0) (2026-06-05)


### Features

* **tenant:** add bootstrap_token to CreateAgentIdentityResponse (gibson[#648](https://github.com/zeroroot-ai/sdk/issues/648)) ([#296](https://github.com/zeroroot-ai/sdk/issues/296)) ([a04cbbc](https://github.com/zeroroot-ai/sdk/commit/a04cbbc8263cc46f41e5cf8eb8e28eb7e2291e00))

## [0.134.0](https://github.com/zeroroot-ai/sdk/compare/v0.133.0...v0.134.0) (2026-06-04)


### Features

* **tenant:** add MembershipService invitation RPCs + TenantMember.status (gibson[#626](https://github.com/zeroroot-ai/sdk/issues/626)) ([#290](https://github.com/zeroroot-ai/sdk/issues/290)) ([ed207e1](https://github.com/zeroroot-ai/sdk/commit/ed207e1884c492252355e7d0b9e869c9305ac835))

## [0.133.0](https://github.com/zeroroot-ai/sdk/compare/v0.132.0...v0.133.0) (2026-06-03)


### Features

* **tenant:** add UserService.RevokeUserSessions RPC for session/token revocation ([#289](https://github.com/zeroroot-ai/sdk/issues/289)) ([e57f3cf](https://github.com/zeroroot-ai/sdk/commit/e57f3cf0208cfcbe23158d595a712d82f2d7c0da))


### Bug Fixes

* **ci:** bump go toolchain to 1.25.11 ([#285](https://github.com/zeroroot-ai/sdk/issues/285)) ([8a0c61e](https://github.com/zeroroot-ai/sdk/commit/8a0c61e13ade85362bc73a1f2065947961557f68))
* **mission-authoring:** regenerate glossary for MissionGraph/MissionLayout ([#288](https://github.com/zeroroot-ai/sdk/issues/288)) ([dc53d10](https://github.com/zeroroot-ai/sdk/commit/dc53d10774f6fa8f6e238084fbe5526ffdb513ac)), closes [#287](https://github.com/zeroroot-ai/sdk/issues/287)

## [0.132.0](https://github.com/zeroroot-ai/sdk/compare/v0.131.1...v0.132.0) (2026-06-03)


### Features

* add MissionGraph + layout-store RPCs to DaemonService ([#283](https://github.com/zeroroot-ai/sdk/issues/283)) ([7e5998d](https://github.com/zeroroot-ai/sdk/commit/7e5998d90c567ce48dcafc70a3c26556869c14e1))

## [0.131.1](https://github.com/zeroroot-ai/sdk/compare/v0.131.0...v0.131.1) (2026-06-02)


### Bug Fixes

* **mission:** regenerate CUE schema to include AgentNodeConfig.llmSlots ([#276](https://github.com/zeroroot-ai/sdk/issues/276)) ([6c8316f](https://github.com/zeroroot-ai/sdk/commit/6c8316f4f5e971e852479349035498b028d8d1b5))

## [0.131.0](https://github.com/zeroroot-ai/sdk/compare/v0.130.0...v0.131.0) (2026-06-02)


### Features

* **plugin:** handler-accessible secret resolve API (closes [#263](https://github.com/zeroroot-ai/sdk/issues/263)) ([#273](https://github.com/zeroroot-ai/sdk/issues/273)) ([8e2f910](https://github.com/zeroroot-ai/sdk/commit/8e2f910177adc81df4059895b9ee32b72b7ee1d9))


### Bug Fixes

* **authz:** SetSignupProgress is unauthenticated (pre-tenant) (dashboard[#646](https://github.com/zeroroot-ai/sdk/issues/646)) ([#275](https://github.com/zeroroot-ai/sdk/issues/275)) ([28a1d9c](https://github.com/zeroroot-ai/sdk/commit/28a1d9c3fd2d2ebe4c61bb78b09a02905b2202a0))

## [0.130.0](https://github.com/zeroroot-ai/sdk/compare/v0.129.1...v0.130.0) (2026-06-01)


### Features

* **tenant:** add SetCatalogEnabled RPC to MembershipService (ADR-0041) ([#271](https://github.com/zeroroot-ai/sdk/issues/271)) ([3676532](https://github.com/zeroroot-ai/sdk/commit/36765328d24d322726d8e3d3a4b9a702cc03c870))

## [0.129.1](https://github.com/zeroroot-ai/sdk/compare/v0.129.0...v0.129.1) (2026-06-01)


### Bug Fixes

* **build:** commit generated bindings for decomposed tenant services (ADR-0039) ([#268](https://github.com/zeroroot-ai/sdk/issues/268)) ([7c34d48](https://github.com/zeroroot-ai/sdk/commit/7c34d48a5a74ecaad705106a00fb7e8501914a2c))

## [0.129.0](https://github.com/zeroroot-ai/sdk/compare/v0.128.0...v0.129.0) (2026-06-01)


### ⚠ BREAKING CHANGES

* **tenant:** decompose tenant administration into focused customer-facing services (ADR-0039) ([#267](https://github.com/zeroroot-ai/sdk/issues/267))

### Features

* **tenant:** decompose tenant administration into focused customer-facing services (ADR-0039) ([#267](https://github.com/zeroroot-ai/sdk/issues/267)) ([7288f4d](https://github.com/zeroroot-ai/sdk/commit/7288f4dbab0b1a2101fa1eb32ec52749c6bfabc1))


### Bug Fixes

* **authz:** default-deny the no-op Authorizer when none is in context (closes [#264](https://github.com/zeroroot-ai/sdk/issues/264)) ([#265](https://github.com/zeroroot-ai/sdk/issues/265)) ([d319410](https://github.com/zeroroot-ai/sdk/commit/d31941046af534d6380efe15e26a1a17ed12f9c3))

## [0.128.0](https://github.com/zeroroot-ai/sdk/compare/v0.127.0...v0.128.0) (2026-05-31)


### Features

* **mission:** multi-slot LLM binding contract on AgentNodeConfig ([#261](https://github.com/zeroroot-ai/sdk/issues/261)) ([d92dad2](https://github.com/zeroroot-ai/sdk/commit/d92dad27f904987f591b48d964b1fd9c8fb27eb7)), closes [#260](https://github.com/zeroroot-ai/sdk/issues/260)

## [0.127.0](https://github.com/zeroroot-ai/sdk/compare/v0.126.0...v0.127.0) (2026-05-30)


### Features

* **mission:** LLMSlotConfig — per-node LLM provider/model binding ([#258](https://github.com/zeroroot-ai/sdk/issues/258)) ([2ecdfee](https://github.com/zeroroot-ai/sdk/commit/2ecdfeec44721717da84aa1039fbe4614456e0b9))

## [0.126.0](https://github.com/zeroroot-ai/sdk/compare/v0.125.0...v0.126.0) (2026-05-29)


### Features

* **target:** gibson.target.v1 + CreateTarget/Get/List/Update/Delete on DaemonService ([#256](https://github.com/zeroroot-ai/sdk/issues/256)) ([9033279](https://github.com/zeroroot-ai/sdk/commit/9033279543675f8d55f81738c071f57099043248))


### Bug Fixes

* **lint:** grandfather ListAuditEvents in lint-pagination ([#253](https://github.com/zeroroot-ai/sdk/issues/253)) ([d4853b1](https://github.com/zeroroot-ai/sdk/commit/d4853b1d229d3565f5e27142c9ca07809d81f42b)), closes [#250](https://github.com/zeroroot-ai/sdk/issues/250)

## [0.125.0](https://github.com/zeroroot-ai/sdk/compare/v0.124.2...v0.125.0) (2026-05-29)


### Features

* **mission:** add cue_source + mission_definition_id proto fields ([#251](https://github.com/zeroroot-ai/sdk/issues/251)) ([467e8d0](https://github.com/zeroroot-ai/sdk/commit/467e8d046596fc0304b9bef44379f26789cf3d0d)), closes [#249](https://github.com/zeroroot-ai/sdk/issues/249)

## [0.124.2](https://github.com/zeroroot-ai/sdk/compare/v0.124.1...v0.124.2) (2026-05-27)


### Bug Fixes

* GetTenantQuota annotation member→admin, add plan_id=13 to response ([#240](https://github.com/zeroroot-ai/sdk/issues/240)) ([06d14a9](https://github.com/zeroroot-ai/sdk/commit/06d14a96e173dfd75547d28563b096aeeeafda1c)), closes [#238](https://github.com/zeroroot-ai/sdk/issues/238) [#239](https://github.com/zeroroot-ai/sdk/issues/239)
* update examples/custom-tool to sdk v0.124.0 (fixes govulncheck) ([#236](https://github.com/zeroroot-ai/sdk/issues/236)) ([7591b87](https://github.com/zeroroot-ai/sdk/commit/7591b87ea0377e80f1adbe4321808f8e56534429))

## [0.124.1](https://github.com/zeroroot-ai/sdk/compare/v0.124.0...v0.124.1) (2026-05-26)


### Bug Fixes

* **cueschemas:** guard against stale zero-day-ai module paths in embedded CUE ([#232](https://github.com/zeroroot-ai/sdk/issues/232)) ([f6fbaa2](https://github.com/zeroroot-ai/sdk/commit/f6fbaa243b91eedd1981d25df032c160a530778c))
* update mission schema \$id domain to zeroroot.ai ([#233](https://github.com/zeroroot-ai/sdk/issues/233)) ([6e6d6ca](https://github.com/zeroroot-ai/sdk/commit/6e6d6cad5799e9278c51a587207b0dc6b0d1569b))

## [0.123.0](https://github.com/zeroroot-ai/sdk/compare/v0.122.0...v0.123.0) (2026-05-26)


### Features

* **daemon:** add UpdateMissionDefinition RPC to DaemonService ([#221](https://github.com/zeroroot-ai/sdk/issues/221)) ([82bf08c](https://github.com/zeroroot-ai/sdk/commit/82bf08c82ad15d8fdfd645c0e9ac782524d7c714))


### Bug Fixes

* **daemonclient:** add UpdateMissionDefinition stub to mocks + client method ([#223](https://github.com/zeroroot-ai/sdk/issues/223)) ([7e7c75c](https://github.com/zeroroot-ai/sdk/commit/7e7c75c1863784140bde24906c07302fd54b7059))

## [0.122.0](https://github.com/zeroroot-ai/sdk/compare/v0.121.0...v0.122.0) (2026-05-26)


### ⚠ BREAKING CHANGES

* GetFallbackChain and SetFallbackChain are no longer present on TenantService. Consumers that called these RPCs (dashboard fallback-chain route, gibson daemon handlers) are being removed in the same wave.

### Features

* remove GetFallbackChain and SetFallbackChain from TenantService ([#218](https://github.com/zeroroot-ai/sdk/issues/218)) ([80f0e1e](https://github.com/zeroroot-ai/sdk/commit/80f0e1e051f90147b1482e76fd3e7defb620b471))

## [0.121.0](https://github.com/zeroroot-ai/sdk/compare/v0.120.0...v0.121.0) (2026-05-25)


### Features

* **cue:** return compiled MissionDefinition from ValidateMissionCUE ([#215](https://github.com/zeroroot-ai/sdk/issues/215)) ([e71a43a](https://github.com/zeroroot-ai/sdk/commit/e71a43a73d3f5a60329b8268ee9e9114448355fe))

## [0.120.0](https://github.com/zeroroot-ai/sdk/compare/v0.119.0...v0.120.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* remove GetMissionSourceYAML from DaemonService ([#213](https://github.com/zeroroot-ai/sdk/issues/213))

### Features

* remove GetMissionSourceYAML from DaemonService ([#213](https://github.com/zeroroot-ai/sdk/issues/213)) ([dfdbb8c](https://github.com/zeroroot-ai/sdk/commit/dfdbb8cc1b4f01ec03a3235058177fa1c9d918d3))


### Bug Fixes

* **ci:** bump go directive to 1.25.10 to suppress govulncheck false positives ([#210](https://github.com/zeroroot-ai/sdk/issues/210)) ([6037a82](https://github.com/zeroroot-ai/sdk/commit/6037a82ecb0726260b2318e50e22d65dec45da70))

## [0.119.0](https://github.com/zeroroot-ai/sdk/compare/v0.118.0...v0.119.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* replace init()-driven Prometheus registration with opt-in interface (sdk#130) ([#207](https://github.com/zeroroot-ai/sdk/issues/207))

### Features

* replace init()-driven Prometheus registration with opt-in interface (sdk[#130](https://github.com/zeroroot-ai/sdk/issues/130)) ([#207](https://github.com/zeroroot-ai/sdk/issues/207)) ([089c812](https://github.com/zeroroot-ai/sdk/commit/089c8122bb09b9fbdd47ed61d382f83755857acb))
* replace unbounded mission/findings maps with injectable bounded store (sdk[#131](https://github.com/zeroroot-ai/sdk/issues/131)) ([#209](https://github.com/zeroroot-ai/sdk/issues/209)) ([986909c](https://github.com/zeroroot-ai/sdk/commit/986909ca66e16d17617b9d77a80c34c5d5e1247b))

## [0.118.0](https://github.com/zeroroot-ai/sdk/compare/v0.117.0...v0.118.0) (2026-05-24)


### Features

* **proto:** add CUE RPCs to DaemonService; add TenantService with tenant management RPCs ([#204](https://github.com/zeroroot-ai/sdk/issues/204)) ([#205](https://github.com/zeroroot-ai/sdk/issues/205)) ([677ff7a](https://github.com/zeroroot-ai/sdk/commit/677ff7af616e0082447c4ecf2c70854bcd342b4d))

## [0.117.0](https://github.com/zeroroot-ai/sdk/compare/v0.116.0...v0.117.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* single serve codepath — SPIFFE transport folded into platform_serve; delete spiffe_serve.go ([#199](https://github.com/zeroroot-ai/sdk/issues/199))
* replace detectMode with validateConfig+useSPIFFETransport; remove Mode enum ([#196](https://github.com/zeroroot-ai/sdk/issues/196))
* remove ModeLocal — every component requires platform auth ([#187](https://github.com/zeroroot-ai/sdk/issues/187))
* removed `gibson.budget.v1` proto package and Go bindings `github.com/zeroroot-ai/sdk/api/gen/gibson/budget/v1` from the OSS SDK. Consumers must migrate to `github.com/zeroroot-ai/platform-sdk/gen/gibson/budget/v1` (private module). Customer-facing `llm.IsBudgetExceeded` is unchanged.
* purge sdk/secrets and drop infra deps from oss sdk go.mod ([#113](https://github.com/zeroroot-ai/sdk/issues/113))
* gibson.admin.v1, gibson.usage.v1, gibson.authz.v1, and gibson.daemon.discovery.v1 have been removed from the OSS SDK. Daemon + dashboard now resolve these descriptors via github.com/zeroroot-ai/platform-sdk. SDK consumers that imported these packages directly (none expected — they were dashboard-only historically) must switch to the platform-sdk module.
* **secrets/vault:** secrets/providers/vault now imports github.com/openbao/openbao/api/v2 instead of github.com/hashicorp/vault/api. Public Go types unchanged, but transitive dep tree is different; consumers should re-run `go mod tidy`. AuthMethodAWSIAM returns an unsupported error pending #98.
* **vault:** AuthMethodKubernetes and AuthConfig.ServiceAccountTokenPath are removed from the public API. Callers that previously selected kubernetes auth must migrate to AuthMethodJWT (SPIFFE/Zitadel JWT) or AuthMethodAppRole.
* add GetMissionDefinition RPC + unify MissionConstraints (M5 + M2-sdk) ([#63](https://github.com/zeroroot-ai/sdk/issues/63))
* GIBSON_API_KEY env var deleted. EnvAPIKey constant removed from public daemonclient package surface. Tests using the env var to satisfy credential detection must switch to OIDC env config or explicit NewWithCredentials.
* **sdk:** v1.2.0 — manifest-driven plugin runtime (BREAKING)
* **graphrag:** Removes graphrag/domain package
* Removes schema.TaxonomyMapping and WithTaxonomy() API.

### Features

* Add --port CLI flag and GIBSON_PORT env var support for tools ([5650213](https://github.com/zeroroot-ai/sdk/commit/565021338a9fd9d9c1391b98c8256a04c6c8ea7d))
* add auth package and intelligence queries ([9439b1a](https://github.com/zeroroot-ai/sdk/commit/9439b1a8b15d93108cf52a7dac8163b3bbeecbc6))
* add canonical_constraints to harness CreateMissionRequest (sdk[#64](https://github.com/zeroroot-ai/sdk/issues/64)) ([#77](https://github.com/zeroroot-ai/sdk/issues/77)) ([6929fe6](https://github.com/zeroroot-ai/sdk/commit/6929fe6bc2009fddbfdfc88ec8889bdcc874a784))
* add checkpoint types for harness integration ([cc217f7](https://github.com/zeroroot-ai/sdk/commit/cc217f7a261f31b707fda28be7d3f16694b48296))
* add daemon proto, generated Go types, and daemon client package ([b2ad220](https://github.com/zeroroot-ai/sdk/commit/b2ad220f33434164357b5699308e401415e0ef93))
* add distributed tracing with span proxying to daemon ([4bdba9d](https://github.com/zeroroot-ai/sdk/commit/4bdba9d81abf31355a9f2740bc629b35ffd67899))
* Add enum value normalization for tool input JSON ([4e8b102](https://github.com/zeroroot-ai/sdk/commit/4e8b102c030263aa66dd114d825a04f7b3870e5b))
* add explicit mission_id field to callback protocol ([1452d3c](https://github.com/zeroroot-ai/sdk/commit/1452d3c8a028ee4819333ad09933d9b7d3c5cf0b))
* Add FileDescriptorSet support for tool schema introspection ([e0762e5](https://github.com/zeroroot-ai/sdk/commit/e0762e50a6bac752b1c5b6e3269650bcf9a34b8d))
* add GetCredential RPC for secure credential retrieval ([79aff3a](https://github.com/zeroroot-ai/sdk/commit/79aff3a77befe645413e3f833ce0b6895deb2230))
* add GetMissionDefinition RPC + unify MissionConstraints (M5 + M2-sdk) ([#63](https://github.com/zeroroot-ai/sdk/issues/63)) ([460b305](https://github.com/zeroroot-ai/sdk/commit/460b305e81ce214207f29fb703af3e79a613e6c1))
* Add GraphRAG domain types, remove taxonomy mapping ([1740c76](https://github.com/zeroroot-ai/sdk/commit/1740c769f430c39284c8852968987c12fc38faa2))
* add gRPC keepalive and parallel tool execution ([023b1ab](https://github.com/zeroroot-ai/sdk/commit/023b1abd05bb09605d1c366b684f42f194424cc7))
* add harness callback support for external agents ([96ca09a](https://github.com/zeroroot-ai/sdk/commit/96ca09ad25b95c0d5be90984272a74970731992c))
* Add health/http and errors packages ([3bfa68f](https://github.com/zeroroot-ai/sdk/commit/3bfa68f7a152b93ca7da833056926d6ec906c01e))
* add memory tier support for callbacks and GraphRAG taxonomy ([ca91151](https://github.com/zeroroot-ai/sdk/commit/ca91151b10a63233ab33c7e200cd326a58d6e3a3))
* Add missing taxonomy constants for GraphRAG alignment ([677021b](https://github.com/zeroroot-ai/sdk/commit/677021b342b75130e26f5d844bc3e1fd34fdb116))
* add mission management to SDK Harness interface ([40bc7a2](https://github.com/zeroroot-ai/sdk/commit/40bc7a27aaaebc487cd9e27af1bb15ad81928cc9))
* Add mission-scoped storage Phase 1 ([b681174](https://github.com/zeroroot-ai/sdk/commit/b681174125f62b4776d1189c2bffecebb5924357))
* add MissionConstraints proto + wire through FromDefinition projection (closes [#47](https://github.com/zeroroot-ai/sdk/issues/47)) ([#49](https://github.com/zeroroot-ai/sdk/issues/49)) ([95b3c94](https://github.com/zeroroot-ai/sdk/commit/95b3c94d735c5980bb575302985f79c54c86de3e))
* add ontology extension types, parser, and validator ([#29](https://github.com/zeroroot-ai/sdk/issues/29)) ([aa435c9](https://github.com/zeroroot-ai/sdk/commit/aa435c9929ca6d9699ce45037b805d2c0aad35c9))
* add pluginkit SDK package and platform config injection ([9b4d6b4](https://github.com/zeroroot-ai/sdk/commit/9b4d6b43e8bff1e1465d1755ce6a300386473fd4))
* Add Redis-based tool work queue for horizontal scaling ([3dc4464](https://github.com/zeroroot-ai/sdk/commit/3dc4464f795a7139af81b83ccf1cf4205379d656))
* add SDK fan-out workflow ([#5](https://github.com/zeroroot-ai/sdk/issues/5)) ([9313a12](https://github.com/zeroroot-ai/sdk/commit/9313a127e2d737f195e79dbbc35150117a7a4688))
* Add semantic error recovery with classification and recovery hints ([e3f3e88](https://github.com/zeroroot-ai/sdk/commit/e3f3e882900fd3a56f8530d44e5160c8e0659e3d))
* add streaming support for real-time agent event emission ([3528d6f](https://github.com/zeroroot-ai/sdk/commit/3528d6f52cdc57a66e7c1639cf29dbc1e7076bad))
* Add structured error handling with ResultError type ([fcd00e8](https://github.com/zeroroot-ai/sdk/commit/fcd00e8ee0b85b1f40251a36b58035553c3f2ea9))
* Add taxonomy adapter and callback enhancements ([86ab149](https://github.com/zeroroot-ai/sdk/commit/86ab1492618daa4225d288a747c56204b1aaf96e))
* Add toolspb proto definitions for open source tools ([bbb85dc](https://github.com/zeroroot-ai/sdk/commit/bbb85dc9e0cac9d6f66b78ede4dce68e94181b72))
* Add UUID-based entity identity and taxonomy-driven relationships ([b899846](https://github.com/zeroroot-ai/sdk/commit/b899846520dd6ea0f22c457c69b9ab06222e28fa))
* add WithPlatformFromEnv() unified platform auto-detect option ([2a8faba](https://github.com/zeroroot-ai/sdk/commit/2a8fabaed6896226acec26431596e898f8d9df0e))
* **admin:** add TenantAdminService.CountSecrets RPC ([12bfb16](https://github.com/zeroroot-ai/sdk/commit/12bfb167698de52d36c49e989d0db590243db088))
* Agent Auth Protocol client, remove legacy auth ([64da4bd](https://github.com/zeroroot-ai/sdk/commit/64da4bdf69985a590f1758539d50a4bdcdba4a7f))
* **agentauth:** bootstrap-from-secret + revocation helpers for AgentEnrollment ([e0de5dc](https://github.com/zeroroot-ai/sdk/commit/e0de5dcbba9c12f37ef340e37dd6bc3c31918db9))
* **api:** v0.103.0 — checkpoint browser RPCs + mission-memory callbacks + idempotency ([52fd824](https://github.com/zeroroot-ai/sdk/commit/52fd824027bf8653a3510637fe39ed8c9bc87171))
* **api:** v1.0.0 — manifest field 100 renumber to 200/201 (BREAKING) ([18e2f69](https://github.com/zeroroot-ai/sdk/commit/18e2f694c635edeb94ecdca3d9d6a1ea042ef762))
* **auth:** add Capabilities field to Identity and read_only to PluginMethodDescriptor ([951bd7f](https://github.com/zeroroot-ai/sdk/commit/951bd7f7d5288b68790bacf3cd6061998063d32b))
* authz-05 SDK harness.Authorize, AuthzContext envelope, HMAC signing ([3c560fc](https://github.com/zeroroot-ai/sdk/commit/3c560fc300d8e65e92abef0f377ebe55e6a0c7ba))
* **classifier:** add vector-based category classifier ([2dd2a92](https://github.com/zeroroot-ai/sdk/commit/2dd2a923e1035a44ee1ab091421e7654417662b9))
* **codegen:** add code generation SDK for autonomous agents ([51cd462](https://github.com/zeroroot-ai/sdk/commit/51cd4628c8fac902cb296dfa9357eefbceb7cc3a))
* complete platform dispatch SDK ([48d9877](https://github.com/zeroroot-ai/sdk/commit/48d98776d9b451774af537e2bffe9b77687d061c))
* convert check-no-gibson to AST boundary test (slice 3.4) ([#85](https://github.com/zeroroot-ai/sdk/issues/85)) ([bb4ce36](https://github.com/zeroroot-ai/sdk/commit/bb4ce367fb5b8adbf7bebd77eb11fd33846aec6f))
* **cueschemas:** export embed.FS of CUE module tree ([#148](https://github.com/zeroroot-ai/sdk/issues/148)) ([c592d16](https://github.com/zeroroot-ai/sdk/commit/c592d1600505915b6cf418c2f7a75268989fbb15))
* declare bsr module name and publish on tagged releases ([#110](https://github.com/zeroroot-ai/sdk/issues/110)) ([df50528](https://github.com/zeroroot-ai/sdk/commit/df505280d622f189f5183e36df296275ffbd73ab))
* delete vestigial GIBSON_API_KEY path from daemonclient ([#41](https://github.com/zeroroot-ai/sdk/issues/41)) ([e67acf3](https://github.com/zeroroot-ai/sdk/commit/e67acf30ee0d05075fe3ccf7444caf8119b7666e))
* derive fanout token_repos from consumer go.mod ([#123](https://github.com/zeroroot-ai/sdk/issues/123)) ([e934d17](https://github.com/zeroroot-ai/sdk/commit/e934d17627365bf174ed613c26bb21d72d2734e7)), closes [#120](https://github.com/zeroroot-ai/sdk/issues/120)
* **domain:** Add convenience accessors for DiscoveryResult ([b0f5db0](https://github.com/zeroroot-ai/sdk/commit/b0f5db06d6b2fa73ee3e5818956309d40a0aa096))
* **domain:** Add CustomNode builder and convenience methods ([cd76bc1](https://github.com/zeroroot-ai/sdk/commit/cd76bc111d0533d3a6cd4b7160d3c4b47b495f49))
* eliminate lazy JSON fields from proto definitions ([2a393b9](https://github.com/zeroroot-ai/sdk/commit/2a393b931dd1bdd92b89f4a099f2806da5dfa604))
* extract budget value types to public gibson.budget_status.v1 package ([#124](https://github.com/zeroroot-ai/sdk/issues/124)) ([bbe180e](https://github.com/zeroroot-ai/sdk/commit/bbe180ed9c8c4087155739ea084e7215fd993631))
* extract capabilitygrantinfo to public gibson.capability.v1 package ([#103](https://github.com/zeroroot-ai/sdk/issues/103)) ([8f11f8e](https://github.com/zeroroot-ai/sdk/commit/8f11f8e44a2111e75d70a1a31f25db8ac886b824))
* **extraction:** add EntityExtractor framework and serve.WithExtractor() option ([cd9e354](https://github.com/zeroroot-ai/sdk/commit/cd9e354a38935ab273deb2a0ae276af6c0920644))
* **finding:** Add metadata, registry, and security packages ([0ebf998](https://github.com/zeroroot-ai/sdk/commit/0ebf9985bb78161d7496e0976f9fcf32f2122271))
* **graph,daemon:** GetFindings + GetMissionSourceYAML + source_yaml capture ([bb761fe](https://github.com/zeroroot-ai/sdk/commit/bb761fec22709a6176c181209a660c72a1d705af))
* **graph:** add finding-analytics + graph-stats/summary/context RPCs ([2277cd4](https://github.com/zeroroot-ai/sdk/commit/2277cd4439dd21c437f592cf1fc29e07e326735c))
* **graph:** add gibson.graph.v1.GraphService ([cd9c167](https://github.com/zeroroot-ai/sdk/commit/cd9c16765146536c1052a075a0a3bfda0304923d))
* **graphrag:** add taxonomy extension introspection to SDK ([c23a2c0](https://github.com/zeroroot-ai/sdk/commit/c23a2c002365295c63309de34162fbb789552d0a))
* **graphrag:** Proto-first taxonomy - eliminate domain wrapper layer ([2da3e7a](https://github.com/zeroroot-ai/sdk/commit/2da3e7ae898b225a5f2ee4e7d1d4df714eacdbc8))
* **harness:** callback Workspace RPCs (file IO + commit + push) ([fd81eb0](https://github.com/zeroroot-ai/sdk/commit/fd81eb0912741057d5b14d3b28d9f69b0562348e))
* Implement deterministic content-addressable graph IDs ([3c13f29](https://github.com/zeroroot-ai/sdk/commit/3c13f294e891615521ee46a0fd15b6bc554a44aa))
* implement mission execution context and scoped GraphRAG methods ([60c4699](https://github.com/zeroroot-ai/sdk/commit/60c46993de2e164164e9c94062fa9e71a763c953))
* install release-please and pr-title-lint ([#4](https://github.com/zeroroot-ai/sdk/issues/4)) ([86e1f58](https://github.com/zeroroot-ai/sdk/commit/86e1f5837ca6ec77ae949838ce21fd7a961c24f0))
* **memory:** delete Redis/Neo4j backends, route all tiers through daemon ([#179](https://github.com/zeroroot-ai/sdk/issues/179)) ([eb01d29](https://github.com/zeroroot-ai/sdk/commit/eb01d29fbe083a47bcfbf554bc00426b7a7c80c5))
* **memory:** harnessLongTermMemory — long-term tier via ComponentService ([#177](https://github.com/zeroroot-ai/sdk/issues/177)) ([f09a145](https://github.com/zeroroot-ai/sdk/commit/f09a1452a4ab62a19cbea215c68279090c793a3c))
* **memory:** harnessMissionMemory — mission tier via ComponentService ([#178](https://github.com/zeroroot-ai/sdk/issues/178)) ([ab1512b](https://github.com/zeroroot-ai/sdk/commit/ab1512ba4e12f5b2e61e1e959e1f03fc4ca52da3)), closes [#172](https://github.com/zeroroot-ai/sdk/issues/172)
* **memory:** harnessWorkingMemory — working tier via ComponentService ([#175](https://github.com/zeroroot-ai/sdk/issues/175)) ([a5ef5d9](https://github.com/zeroroot-ai/sdk/commit/a5ef5d9409ee6a4648470535b8de2edb9fa964d0))
* Migrate tool interface from JSON schema to proto-based execution ([9fbc4de](https://github.com/zeroroot-ai/sdk/commit/9fbc4deba12313d36b86ec55384ef14cc2429234))
* move componentpb and taxonomy-gen to SDK, remove gibson dependency ([e1f9ec7](https://github.com/zeroroot-ai/sdk/commit/e1f9ec75415e3cbf49cc70f8dc8995ee6abbe4cf))
* **protoresolver:** Add ProtoResolver for dynamic type resolution ([cf4961c](https://github.com/zeroroot-ai/sdk/commit/cf4961cb871cd0d231e12b68eac19c962613dec9))
* purge sdk/secrets and drop infra deps from oss sdk go.mod ([#113](https://github.com/zeroroot-ai/sdk/issues/113)) ([774c639](https://github.com/zeroroot-ai/sdk/commit/774c6391ef9d9fc2c99028e3a1f66eb245620f60))
* **registry:** add automatic re-registration on lease expiry ([d1cd0dd](https://github.com/zeroroot-ai/sdk/commit/d1cd0ddceddcce2df01abf29b12adf22c1e8c56c))
* remove admin v1 + usage + authz + discovery protos (moved to platform-sdk) ([#105](https://github.com/zeroroot-ai/sdk/issues/105)) ([1a87af4](https://github.com/zeroroot-ai/sdk/commit/1a87af42971ae7d6c272dffc0f640d860d73df35))
* remove gibson.budget.v1.BudgetService from oss sdk (moved to platform-sdk under sdk[#106](https://github.com/zeroroot-ai/sdk/issues/106)) ([#127](https://github.com/zeroroot-ai/sdk/issues/127)) ([4fbcea6](https://github.com/zeroroot-ai/sdk/commit/4fbcea6bb398f265b214fe785a209a8192e25869))
* remove ModeLocal — every component requires platform auth ([#187](https://github.com/zeroroot-ai/sdk/issues/187)) ([be74475](https://github.com/zeroroot-ai/sdk/commit/be74475f8e20c0688a6d64f8346afe6d9e339b3e))
* replace detectMode with validateConfig+useSPIFFETransport; remove Mode enum ([#196](https://github.com/zeroroot-ai/sdk/issues/196)) ([dc9580e](https://github.com/zeroroot-ai/sdk/commit/dc9580e0d0030b38a2069a4a20d9bf760c8a7f90))
* **schema:** add Any() function for untyped schema definitions ([90ef5b7](https://github.com/zeroroot-ai/sdk/commit/90ef5b7117f9a91b4c19ea5a0b651c57f6e9c776))
* **schema:** Add taxonomy mapping support for GraphRAG integration ([70c1292](https://github.com/zeroroot-ai/sdk/commit/70c12927ab70eb710eb74edd20581814e454d3f6))
* **sdk:** v1.1.1 — vault token refresh + SPIFFE callback creds ([79a8b22](https://github.com/zeroroot-ai/sdk/commit/79a8b22ca7e4b5af3aaac06cacc88de7e500f972))
* **sdk:** v1.2.0 — manifest-driven plugin runtime (BREAKING) ([979a02e](https://github.com/zeroroot-ai/sdk/commit/979a02ec684f422b1a70bb9a1f2d8206f81cb694))
* **secrets/vault:** add OpenBao 2.5.3 CI smoke test service ([#92](https://github.com/zeroroot-ai/sdk/issues/92)) ([9f95c61](https://github.com/zeroroot-ai/sdk/commit/9f95c61d433a4b11ce383b2800cfb9081c8ee28a))
* **secrets/vault:** add OpenBao compat suite (slice 3 of [#90](https://github.com/zeroroot-ai/sdk/issues/90)) ([#97](https://github.com/zeroroot-ai/sdk/issues/97)) ([9a9786a](https://github.com/zeroroot-ai/sdk/commit/9a9786ae35e4728245dd55ad1113e1d2c24eefa3))
* **secrets/vault:** swap Go client from hashicorp/vault/api to openbao/openbao/api/v2 ([#99](https://github.com/zeroroot-ai/sdk/issues/99)) ([4aaa996](https://github.com/zeroroot-ai/sdk/commit/4aaa996e79544f75197d7778eb92b958e93ba4e8))
* self-contained tool protos architecture ([b37248f](https://github.com/zeroroot-ai/sdk/commit/b37248fbae2a4d8cb8367e948aee82713b4c9d2b))
* **serve:** add subprocess execution mode for tools ([5f32cae](https://github.com/zeroroot-ai/sdk/commit/5f32caef0675856aaed0777e19cf4c66b4978d20))
* **serve:** auto-detect K8s ServiceAccount token for component auth ([fa8a9da](https://github.com/zeroroot-ai/sdk/commit/fa8a9da4942705ce92ea487bcf363359724875b4))
* **serve:** full PlatformHarness parity with CallbackHarness — v0.60.0 ([49e03df](https://github.com/zeroroot-ai/sdk/commit/49e03dfacc218eabccdca8d65ea0e32ca1be44c1))
* **serve:** wire OntologyExtension over RegisterComponent enrollment ([#32](https://github.com/zeroroot-ai/sdk/issues/32)) ([346935b](https://github.com/zeroroot-ai/sdk/commit/346935bc046604e8342a533ded17f8b4b1063cc8))
* single serve codepath — SPIFFE transport folded into platform_serve; delete spiffe_serve.go ([#199](https://github.com/zeroroot-ai/sdk/issues/199)) ([38086dc](https://github.com/zeroroot-ai/sdk/commit/38086dcf560f53f8241fed1ccdb59c7a2680a074))
* SPIFFE serve loop for in-cluster workload identity ([eaa053f](https://github.com/zeroroot-ai/sdk/commit/eaa053f30470eccdd3ad3708b47719884e6903e1))
* **vault:** add explicit json tags to Config + AuthConfig ([#79](https://github.com/zeroroot-ai/sdk/issues/79)) ([95ba36a](https://github.com/zeroroot-ai/sdk/commit/95ba36a11c2ffb4fdadecc1890897ca80d0dc222))
* **vault:** rip AuthMethodKubernetes — see ADR-0009 (no TokenReview-based auth) ([#81](https://github.com/zeroroot-ai/sdk/issues/81)) ([4cbf37b](https://github.com/zeroroot-ai/sdk/commit/4cbf37b4e035ffac96dc500538c27be806b4d31b))
* wire platform memory tiers, bump to v0.43.0 ([29033be](https://github.com/zeroroot-ai/sdk/commit/29033bee78dbdc7a9e572037dd00f5ff496bc7b1))
* YAML-driven taxonomy system with generated domain types ([421a878](https://github.com/zeroroot-ai/sdk/commit/421a87826f052f46a58bdf53650f31ab62b845d5))


### Bug Fixes

* auto-re-register components when daemon restarts ([efd8a77](https://github.com/zeroroot-ai/sdk/commit/efd8a770e47b4475490b2dba4f2ef09ecb1a9584))
* **build:** remove polyrepo path leak from mission-jsonschema ([#16](https://github.com/zeroroot-ai/sdk/issues/16)) ([d884dca](https://github.com/zeroroot-ai/sdk/commit/d884dca9ca2c45a274a86a10cc35d97c4313321d))
* **ci:** add Docs-PR trailer to fan-out bump PR body ([#168](https://github.com/zeroroot-ai/sdk/issues/168)) ([dce16b6](https://github.com/zeroroot-ai/sdk/commit/dce16b6139f1b4ad694368d0a285f8e4754aca1c))
* **ci:** fan-out derive step: use jq .content // empty to handle null ([#157](https://github.com/zeroroot-ai/sdk/issues/157)) ([c8dcded](https://github.com/zeroroot-ai/sdk/commit/c8dcded4379b268ffb512f8fe0151d207a234d4a))
* **ci:** fan-out derive: strip literal null returned by gh api --jq .content ([#160](https://github.com/zeroroot-ai/sdk/issues/160)) ([e76dfc4](https://github.com/zeroroot-ai/sdk/commit/e76dfc48fe75f9003a2f74b983250ade0861fcb6))
* **ci:** fan-out derive: use minimal App token to read private consumer go.mod ([#165](https://github.com/zeroroot-ai/sdk/issues/165)) ([7d8676f](https://github.com/zeroroot-ai/sdk/commit/7d8676f31499b6f7d6abc13a64f96135a7ffaa8b))
* **ci:** fan-out derive: use standalone jq pipe instead of --jq flag ([#163](https://github.com/zeroroot-ai/sdk/issues/163)) ([b551766](https://github.com/zeroroot-ai/sdk/commit/b5517660ef03d1d95c3702e99790fbfa292ac7d1))
* **ci:** fix fan-out YAML parse error + skip dispatch when token unset ([#181](https://github.com/zeroroot-ai/sdk/issues/181)) ([db679fb](https://github.com/zeroroot-ai/sdk/commit/db679fb5ef3b03f423061bc7bdecb05b449681f1))
* **ci:** migrate vault-deny-list allowlist to content-based entries ([#161](https://github.com/zeroroot-ai/sdk/issues/161)) ([c99a173](https://github.com/zeroroot-ai/sdk/commit/c99a1737b7e7b02796fcdf31a72d6fc5f2d8947c))
* **ci:** update GitHub Actions to Node.js 24 compatible versions ([#169](https://github.com/zeroroot-ai/sdk/issues/169)) ([8d49b1e](https://github.com/zeroroot-ai/sdk/commit/8d49b1ea3a91aaf0789d1e9f20e8d7b0c01dc7ef))
* **ci:** update vault deny-list allowlist for CHANGELOG line shift ([#155](https://github.com/zeroroot-ai/sdk/issues/155)) ([7d09cdf](https://github.com/zeroroot-ai/sdk/commit/7d09cdfed7012c6619926d5fc445ba61d4db5937))
* complete TaxonomyIntrospector implementation across SDK ([c5ad146](https://github.com/zeroroot-ai/sdk/commit/c5ad1460cfec79eb8c4162c317972535f99d3e81))
* correct generated CUE import paths + package aliases (closes [#48](https://github.com/zeroroot-ai/sdk/issues/48)) ([#51](https://github.com/zeroroot-ai/sdk/issues/51)) ([ddb013a](https://github.com/zeroroot-ai/sdk/commit/ddb013aa70b2ff308aff9b5c67190733c6e0c26a))
* **deps:** bump grpc to v1.81.0 (CVE) + docker to v28.5.2 (Moby AuthZ + privilege) ([48e2b86](https://github.com/zeroroot-ai/sdk/commit/48e2b86617adb5ac792e590e56184b1bccd0b452))
* **deps:** bump toolchain go1.25.5 -&gt; go1.25.10 to patch 14 stdlib CVEs ([#24](https://github.com/zeroroot-ai/sdk/issues/24)) ([0754769](https://github.com/zeroroot-ai/sdk/commit/075476918f51204c252a8da8b372f174fe77cfea))
* **errors:** wrap sentinel errors at all not-found call sites (fixes [#132](https://github.com/zeroroot-ai/sdk/issues/132)) ([#152](https://github.com/zeroroot-ai/sdk/issues/152)) ([9fadfe4](https://github.com/zeroroot-ai/sdk/commit/9fadfe4ce8f2820da33a647096184ee3b095a885))
* **fan-out:** close YAML block scalars that were ending early ([cc1a6ce](https://github.com/zeroroot-ai/sdk/commit/cc1a6ce553539a2132dafccd0fdb596493aeb6ec))
* **fan-out:** drop --label dependencies, soft-fail authz regen ([6a9b4c5](https://github.com/zeroroot-ai/sdk/commit/6a9b4c58b24550c8d3df534c4608f3b9fbf8ee0a))
* handle array format for MissionContext.Constraints ([c44e298](https://github.com/zeroroot-ai/sdk/commit/c44e298131cfb5ffbf6fda4b08145160df348c5f))
* improve callback client connection handling and add observability ([a71e603](https://github.com/zeroroot-ai/sdk/commit/a71e603ba73d026ad1caabcb4abe25c80d2e8e46))
* include mission ID in callback task context for proper harness lookup ([f8e59d3](https://github.com/zeroroot-ai/sdk/commit/f8e59d3ca333c18246d067dd08e55dd9b55770f8))
* move helpers_generated.go to graphrag/domain/ (package mismatch) ([451cb41](https://github.com/zeroroot-ai/sdk/commit/451cb418ba4e74b05bab0f003d1e60c0ae2d74aa))
* **queue:** read file_descriptor_set in ListTools ([93af9c3](https://github.com/zeroroot-ai/sdk/commit/93af9c3c69dc94bed870275c09567b30a6a0f312))
* regen Go bindings missed by v0.105.0 release of ADR 0004 deletion ([#67](https://github.com/zeroroot-ai/sdk/issues/67)) ([400420c](https://github.com/zeroroot-ai/sdk/commit/400420c20414739cb34141f31cab4456f2c536c3))
* **release-please:** use App token so tag pushes trigger fan-out ([46fac73](https://github.com/zeroroot-ai/sdk/commit/46fac7380c844e52fe35ecc23dcea106ece0cf49))
* remove compat shims, update helpers tests for taxonomy v4 constructors ([792df30](https://github.com/zeroroot-ai/sdk/commit/792df3072d562a4711fb0be7dbd2f1f1a69a02a5))
* Remove misplaced taxonomy.pb.go from proto package ([28f7db4](https://github.com/zeroroot-ai/sdk/commit/28f7db48e8ab6c73dd3c292ae3914ac77d04dbe5))
* resolve proto type conflicts between commonpb and proto packages ([6dcbf7f](https://github.com/zeroroot-ai/sdk/commit/6dcbf7f414eca2c09eb4e1443a694a0e09919dc8))
* restore toolspb package (still imported by consumers) ([c11a1d9](https://github.com/zeroroot-ai/sdk/commit/c11a1d931b93e5f98cce4e69f0ab791bbfc528b1))
* rewrite fan-out workflow to avoid YAML conditional issues ([#7](https://github.com/zeroroot-ai/sdk/issues/7)) ([54d2488](https://github.com/zeroroot-ai/sdk/commit/54d2488be1e42579200b5110b7a01f4c8c7ea02c))
* sanitize invalid UTF-8 in protobuf string fields ([efb4d27](https://github.com/zeroroot-ai/sdk/commit/efb4d27857eb4f6daa8d5b1df6076649c78d9a27))
* SDK v0.61.1 — restore toolspb package ([f7e666e](https://github.com/zeroroot-ai/sdk/commit/f7e666e95f9950d50be1500e46953b61c443db37))
* Store tags as JSON string in Redis HSET ([d5ee641](https://github.com/zeroroot-ai/sdk/commit/d5ee6417d2b4646452da62eb43951a7d7eb1df74))
* update Makefile, finding categories, and tool serve callbacks ([2a2a3c9](https://github.com/zeroroot-ai/sdk/commit/2a2a3c9dfe90fb2f9169260ce4d01f2e2629c65b))
* Use string map for Redis HSET to avoid marshal errors ([9902531](https://github.com/zeroroot-ai/sdk/commit/9902531ca80b88b9d0c119de399494ddce4b1d11))

## [0.116.0](https://github.com/zeroroot-ai/sdk/compare/v0.115.1...v0.116.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* single serve codepath — SPIFFE transport folded into platform_serve; delete spiffe_serve.go ([#199](https://github.com/zeroroot-ai/sdk/issues/199))
* replace detectMode with validateConfig+useSPIFFETransport; remove Mode enum ([#196](https://github.com/zeroroot-ai/sdk/issues/196))
* remove ModeLocal — every component requires platform auth ([#187](https://github.com/zeroroot-ai/sdk/issues/187))

### Features

* remove ModeLocal — every component requires platform auth ([#187](https://github.com/zeroroot-ai/sdk/issues/187)) ([be74475](https://github.com/zeroroot-ai/sdk/commit/be74475f8e20c0688a6d64f8346afe6d9e339b3e))
* replace detectMode with validateConfig+useSPIFFETransport; remove Mode enum ([#196](https://github.com/zeroroot-ai/sdk/issues/196)) ([dc9580e](https://github.com/zeroroot-ai/sdk/commit/dc9580e0d0030b38a2069a4a20d9bf760c8a7f90))
* single serve codepath — SPIFFE transport folded into platform_serve; delete spiffe_serve.go ([#199](https://github.com/zeroroot-ai/sdk/issues/199)) ([38086dc](https://github.com/zeroroot-ai/sdk/commit/38086dcf560f53f8241fed1ccdb59c7a2680a074))

## [0.115.1](https://github.com/zeroroot-ai/sdk/compare/v0.115.0...v0.115.1) (2026-05-24)


### Bug Fixes

* **ci:** fix fan-out YAML parse error + skip dispatch when token unset ([#181](https://github.com/zeroroot-ai/sdk/issues/181)) ([db679fb](https://github.com/zeroroot-ai/sdk/commit/db679fb5ef3b03f423061bc7bdecb05b449681f1))

## [0.115.0](https://github.com/zeroroot-ai/sdk/compare/v0.114.3...v0.115.0) (2026-05-24)


### Features

* **memory:** delete Redis/Neo4j backends, route all tiers through daemon ([#179](https://github.com/zeroroot-ai/sdk/issues/179)) ([eb01d29](https://github.com/zeroroot-ai/sdk/commit/eb01d29fbe083a47bcfbf554bc00426b7a7c80c5))
* **memory:** harnessLongTermMemory — long-term tier via ComponentService ([#177](https://github.com/zeroroot-ai/sdk/issues/177)) ([f09a145](https://github.com/zeroroot-ai/sdk/commit/f09a1452a4ab62a19cbea215c68279090c793a3c))
* **memory:** harnessMissionMemory — mission tier via ComponentService ([#178](https://github.com/zeroroot-ai/sdk/issues/178)) ([ab1512b](https://github.com/zeroroot-ai/sdk/commit/ab1512ba4e12f5b2e61e1e959e1f03fc4ca52da3)), closes [#172](https://github.com/zeroroot-ai/sdk/issues/172)
* **memory:** harnessWorkingMemory — working tier via ComponentService ([#175](https://github.com/zeroroot-ai/sdk/issues/175)) ([a5ef5d9](https://github.com/zeroroot-ai/sdk/commit/a5ef5d9409ee6a4648470535b8de2edb9fa964d0))

## [Unreleased]

### Changed

- `NewHarnessStore` long-term tier is now fully wired: backed by `harnessLongTermMemory` which calls ComponentService Memory* RPCs with tier="long_term". The previous stub (`NewMockLongTermMemory`) is replaced.

### Removed

- `memory.Factory`, `memory.NewFactory`, `memory.ProductionConfig` — direct-connection Redis/Neo4j memory backends removed. Memory is now accessed through `Harness.Memory()` backed by the daemon's `ComponentService` RPCs.
- `memory.NewRedisWorkingMemory`, `memory.NewRedisMissionMemory`, `memory.NewNeo4jLongTermMemory`, `memory.NewNeo4jDriver`, `memory.NewRedisClientFromConfig` — same reason.
- `queue` package removed entirely. Tool dispatch is managed by the daemon; customer code does not interact with work queues directly.
- `tool/worker` package removed (depended on the deleted `queue` package).
- Removed direct dependencies: `github.com/redis/go-redis/v9`, `github.com/neo4j/neo4j-go-driver/v5`, `github.com/alicebob/miniredis/v2`.

## [0.114.3](https://github.com/zeroroot-ai/sdk/compare/v0.114.2...v0.114.3) (2026-05-24)


### Bug Fixes

* **ci:** add Docs-PR trailer to fan-out bump PR body ([#168](https://github.com/zeroroot-ai/sdk/issues/168)) ([dce16b6](https://github.com/zeroroot-ai/sdk/commit/dce16b6139f1b4ad694368d0a285f8e4754aca1c))
* **ci:** fan-out derive: use minimal App token to read private consumer go.mod ([#165](https://github.com/zeroroot-ai/sdk/issues/165)) ([7d8676f](https://github.com/zeroroot-ai/sdk/commit/7d8676f31499b6f7d6abc13a64f96135a7ffaa8b))
* **ci:** update GitHub Actions to Node.js 24 compatible versions ([#169](https://github.com/zeroroot-ai/sdk/issues/169)) ([8d49b1e](https://github.com/zeroroot-ai/sdk/commit/8d49b1ea3a91aaf0789d1e9f20e8d7b0c01dc7ef))

## [0.114.2](https://github.com/zeroroot-ai/sdk/compare/v0.114.1...v0.114.2) (2026-05-24)


### Bug Fixes

* **ci:** fan-out derive step: use jq .content // empty to handle null ([#157](https://github.com/zeroroot-ai/sdk/issues/157)) ([c8dcded](https://github.com/zeroroot-ai/sdk/commit/c8dcded4379b268ffb512f8fe0151d207a234d4a))
* **ci:** fan-out derive: strip literal null returned by gh api --jq .content ([#160](https://github.com/zeroroot-ai/sdk/issues/160)) ([e76dfc4](https://github.com/zeroroot-ai/sdk/commit/e76dfc48fe75f9003a2f74b983250ade0861fcb6))
* **ci:** fan-out derive: use standalone jq pipe instead of --jq flag ([#163](https://github.com/zeroroot-ai/sdk/issues/163)) ([b551766](https://github.com/zeroroot-ai/sdk/commit/b5517660ef03d1d95c3702e99790fbfa292ac7d1))
* **ci:** migrate vault-deny-list allowlist to content-based entries ([#161](https://github.com/zeroroot-ai/sdk/issues/161)) ([c99a173](https://github.com/zeroroot-ai/sdk/commit/c99a1737b7e7b02796fcdf31a72d6fc5f2d8947c))

## [0.114.1](https://github.com/zeroroot-ai/sdk/compare/v0.114.0...v0.114.1) (2026-05-24)


### Bug Fixes

* **ci:** update vault deny-list allowlist for CHANGELOG line shift ([#155](https://github.com/zeroroot-ai/sdk/issues/155)) ([7d09cdf](https://github.com/zeroroot-ai/sdk/commit/7d09cdfed7012c6619926d5fc445ba61d4db5937))
* **errors:** wrap sentinel errors at all not-found call sites (fixes [#132](https://github.com/zeroroot-ai/sdk/issues/132)) ([#152](https://github.com/zeroroot-ai/sdk/issues/152)) ([9fadfe4](https://github.com/zeroroot-ai/sdk/commit/9fadfe4ce8f2820da33a647096184ee3b095a885))

## [0.114.0](https://github.com/zeroroot-ai/sdk/compare/v0.113.0...v0.114.0) (2026-05-23)


### Features

* **cueschemas:** export embed.FS of CUE module tree ([#148](https://github.com/zeroroot-ai/sdk/issues/148)) ([c592d16](https://github.com/zeroroot-ai/sdk/commit/c592d1600505915b6cf418c2f7a75268989fbb15))

## [0.113.0](https://github.com/zeroroot-ai/sdk/compare/v0.112.0...v0.113.0) (2026-05-21)


### ⚠ BREAKING CHANGES

* removed `gibson.budget.v1` proto package and Go bindings `github.com/zeroroot-ai/sdk/api/gen/gibson/budget/v1` from the OSS SDK. Consumers must migrate to `github.com/zeroroot-ai/platform-sdk/gen/gibson/budget/v1` (private module). Customer-facing `llm.IsBudgetExceeded` is unchanged.

### Features

* remove gibson.budget.v1.BudgetService from oss sdk (moved to platform-sdk under sdk[#106](https://github.com/zeroroot-ai/sdk/issues/106)) ([#127](https://github.com/zeroroot-ai/sdk/issues/127)) ([4fbcea6](https://github.com/zeroroot-ai/sdk/commit/4fbcea6bb398f265b214fe785a209a8192e25869))

## [0.112.0](https://github.com/zeroroot-ai/sdk/compare/v0.111.0...v0.112.0) (2026-05-21)


### Features

* derive fanout token_repos from consumer go.mod ([#123](https://github.com/zeroroot-ai/sdk/issues/123)) ([e934d17](https://github.com/zeroroot-ai/sdk/commit/e934d17627365bf174ed613c26bb21d72d2734e7)), closes [#120](https://github.com/zeroroot-ai/sdk/issues/120)
* extract budget value types to public gibson.budget_status.v1 package ([#124](https://github.com/zeroroot-ai/sdk/issues/124)) ([bbe180e](https://github.com/zeroroot-ai/sdk/commit/bbe180ed9c8c4087155739ea084e7215fd993631))

## [0.111.0](https://github.com/zeroroot-ai/sdk/compare/v0.110.0...v0.111.0) (2026-05-21)


### ⚠ BREAKING CHANGES

* purge sdk/secrets and drop infra deps from oss sdk go.mod ([#113](https://github.com/zeroroot-ai/sdk/issues/113))

### Features

* purge sdk/secrets and drop infra deps from oss sdk go.mod ([#113](https://github.com/zeroroot-ai/sdk/issues/113)) ([774c639](https://github.com/zeroroot-ai/sdk/commit/774c6391ef9d9fc2c99028e3a1f66eb245620f60))

## [0.110.0](https://github.com/zeroroot-ai/sdk/compare/v0.109.0...v0.110.0) (2026-05-20)


### Features

* declare bsr module name and publish on tagged releases ([#110](https://github.com/zeroroot-ai/sdk/issues/110)) ([df50528](https://github.com/zeroroot-ai/sdk/commit/df505280d622f189f5183e36df296275ffbd73ab))

## [0.109.0](https://github.com/zeroroot-ai/sdk/compare/v0.108.0...v0.109.0) (2026-05-20)


### ⚠ BREAKING CHANGES

* gibson.admin.v1, gibson.usage.v1, gibson.authz.v1, and gibson.daemon.discovery.v1 have been removed from the OSS SDK. Daemon + dashboard now resolve these descriptors via github.com/zeroroot-ai/platform-sdk. SDK consumers that imported these packages directly (none expected — they were dashboard-only historically) must switch to the platform-sdk module.

### Features

* remove admin v1 + usage + authz + discovery protos (moved to platform-sdk) ([#105](https://github.com/zeroroot-ai/sdk/issues/105)) ([1a87af4](https://github.com/zeroroot-ai/sdk/commit/1a87af42971ae7d6c272dffc0f640d860d73df35))

## [0.108.0](https://github.com/zeroroot-ai/sdk/compare/v0.107.0...v0.108.0) (2026-05-20)


### Features

* extract capabilitygrantinfo to public gibson.capability.v1 package ([#103](https://github.com/zeroroot-ai/sdk/issues/103)) ([8f11f8e](https://github.com/zeroroot-ai/sdk/commit/8f11f8e44a2111e75d70a1a31f25db8ac886b824))

## [0.107.0](https://github.com/zeroroot-ai/sdk/compare/v0.106.0...v0.107.0) (2026-05-20)


### ⚠ BREAKING CHANGES

* **secrets/vault:** secrets/providers/vault now imports github.com/openbao/openbao/api/v2 instead of github.com/hashicorp/vault/api. Public Go types unchanged, but transitive dep tree is different; consumers should re-run `go mod tidy`. AuthMethodAWSIAM returns an unsupported error pending #98.

### Features

* **secrets/vault:** add OpenBao 2.5.3 CI smoke test service ([#92](https://github.com/zeroroot-ai/sdk/issues/92)) ([9f95c61](https://github.com/zeroroot-ai/sdk/commit/9f95c61d433a4b11ce383b2800cfb9081c8ee28a))
* **secrets/vault:** add OpenBao compat suite (slice 3 of [#90](https://github.com/zeroroot-ai/sdk/issues/90)) ([#97](https://github.com/zeroroot-ai/sdk/issues/97)) ([9a9786a](https://github.com/zeroroot-ai/sdk/commit/9a9786ae35e4728245dd55ad1113e1d2c24eefa3))
* **secrets/vault:** swap Go client from hashicorp/vault/api to openbao/openbao/api/v2 ([#99](https://github.com/zeroroot-ai/sdk/issues/99)) ([4aaa996](https://github.com/zeroroot-ai/sdk/commit/4aaa996e79544f75197d7778eb92b958e93ba4e8))

## [0.106.0](https://github.com/zeroroot-ai/sdk/compare/v0.105.1...v0.106.0) (2026-05-19)


### ⚠ BREAKING CHANGES

* **vault:** AuthMethodKubernetes and AuthConfig.ServiceAccountTokenPath are removed from the public API. Callers that previously selected kubernetes auth must migrate to AuthMethodJWT (SPIFFE/Zitadel JWT) or AuthMethodAppRole.

### Features

* add canonical_constraints to harness CreateMissionRequest (sdk[#64](https://github.com/zeroroot-ai/sdk/issues/64)) ([#77](https://github.com/zeroroot-ai/sdk/issues/77)) ([6929fe6](https://github.com/zeroroot-ai/sdk/commit/6929fe6bc2009fddbfdfc88ec8889bdcc874a784))
* convert check-no-gibson to AST boundary test (slice 3.4) ([#85](https://github.com/zeroroot-ai/sdk/issues/85)) ([bb4ce36](https://github.com/zeroroot-ai/sdk/commit/bb4ce367fb5b8adbf7bebd77eb11fd33846aec6f))
* **vault:** add explicit json tags to Config + AuthConfig ([#79](https://github.com/zeroroot-ai/sdk/issues/79)) ([95ba36a](https://github.com/zeroroot-ai/sdk/commit/95ba36a11c2ffb4fdadecc1890897ca80d0dc222))
* **vault:** rip AuthMethodKubernetes — see ADR-0009 (no TokenReview-based auth) ([#81](https://github.com/zeroroot-ai/sdk/issues/81)) ([4cbf37b](https://github.com/zeroroot-ai/sdk/commit/4cbf37b4e035ffac96dc500538c27be806b4d31b))

## [0.105.1](https://github.com/zeroroot-ai/sdk/compare/v0.105.0...v0.105.1) (2026-05-17)


### Bug Fixes

* regen Go bindings missed by v0.105.0 release of ADR 0004 deletion ([#67](https://github.com/zeroroot-ai/sdk/issues/67)) ([400420c](https://github.com/zeroroot-ai/sdk/commit/400420c20414739cb34141f31cab4456f2c536c3))

## [0.105.0](https://github.com/zeroroot-ai/sdk/compare/v0.104.0...v0.105.0) (2026-05-17)


### ⚠ BREAKING CHANGES

* add GetMissionDefinition RPC + unify MissionConstraints (M5 + M2-sdk) ([#63](https://github.com/zeroroot-ai/sdk/issues/63))

### Features

* add GetMissionDefinition RPC + unify MissionConstraints (M5 + M2-sdk) ([#63](https://github.com/zeroroot-ai/sdk/issues/63)) ([460b305](https://github.com/zeroroot-ai/sdk/commit/460b305e81ce214207f29fb703af3e79a613e6c1))

## [0.104.0](https://github.com/zeroroot-ai/sdk/compare/v0.103.1...v0.104.0) (2026-05-17)

This release marks the polyrepo zero-dot-x reset (PRD zeroroot-ai/.github#25, board #14).

The v1.x line (v1.0.0 → v1.9.0) and v2.0.0 were cut prematurely; nothing in the platform is at 1.0 maturity yet. Those tags + GitHub Releases have been deleted. Going forward, all repos sit at 0.x with `bump-minor-pre-major: true`, so `feat!:` commits bump minor instead of major.

### Features

* delete vestigial GIBSON_API_KEY path from daemonclient ([#41](https://github.com/zeroroot-ai/sdk/issues/41))
* every other change previously listed under v1.0.0–v2.0.0 stays in main; only the version label changes

### Notes

* `GIBSON_API_KEY` env var deleted from `daemonclient`. Use OIDC client_credentials env config or `NewWithCredentials` instead.
* All consumer `go.mod` files were re-pinned to `v0.104.0` as part of the reset epic.


## [1.9.0](https://github.com/zeroroot-ai/sdk/compare/v1.8.0...v1.9.0) (2026-05-13)


### Features

* **serve:** wire OntologyExtension over RegisterComponent enrollment ([#32](https://github.com/zeroroot-ai/sdk/issues/32)) ([346935b](https://github.com/zeroroot-ai/sdk/commit/346935bc046604e8342a533ded17f8b4b1063cc8))

## [1.8.0](https://github.com/zeroroot-ai/sdk/compare/v1.7.4...v1.8.0) (2026-05-13)


### Features

* add ontology extension types, parser, and validator ([#29](https://github.com/zeroroot-ai/sdk/issues/29)) ([aa435c9](https://github.com/zeroroot-ai/sdk/commit/aa435c9929ca6d9699ce45037b805d2c0aad35c9))


### Bug Fixes

* **build:** remove polyrepo path leak from mission-jsonschema ([#16](https://github.com/zeroroot-ai/sdk/issues/16)) ([d884dca](https://github.com/zeroroot-ai/sdk/commit/d884dca9ca2c45a274a86a10cc35d97c4313321d))
* **deps:** bump toolchain go1.25.5 -&gt; go1.25.10 to patch 14 stdlib CVEs ([#24](https://github.com/zeroroot-ai/sdk/issues/24)) ([0754769](https://github.com/zeroroot-ai/sdk/commit/075476918f51204c252a8da8b372f174fe77cfea))

## [1.7.4](https://github.com/zeroroot-ai/sdk/compare/v1.7.3...v1.7.4) (2026-05-10)


### Bug Fixes

* **fan-out:** drop --label dependencies, soft-fail authz regen ([6a9b4c5](https://github.com/zeroroot-ai/sdk/commit/6a9b4c58b24550c8d3df534c4608f3b9fbf8ee0a))

## [1.7.3](https://github.com/zeroroot-ai/sdk/compare/v1.7.2...v1.7.3) (2026-05-10)


### Bug Fixes

* **release-please:** use App token so tag pushes trigger fan-out ([46fac73](https://github.com/zeroroot-ai/sdk/commit/46fac7380c844e52fe35ecc23dcea106ece0cf49))

## [1.7.2](https://github.com/zeroroot-ai/sdk/compare/v1.7.1...v1.7.2) (2026-05-10)


### Bug Fixes

* **fan-out:** close YAML block scalars that were ending early ([cc1a6ce](https://github.com/zeroroot-ai/sdk/commit/cc1a6ce553539a2132dafccd0fdb596493aeb6ec))

## [1.7.1](https://github.com/zeroroot-ai/sdk/compare/v1.7.0...v1.7.1) (2026-05-10)


### Bug Fixes

* rewrite fan-out workflow to avoid YAML conditional issues ([#7](https://github.com/zeroroot-ai/sdk/issues/7)) ([54d2488](https://github.com/zeroroot-ai/sdk/commit/54d2488be1e42579200b5110b7a01f4c8c7ea02c))

## [1.7.0](https://github.com/zeroroot-ai/sdk/compare/v1.6.0...v1.7.0) (2026-05-10)


### Features

* add SDK fan-out workflow ([#5](https://github.com/zeroroot-ai/sdk/issues/5)) ([9313a12](https://github.com/zeroroot-ai/sdk/commit/9313a127e2d737f195e79dbbc35150117a7a4688))
* install release-please and pr-title-lint ([#4](https://github.com/zeroroot-ai/sdk/issues/4)) ([86e1f58](https://github.com/zeroroot-ai/sdk/commit/86e1f5837ca6ec77ae949838ce21fd7a961c24f0))

## v1.2.0 — 2026-05-07

Removes the developer-facing scaffold package; scaffolding is now owned
by the ADK CLI (`zeroroot-ai/adk`).

### Removed

- **`github.com/zeroroot-ai/sdk/plugin/scaffold` (package).** Templates,
  the `Render` function, `ScaffoldInput`, `SecretInput`, and
  `ParseSecretFlag` have moved to
  `github.com/zeroroot-ai/adk/cmd/gibson/internal/scaffold`. Direct
  importers of the SDK package are expected to be limited to the ADK
  itself; downstream code that depended on it must vendor templates
  locally or call `gibson component init` instead.

  This is a breaking change for any out-of-band importer. The migration
  rationale (scaffolding is developer ergonomics, not a runtime
  contract) is captured in `.spec-workflow/specs/adk-developer-workflow/`.

## v0.99.0 — 2026-05-04

Adds a per-tenant secret-count RPC the dashboard uses to gate the
broker-switch migration warning. Also fixes a `make proto` ordering bug
that broke the target on clean trees.

### Added

- **`gibson.admin.v1.TenantAdminService.CountSecrets`** — returns the
  number of secrets currently stored in the tenant's active broker.
  Response carries no names, values, or per-row metadata — only an
  integer count. Used by the dashboard to gate the migration-warning UX
  when a tenant admin switches from one broker provider to another.
  `relation: "admin"`, `object_type: "tenant"`, `allowed_identities: 1`
  (USER) — same envelope as the existing `Get/Probe/SetBrokerConfig`
  trio.

  Spec: `tenant-secrets-broker-completion` (Task 1).

### Fixed

- **`make proto` no longer fails on a clean tree.** `proto-authz-registry`
  was a dep of the `proto` target but the recipe never consumed the
  binary it built — it was only used by `proto-authz-registry-emit` and
  `lint-authz`. Combined with `proto-clean` running first, this created
  a chicken-and-egg: the registry-gen tool imports from `api/gen/`,
  which `proto-clean` wipes. Removed the stale dep.

## v0.98.1 — 2026-05-02

Re-release of v0.98.0 against a fresh commit so the Go module proxy /
checksum database picks up the correct release tree (an aborted v0.98.0 tag
was briefly published against a commit without the `api/gen` snapshot, and
sum.golang.org pinned that snapshot — bumping the patch is the cleanest way
to invalidate it for downstream consumers).

No code changes vs. v0.98.0 below.

## v0.98.0 — 2026-05-02

Codegen surface change: drops the `fga_model.fga` coverage stub from the
authz registry. The OpenFGA model is hand-maintained at
`core/gibson/internal/authz/model.fga` (loaded by the `gibson-fga-init` Job
via a JSON file derived from that authoritative source); the generated stub
was unused at runtime and existed only as a derived snapshot of the
proto-annotated relations, which is already covered by `registry.yaml` and
`audit.csv`.

### Removed

- **`emitFGA` from `cmd/authz-registry-gen`** and its `fga_model.fga` output.
  `make proto-authz-registry-emit` and `make authz-registry` (in
  `zeroroot-ai/gibson`) now emit only `registry.go`, `registry.yaml`, and
  `permissions.ts`.
- `auth/registry/fga_model.fga` from `.gitignore` (no longer generated).
- `TestRun_FGAOutputContent` from `cmd/authz-registry-gen/main_test.go`;
  `TestRun_HappyPath_FourArtifacts` renamed to `…_ThreeArtifacts`.

### Migration

Downstream consumers must drop any reference to the
`fga_model.fga` artifact:

- `zeroroot-ai/gibson` — the regenerated registry no longer contains the
  file; the OCI publish workflow stops pushing the layer.
- ext-authz Helm chart — no change (it never read the artifact at runtime).
- `gibson-fga-init` Job — no change (it reads `files/fga-model.json` derived
  from the authoritative `model.fga`).

## v0.96.0 — 2026-05-01

Corrects eleven incoherent `allowed_identities` annotations on
`DiscoveryService` RPCs and adds CI guards against the same shape of
breakage recurring. Spec: `discovery-bitfield-coherence`.

### Fixed

- **`DiscoveryService` bitfields corrected from `8` (PLATFORM\_OPERATOR-only)
  to `7` (USER | SERVICE | COMPONENT)** on all eleven RPCs: `WhoAmI`,
  `ListPlugins`, `DescribePlugin`, `ListTools`, `DescribeTool`, `ListAgents`,
  `DescribeAgent`, `ListLLMSlots`, `ListReportSurfaces`, `ValidateComponent`,
  `SuggestMissingCapability`.

  The prior value was incoherent: `relation: "member"` (any tenant member may
  call) combined with `allowed_identities: 8` (PLATFORM\_OPERATOR-only) caused
  every USER, SERVICE, and COMPONENT caller to be rejected at the ext-authz
  identity-class gate before the FGA check ran, surfacing as 403s on all
  dashboard Discovery-backed pages. The `relation`, `object_type`, and
  `object_deriver` fields are unchanged — tenant isolation is preserved at the
  FGA layer. `PLATFORM_OPERATOR` (bit 8) is intentionally excluded: cross-tenant
  operator inspection goes via `ImpersonateTenant`, not direct Discovery calls.

### Added

- **`TestMemberRelationIncludesUserBit` in `auth/registry/coverage_test.go`.**
  Walks `protoregistry.GlobalFiles` and asserts every RPC with
  `relation == "member"` or `"tenant_member"` (excluding COMPONENT-only
  `gibson.agent.v1` / `gibson.tool.v1` surfaces validated by the existing
  opposite-polarity guard) has USER bit (1) set in `allowed_identities`.
  Failure messages name the offending RPC and include the string
  `discovery-bitfield-coherence`.

- **`TestAdminRelationIncludesUserOrServiceBit` in `auth/registry/coverage_test.go`.**
  Same walker; asserts every `relation == "admin"` or `"tenant_admin"` RPC has
  USER (1) or SERVICE (2) bit set. Pure PLATFORM\_OPERATOR-only is not a valid
  mask for tenant-admin relations.

### Notes

- Proto wire shape is unchanged; only the semantic value of `allowed_identities`
  changes on eleven RPCs. Old ext-authz binaries running against the new
  registry artifact parse the value without error. New binaries against old
  registry continue behaving as before.
- Downstream consumers (`zeroroot-ai/gibson`) must re-run `make authz-registry`
  after bumping to this SDK version. The eleven Discovery RPCs will appear with
  `allowed_identities: [USER, SERVICE, COMPONENT]` in `registry.yaml` and the
  corresponding `audit.csv` rows.
- Spec: `discovery-bitfield-coherence` (Requirements 1, 3, 4.2).

## v0.95.0 — 2026-05-01

Introduces the `self` authz mode from spec `self-mode-authz` — a third
mutually-exclusive form in the per-RPC `gibson.auth.v1.AuthOptions`
annotation for "authenticated user reading their own data, no FGA tuple
lookup required". Closes the hotfix hole opened by `zero-trust-hardening`
without reintroducing the `unauthenticated: true` band-aid.

### Added

- **`bool self = 6` in `gibson.auth.v1.AuthOptions`.** Field 6 (next free;
  fields 1-5 are wire-locked forever). When `self: true`, Envoy's
  `jwt_authn` still runs, ext-authz skips the FGA Check call, ext-authz
  still applies the `allowed_identities` bitfield, and the daemon handler
  is responsible for scoping the response to the verified caller subject.
  Mutually exclusive with `unauthenticated: true` and with the standard rule
  form (`relation`/`object_type`/`object_deriver`). `allowed_identities`
  is required when `self: true`.

- **Three new codegen validators in `cmd/authz-registry-gen`.** (1) `self`
  AND `unauthenticated` → fail with spec-named error. (2) `self` AND any
  rule field → fail. (3) `self` without `allowed_identities` → fail. All
  error messages include the offending RPC's full method name and the string
  `self-mode-authz` for traceability.

- **`Self bool` field in the generated `Entry` struct and YAML/TS outputs.**
  The `registry.go`, `registry.yaml`, and `permissions.ts` artifacts emit a
  distinct shape for self-mode entries (`self: true` + `allowed_identities`
  in YAML; `Self: true` in Go; `self: true` in TS). The `fga_model.fga`
  artifact correctly skips self-mode entries (no FGA relation to declare).

- **`TestSelfModeIsValidForEveryRPC` in `auth/registry/coverage_test.go`.**
  Proto-reflection walk that asserts every `self: true` RPC in GlobalFiles
  has `allowed_identities` non-zero and no rule fields set. Fires on
  `go test ./auth/registry/...`.

### Changed

- **`DaemonService.GetMyPermissions` and `DaemonService.ListMyMemberships`
  migrated from `unauthenticated: true` (hotfix) to `self: true +
  allowed_identities: 1` (USER only).** The hotfix annotations are removed
  in this same release (Req 6.5 — no transitional state). Identity-class
  enforcement is restored on both RPCs: SERVICE and COMPONENT tokens can no
  longer call them.

- **`TestAnnotatedRPCMinimumCount` updated** to count self-mode entries
  alongside rule-mode and unauthenticated entries in the minimum-count
  sentinel.

### Notes

- Wire-compatible: adding field 6 is forward-compat. Old ext-authz binaries
  that receive a registry with `self: true` entries fail closed with the
  existing "missing rule fields" error (old binary, new registry = fail
  closed per design). See spec `self-mode-authz` for the release ordering
  contract (ext-authz v0.2.0 must ship before the gibson v0.24.0 registry
  artifact).
- Spec: `self-mode-authz` (Requirements 1, 2, 4).

## v0.94.0 — 2026-05-01

Phase 1 of the `component-bootstrap-e2e` spec — adds the SDK proto
surface that the daemon, ADK CLI, and dashboard will consume to ship
the per-action component-grant UX, the canonical "what can I do?"
inspection RPC, and a typed manifest kind discriminator.

### Added

- **`gibson.identity.v1` package (new).** Defines `IdentityService.WhoAmI`,
  the canonical "what can I do?" RPC. Returns the caller's effective
  component grants (per-action read/configure/execute), plugin
  invocation grants, and active capability grants. Tenant_admins may
  pass `target_principal_id` to inspect another principal in their
  tenant; the implementation rejects target lookups for non-admin
  callers.

- **`gibson.identity.v1.PrincipalKind` enum.** New canonical home for
  AGENT/TOOL/PLUGIN. Subsumes the daemon-local `PrincipalKind` in
  `tenant_admin.proto` and the SDK's `gibson.admin.v1.RecipientClass`
  — both should migrate to importing this enum over time.

- **`gibson.admin.v1.GrantsAdminService.WriteAgentGrants` and
  `DeleteAgentGrants`.** Additive write/delete of per-action FGA
  tuples (`can_read` / `can_configure` / `can_execute` /
  `can_invoke`). Idempotent — already-present writes and missing
  deletes count toward dedicated counters rather than failing. Each
  successful operation emits an audit event server-side. Validation
  enforces that the target's kind is consistent with the relation
  (only TOOL targets may receive `can_invoke`).

- **`gibson.manifest.v1.ComponentCapability.principal_kind` field.**
  Typed `PrincipalKind` discriminator alongside the legacy
  `string kind` field. The string field is deprecated and will be
  removed one minor release after v0.94.

- **YAML manifest schemas for agents and tools.** `agent/manifest/schema.json`
  and `tool/manifest/schema.json` mirror the existing `plugin/manifest/schema.json`
  shape and use the conventional `apiVersion: <kind>.gibson.zero-day.ai/v1`
  / `kind: Agent | Tool` discriminator pair. Plugin schema's `kind`
  description updated to cross-reference `PrincipalKind`.

### Notes

- All proto changes are additive; no field numbers were renumbered or
  reused. Consumers built against v0.93.0 continue to compile.
- Spec: `component-bootstrap-e2e` (Requirements 9, 10, 12).

## v0.93.0 — 2026-05-01

Reverts the `tenant_admin` / `tenant_member` rename from v0.92.0 back to
the original `admin` / `member` FGA relations on `tenant`. The OpenFGA
model and existing tuples were never migrated to the renamed relations,
and `core/gibson/internal/authz/model.fga` explicitly forbids relation
renames. v0.92.0 consumers should bump to v0.93.0+ to stay aligned with
the live FGA model.

## v0.92.0 — 2026-05-01

Security hardening from spec `zero-trust-hardening` (Requirements 2.5, 5.1, 5.2, 10.1-10.5).

### Changed

- **`agent.proto` / `tool.proto` — identity bitfield corrected to COMPONENT-only.**
  `allowed_identities` on all four `AgentService` methods and all four
  `ToolService` methods changed from `3` (USER|SERVICE) to `4` (COMPONENT).
  These are component-to-component RPC surfaces; human users and platform
  services must not call them directly. Consumers that were relying on the
  incorrect USER|SERVICE bitfield will now be rejected by ext-authz once it
  enforces the bitfield (task 2.2). Wire shape is unchanged — this is a
  policy-enforcement tightening only.

- **`daemon.proto` — `GetMyPermissions` and `ListMyMemberships` are now
  authenticated (USER-only).**
  Both RPCs previously carried `unauthenticated: true`, which allowed
  anyone reaching the daemon to enumerate "their own" permissions for an
  arbitrary caller-supplied subject (confused-deputy). They now require a
  valid user JWT and are gated on `relation: tenant_member`. `Connect` and
  `Ping` retain `unauthenticated: true`.

### Added

- **`cmd/authz-registry-gen` — three new fail-closed validators.**
  `validateObjectDeriver` (allowlist regex rejects unknown deriver names),
  `validateIdentityBits` (rejects bits outside `0xF`), and
  `detectDuplicateMethodKeys` (rejects duplicate `/<pkg>.<svc>/<method>`
  keys). These run inside the existing `Lint authz annotations` CI step
  without any CI shape change. A typo'd `object_deriver` that previously
  shipped silently and bricked ext-authz at startup is now caught at
  codegen time.

- **`auth/registry/coverage_test.go` — four new invariant tests.**
  `TestObjectDeriverIsValidForEveryRPC`, `TestIdentityBitsAreValidForEveryRPC`,
  `TestNoDuplicateMethodKeysAcrossFiles`, and `TestAgentToolServicesNoUserBit`.
  Run as part of `go test ./auth/registry/...`. Codify the codegen
  invariants at SDK test time so a contributor weakening annotation
  strictness fails CI without needing to run the full codegen pipeline.

- **`daemonclient.NewTokenCredentials` — JWT-only contract documented.**
  Doc comment now explicitly states: accepted tokens are Zitadel-issued
  JWTs (client_credentials or OIDC); opaque tokens, API keys, and
  third-party IdP tokens are rejected by Envoy jwt_authn before reaching
  the daemon. No behaviour change.

## v0.80.0 — 2026-04-26

BREAKING CHANGES — Phase 1 of `unified-identity-and-authorization` spec.
The SDK auth package is replaced wholesale: JWT validation moves out (it
is the job of Envoy + ext-authz upstream of the daemon); the package now
carries only the typed identity surface, the gRPC interceptor that reads
ext-authz–emitted headers, the TenantID sealed type, capability-grant
claims/verification, and the SPIFFE Workload API helpers.

### Added

- `gibson.auth.v1.AuthOptions` proto annotation (`api/proto/gibson/auth/v1/options.proto`)
  with `(gibson.auth.v1.authz)` MethodOptions extension at field 50001.
  Every RPC in the SDK now carries this annotation; CI fails closed on
  any new RPC missing it.

- `auth.TenantID` sealed type with `NewTenantID/MustNewTenantID`
  constructors. Validation is pattern-based and rejects empty/oversize/
  reserved/uppercase identifiers. The data-plane spec uses this as the
  connection-pool selector — handler code that forgets to thread tenant
  fails to compile.

- `auth.Identity{Subject,Issuer,CredentialType,Tenant,IssuedAt}` and
  header parser (`IdentityFromMetadata`) that consumes the
  `x-gibson-identity-*` headers ext-authz emits. NO HMAC verification
  — channel security is provided by SPIFFE-pinned mTLS between Envoy
  and the daemon.

- `auth.UnaryServerInterceptor` and `auth.StreamServerInterceptor`
  that fail-closed on missing/invalid identity headers
  (`codes.PermissionDenied`).

- `auth.WithIdentity / IdentityFromContext / TenantFromContext /
  WithTenant` context helpers. `TenantFromContext` returns
  `(TenantID, bool)` with NO `_system` fallback — closes audit
  findings C11/C12.

- `capabilitygrant.Claims` typed CG-JWT payload + `capabilitygrant.Verify`
  EdDSA-only signature/claim verifier. Used by ext-authz to verify the
  per-task tokens the Gibson daemon mints at mission dispatch.

- `spiffe.DialOptions / ServerOptions / ExpectPeerSPIFFEID` helpers
  wrapping go-spiffe/v2's Workload API. DialOptions auto-falls-back to
  plain TLS when the SPIFFE socket is absent (external agents on
  customer networks not in k8s).

- `cmd/authz-registry-gen` standalone tool that walks the proto
  FileDescriptorSet, extracts `(gibson.auth.v1.authz)` annotations,
  and emits four artifacts to `auth/registry/`:
    - `registry.go` — typed Go map (daemon startup self-check)
    - `registry.yaml` — YAML (ext-authz consumption)
    - `fga_model.fga` — OpenFGA schema 1.1
    - `permissions.ts` — TypeScript constants for the dashboard
  Wired via `make proto` (and `make lint-authz` for CI annotation
  guard).

- `auth/registry/coverage_test.go` runtime guard that asserts every
  Gibson service-descriptor method has a registry entry — fails with
  a clear "regenerate proto" message on regression.

### Changed (BREAKING)

- `auth.Validator`, `auth.UnaryInterceptor(v Validator, cfg)`,
  `auth.StreamInterceptor(v Validator, cfg)`, `auth.RoleBinder`, and
  the OIDC validation surface are removed. JWT validation belongs to
  Envoy + ext-authz; the daemon trusts identity headers because the
  channel is SPIFFE-pinned.

- `auth.Identity` slimmed to {Subject, Issuer, CredentialType, Tenant,
  IssuedAt}. The previous Email/Groups/Capabilities/Claims fields are
  removed; if downstream consumers need additional claims, they should
  be forwarded by ext-authz as additional `x-gibson-identity-*`
  headers and parsed by an extension to `IdentityFromMetadata`.

- `auth.ContextWithIdentity → auth.WithIdentity`. The new function
  takes Identity by value (not pointer); a zero-value Identity is
  reported by `IdentityFromContext` as missing rather than as a valid
  unauthenticated principal.

- `auth.TrustLocalhost` interceptor option, all `_system` fallbacks
  on missing identity, and all dev-mode bypasses are deleted. Empty
  tenant on context = `codes.PermissionDenied`. Closes audit
  C11/C12/C17.

### Removed

- Legacy `auth/{validator.go,roles.go,roles_test.go,metrics.go,
  integration_test.go,errors.go,errors_test.go,doc.go}` files. The
  doc.go is rewritten; the rest is gone.

### Notes

- The existing `capabilitygrant/{bootstrap,client,discovery,jwt,
  keypair,revocation}.go` files are NOT touched in this rev; they are
  the prior agent-host-registration model and will be replaced in
  Phase 5 (ADK refactor) by Zitadel service-account enrollment.

## v0.76.0 — 2026-04-21

BREAKING CHANGES — Harness constructors migrated to functional options (spec
`gibson-sota-patterns` Phase B). Internal concurrency modernization (Phase H).

### Changed (BREAKING)

- **`serve.NewCallbackHarness`** signature changed from
  `NewCallbackHarness(client, logger, tracer, mission, target)` (5 positional
  args) to `NewCallbackHarness(client, opts ...CallbackHarnessOption)`.
  Defaults: `slog.Default()` logger, no-op OTel tracer, empty mission/target.
  Use `WithCallbackLogger`, `WithCallbackTracer`, `WithCallbackMission`,
  `WithCallbackTarget` to customize.

- **`serve.NewPlatformHarness`** signature changed from
  `NewPlatformHarness(client, workID, mission, target, logger, tracer)`
  (6 positional args) to `NewPlatformHarness(client, opts ...PlatformHarnessOption)`.
  Defaults: `slog.Default()`, no-op tracer, empty work ID / mission / target.
  Use `WithPlatformWorkID`, `WithPlatformMission`, `WithPlatformTarget`,
  `WithPlatformLogger`, `WithPlatformTracer`.

`CallbackHarnessOption` and `PlatformHarnessOption` are exported types — they
are part of the public SDK API. Adding a new option is non-breaking; changing
a default is a minor bump; removing or retyping is a major bump.

### Changed (internal, non-breaking)

- **Concurrency** — migrated `sync.WaitGroup` fan-out patterns to
  `errgroup.WithContext` in `health/http/server.go` (`runChecks`),
  `startup/dependencies.go` (`getVersions`), `manifest/runtime.go`,
  `eval/feedback_harness.go`, `eval/langfuse.go`. Context cancellation now
  propagates to in-flight work. `tool/worker/worker.go` retains `sync.WaitGroup`
  intentionally (long-running worker pool — documented in godoc).

### Added

- `serve/callback_options.go` — `CallbackHarnessOption` type and `With*` constructors
- `serve/platform_options.go` — `PlatformHarnessOption` type and `With*` constructors
- `health/http/server_test.go` — coverage for context-propagation under errgroup
- `CLAUDE.md` "API Stability" section — documents harness options pattern and
  breaking-change versioning policy

---

## v0.75.0 — 2026-04-19

BREAKING CHANGES — Agent Auth → Capability Grant rename (spec `zitadel-envoy-gateway-migration` task 30).

### Removed

- **`github.com/zeroroot-ai/sdk/agentauth` package** — renamed to
  `github.com/zeroroot-ai/sdk/capabilitygrant`. All Go identifiers, gRPC method
  names in `DaemonAdminService`, YAML config keys, Prometheus metric names, log
  messages, and comments updated throughout. Callers must update import paths from
  `agentauth` to `capabilitygrant`.

  Migration summary:
  - `agentauth.Client` → `capabilitygrant.Client`
  - `agentauth.NewClient` → `capabilitygrant.NewClient`
  - `serve.WithAgentAuthFromEnv()` → `serve.WithCapabilityGrantFromEnv()`
  - `*PlatformClient.AgentAuthClient()` → `*PlatformClient.CapabilityGrantClient()`
  - `RegisterAgentAuth` RPC → `RegisterCapabilityGrant`
  - `GetAgentAuthStatus` RPC → `GetCapabilityGrantStatus`
  - `RevokeAgentAuth` RPC → `RevokeCapabilityGrant`
  - `ListAgentAuthAgents` RPC → `ListCapabilityGrantAgents`

  Note: the discovery proto field `is_agent_auth` (wire field 6) is NOT renamed in
  this release to preserve JSON wire-format compatibility. It will be renamed in a
  future breaking release with a reserved field tombstone.

---

## v0.74.0 — 2026-04-19

BREAKING CHANGES — Envoy Gateway migration (spec `zitadel-envoy-gateway-migration`).

### Removed

- **Kubernetes ServiceAccount credential path** — `detectCredentials` no longer
  probes the in-cluster ServiceAccount token. Component identity in platform
  mode is established via `GIBSON_API_KEY` (API key bearer token) or SPIFFE
  mTLS. K8s SA tokens were only forwarded to the daemon as an opaque bearer;
  with Envoy Gateway terminating auth, the daemon no longer accepts them.

- **`insecure.NewCredentials()` / WithInsecure transport option** — TLS is now
  mandatory for all SDK-to-daemon connections. There is no environment variable
  or option to re-enable cleartext transport. Callers that previously connected
  to a plaintext local daemon must configure TLS (see `GIBSON_DAEMON_CA` for
  custom CA PEM in Kind/dev clusters).

- **All `x-gibson-identity-*` and `x-gibson-tenant` / `x-gibson-user-id` header
  injection** — The SDK never set these headers (the audit against this release
  confirms zero injection sites). Documenting explicitly: callers must not
  attempt to forge these headers. Envoy Gateway injects the trusted
  `x-gibson-identity-*` headers on the daemon side after validating the
  credential; SDK is upstream of Envoy and must not pre-fill any of them.

### What callers must change

- **No-credential construction fails.** `daemonclient.New()` returns an error if
  no credential is detected (`GIBSON_API_KEY` env, SPIFFE socket, or
  `OIDC_CLIENT_CREDENTIALS_*` env). Previously it fell back to insecure
  transport. Set `GIBSON_API_KEY` in the caller's environment or use
  `daemonclient.NewWithCredentials` with explicit `PerRPCCredentials`.

- **Remove any `grpc.WithInsecure()` dial options.** The SDK's `dial` helper
  rejects non-TLS transport credentials.

### Retained (not a breaking change)

- `x-gibson-manifest-version` header in `manifest/authorized_call.go` — this is
  a legitimate SDK-to-daemon protocol header for manifest staleness detection,
  not an identity injection header. It is retained unchanged.

### Added

- **`github.com/zeroroot-ai/sdk/capabilitygrant` package** — full rename of
  `agentauth` → `capabilitygrant` (task 30 of spec `zitadel-envoy-gateway-migration`).
  All Go identifiers, proto message names, gRPC method names, YAML config keys, and
  log messages updated. The `agentauth` package is removed; callers must update their
  import paths to `github.com/zeroroot-ai/sdk/capabilitygrant`.

---

## v0.73.1 — 2026-04-18

- Add missing `mission_definition.pb.go` to the v0.73.0 tag.

## v0.73.0 — 2026-04-18

BREAKING — spec `mission-api-only-cleanup`.

- Renamed proto package `gibson.workflow.v1` → `gibson.mission.v1`.
- `WorkflowDefinition` → `MissionDefinition`, `WorkflowNode` → `MissionNode`,
  `WorkflowEdge` → `MissionEdge`, `WorkflowDependencies` → `MissionDependencies`.
- `CreateMission` / `RunMission` now require `mission_definition_id` + `target_id`
  only; removed `inline_target`, `inline_workflow`, `workflow_yaml`, `workflow_path`.
- Removed six dead installer/resolver RPCs and four component-installer RPCs.
- Added `CreateMissionDefinition` RPC on `DaemonService`.

## v0.72.0 — prior

See git log for earlier history.
