# Changelog

## [0.1.9](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.8...v0.1.9) (2026-08-20)


### Bug Fixes

* **theme:** define the semantic scale the shared shell is written against ([#74](https://github.com/MustardSeedNetworks/trellis/issues/74)) ([99f014d](https://github.com/MustardSeedNetworks/trellis/commit/99f014da3a2a2f863d215e0f25cd5444d9045b03)), closes [#70](https://github.com/MustardSeedNetworks/trellis/issues/70)
* **ui:** serve client routes from the binary instead of 404ing them ([#73](https://github.com/MustardSeedNetworks/trellis/issues/73)) ([38837ad](https://github.com/MustardSeedNetworks/trellis/commit/38837ad57e9bbf4d984dd3c9e4612ea973bd4296)), closes [#69](https://github.com/MustardSeedNetworks/trellis/issues/69)

## [0.1.8](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.7...v0.1.8) (2026-08-20)


### Features

* **coverage:** give the heatmap its own page, with a legend from the render scale ([#68](https://github.com/MustardSeedNetworks/trellis/issues/68)) ([9e4b40e](https://github.com/MustardSeedNetworks/trellis/commit/9e4b40e28507d61346b39bdb8bde558454df877d))
* **import:** give AirMapper ingest its own page ([#60](https://github.com/MustardSeedNetworks/trellis/issues/60)) ([748cf2e](https://github.com/MustardSeedNetworks/trellis/commit/748cf2e47aba067c960ad0b49ab8dbf97c74fb13))


### Bug Fixes

* **nav:** stop listing Reports and Exports as destinations ([#67](https://github.com/MustardSeedNetworks/trellis/issues/67)) ([2bca1cb](https://github.com/MustardSeedNetworks/trellis/commit/2bca1cb6430400c3336210c8fb7961f8581cda6e))


### Continuous Integration

* reconcile skipped releases with a 3-hourly release-please run ([#58](https://github.com/MustardSeedNetworks/trellis/issues/58)) ([eaa2c0e](https://github.com/MustardSeedNetworks/trellis/commit/eaa2c0e4a76fe5c86466b2b75516313472179d10))


### Miscellaneous

* **deps:** update github actions ([#72](https://github.com/MustardSeedNetworks/trellis/issues/72)) ([b99b251](https://github.com/MustardSeedNetworks/trellis/commit/b99b251ef8d87057b993448ab7a6d51b31f8439b))

## [0.1.7](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.6...v0.1.7) (2026-08-18)


### Features

* **ui:** give trellis the family shell and a central page header ([#55](https://github.com/MustardSeedNetworks/trellis/issues/55)) ([b69bc49](https://github.com/MustardSeedNetworks/trellis/commit/b69bc498a071cd308bdf2c0f92f7dd09fd5207d6))

## [0.1.6](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.5...v0.1.6) (2026-08-17)


### Continuous Integration

* make CI conformance a blocking gate ([#50](https://github.com/MustardSeedNetworks/trellis/issues/50)) ([895f1bb](https://github.com/MustardSeedNetworks/trellis/commit/895f1bba95816a2e0d85d34ca3e4e399b303af79))

## [0.1.5](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.4...v0.1.5) (2026-08-17)


### Continuous Integration

* pin staticcheck via go.mod tool directive ([#48](https://github.com/MustardSeedNetworks/trellis/issues/48)) ([9553576](https://github.com/MustardSeedNetworks/trellis/commit/9553576b1bbb050a316f24631a31b72cffb52ecc))
* pin the last unpinned tool installs ([#46](https://github.com/MustardSeedNetworks/trellis/issues/46)) ([0dfd037](https://github.com/MustardSeedNetworks/trellis/commit/0dfd0372054a5aa2794de03f4d99de4eeacba2e2))

## [0.1.4](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.3...v0.1.4) (2026-08-16)


### Continuous Integration

* add security linting to Trellis ([#44](https://github.com/MustardSeedNetworks/trellis/issues/44)) ([997f70e](https://github.com/MustardSeedNetworks/trellis/commit/997f70e5b9d5dbdf717ebf28354118730f6aa4f0))

## [0.1.3](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.2...v0.1.3) (2026-08-16)


### Continuous Integration

* match the fleet's CI shape ([#41](https://github.com/MustardSeedNetworks/trellis/issues/41)) ([d87415d](https://github.com/MustardSeedNetworks/trellis/commit/d87415dee35b97ccdf92190931a3544f388747ff))

## [0.1.2](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.1...v0.1.2) (2026-08-16)


### Continuous Integration

* adopt the fleet governance gates ([#38](https://github.com/MustardSeedNetworks/trellis/issues/38)) ([d4315b3](https://github.com/MustardSeedNetworks/trellis/commit/d4315b32524f4359305d81b706a1e7c0561f7e2e))


### Miscellaneous

* **release:** drop the no-op trigger-release job ([#35](https://github.com/MustardSeedNetworks/trellis/issues/35)) ([1dcf687](https://github.com/MustardSeedNetworks/trellis/commit/1dcf687a2c8548fcb7dc10c0577b32916c63f527))

## [0.1.1](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.0...v0.1.1) (2026-08-16)


### Features

* **api:** connectrpc measured-survey API + trellisd server ([#5](https://github.com/MustardSeedNetworks/trellis/issues/5)) ([badb34a](https://github.com/MustardSeedNetworks/trellis/commit/badb34aa16362740bbe7c8b72ab653c6a75278ab))
* **core:** bootstrap the Trellis Go module + shared Wi-Fi scan model ([#1](https://github.com/MustardSeedNetworks/trellis/issues/1)) ([6979d1b](https://github.com/MustardSeedNetworks/trellis/commit/6979d1b3ea561ab084a274c448106ca903c2ce30))
* **core:** lift the measured-survey engine from Seed into Trellis core ([#2](https://github.com/MustardSeedNetworks/trellis/issues/2)) ([abced41](https://github.com/MustardSeedNetworks/trellis/commit/abced4196a7dee3ecc6245ef088274150c255592))
* **survey:** single-call AirMapper import → survey → store flow ([#4](https://github.com/MustardSeedNetworks/trellis/issues/4)) ([931ab54](https://github.com/MustardSeedNetworks/trellis/commit/931ab54ac8b421efad8854ea6b38dc89c2634744))
* **ui:** download PDF survey report from the detail pane ([#9](https://github.com/MustardSeedNetworks/trellis/issues/9)) ([4896b3e](https://github.com/MustardSeedNetworks/trellis/commit/4896b3efad31f7c99d7b618f090ffe84678328d0))
* **ui:** React survey UI foundation (list, import, heatmap view) ([#7](https://github.com/MustardSeedNetworks/trellis/issues/7)) ([73fdb0e](https://github.com/MustardSeedNetworks/trellis/commit/73fdb0e2cd024b12b904620b692423ea1ecae160))


### Bug Fixes

* **api:** typed not-found errors + harden trellisd ([#6](https://github.com/MustardSeedNetworks/trellis/issues/6)) ([0dd48d8](https://github.com/MustardSeedNetworks/trellis/commit/0dd48d81c3ad5ff7edb711fa221b9d639d999383))


### Documentation

* absorb Seed survey subsystem — measured=migrate proven Go, predictive engine=only greenfield; reorder roadmap (migrate→engine→G1), add 09-SEED-MIGRATION ([05dbf77](https://github.com/MustardSeedNetworks/trellis/commit/05dbf77b99d9f6b245f32775bfdd736443c87018))
* clarify strategy is Seed-scoped; add Seed↔Trellis Wi-Fi boundary (open decision) ([789182c](https://github.com/MustardSeedNetworks/trellis/commit/789182c5c127673cd7a8e201d0d2ae80f184c5e2))
* DECIDED - all Wi-Fi moves out of Seed; Trellis is the Wi-Fi product (Live + Project modes on shared core) ([ef44a72](https://github.com/MustardSeedNetworks/trellis/commit/ef44a72e982d466ce0e57d7dd2a077e7276a7ad1))
* **roadmap:** add Gate G1 — prove engine accuracy vs ground truth before building the full stack ([0216165](https://github.com/MustardSeedNetworks/trellis/commit/02161657045aeef983286ef20d2eb6a4f0336559))


### Tests

* **survey:** end-to-end proof of the measured-survey pipeline ([#3](https://github.com/MustardSeedNetworks/trellis/issues/3)) ([312da42](https://github.com/MustardSeedNetworks/trellis/commit/312da4236452de5ef372c8ddb82940079e7800de))


### Continuous Integration

* bring Trellis to fleet CI/release parity with seed, stem, and niac ([#31](https://github.com/MustardSeedNetworks/trellis/issues/31)) ([069ed18](https://github.com/MustardSeedNetworks/trellis/commit/069ed18e78880a1fb037363dbf9b69ee79382c64))
* cover the UI + wire the fleet Semgrep gate ([#8](https://github.com/MustardSeedNetworks/trellis/issues/8)) ([c53be34](https://github.com/MustardSeedNetworks/trellis/commit/c53be340c6bfca88d74dee0241e3208a5b71280e))


### Miscellaneous

* **deps:** adopt TypeScript 7 and align tooling with the fleet ([#18](https://github.com/MustardSeedNetworks/trellis/issues/18)) ([facb64a](https://github.com/MustardSeedNetworks/trellis/commit/facb64a3988b9e02802740c514acbb89b595c8d4))
* **deps:** pin actions/checkout to v7.0.1 ([#30](https://github.com/MustardSeedNetworks/trellis/issues/30)) ([2494417](https://github.com/MustardSeedNetworks/trellis/commit/24944170689843e79a184b8fbd31be943a2db2a1))
* **deps:** update actions/checkout action to v7.0.1 ([#22](https://github.com/MustardSeedNetworks/trellis/issues/22)) ([0fd7e5a](https://github.com/MustardSeedNetworks/trellis/commit/0fd7e5ad6d9fa50646fd97ec07e27b022b8a461e))
