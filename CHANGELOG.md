# Changelog

## [0.2.20](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.19...v0.2.20) (2026-09-04)


### Features

* **survey:** measure throughput at a point and map it as its own layer ([#304](https://github.com/MustardSeedNetworks/trellis/issues/304)) ([9146222](https://github.com/MustardSeedNetworks/trellis/commit/914622214fd479d5281409089e78cb4f2fffdca0))
* **survey:** upload a floor plan and calibrate what a pixel of it is worth ([#306](https://github.com/MustardSeedNetworks/trellis/issues/306)) ([a833c10](https://github.com/MustardSeedNetworks/trellis/commit/a833c10b79030dacc6b928cf72bdbd63011e6735))


### Continuous Integration

* arm auto-merge on the release PR ([#309](https://github.com/MustardSeedNetworks/trellis/issues/309)) ([040cbd4](https://github.com/MustardSeedNetworks/trellis/commit/040cbd4c6937fd8759d1b03c5e3cfa0df926077a)), closes [#308](https://github.com/MustardSeedNetworks/trellis/issues/308)
* collapse the file-size gate to one enforced limit ([#311](https://github.com/MustardSeedNetworks/trellis/issues/311)) ([94f04ac](https://github.com/MustardSeedNetworks/trellis/commit/94f04ac7e4c61c25ea2b2e6a5d566acf62b7de25)), closes [#310](https://github.com/MustardSeedNetworks/trellis/issues/310)
* wire the shared dependency-review gate into CI ([#288](https://github.com/MustardSeedNetworks/trellis/issues/288)) ([b2bd0d1](https://github.com/MustardSeedNetworks/trellis/commit/b2bd0d1924e10815b64df96f31d9ff2826211079)), closes [#287](https://github.com/MustardSeedNetworks/trellis/issues/287)

## [0.2.19](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.18...v0.2.19) (2026-09-04)


### Features

* **api:** serve a survey's floors and scope the heatmap to one of them ([#282](https://github.com/MustardSeedNetworks/trellis/issues/282)) ([d77e013](https://github.com/MustardSeedNetworks/trellis/commit/d77e0134f792ecf25ec9f012087b2328f3346d95)), closes [#65](https://github.com/MustardSeedNetworks/trellis/issues/65)
* **coverage:** read the measured value under the pointer and zoom the surface ([#286](https://github.com/MustardSeedNetworks/trellis/issues/286)) ([5a1cf7b](https://github.com/MustardSeedNetworks/trellis/commit/5a1cf7b15a6f4bb34c4a5729b8b2b8e7bacd916a))
* **live:** read the airspace this host hears, without recording a survey ([#296](https://github.com/MustardSeedNetworks/trellis/issues/296)) ([80efab8](https://github.com/MustardSeedNetworks/trellis/commit/80efab8bdf696f223ed90f76c58bca16c96ef608))
* **survey:** place a walk's readings along the way between two marks ([#301](https://github.com/MustardSeedNetworks/trellis/issues/301)) ([9b9d5c9](https://github.com/MustardSeedNetworks/trellis/commit/9b9d5c9183ae4d70e5896cdace4c07a089f8d5d6)), closes [#300](https://github.com/MustardSeedNetworks/trellis/issues/300)
* **survey:** walk a floor continuously instead of stopping at every point ([#299](https://github.com/MustardSeedNetworks/trellis/issues/299)) ([71d0b29](https://github.com/MustardSeedNetworks/trellis/commit/71d0b29b875a5b46f87832885c50133fa348ac8f)), closes [#298](https://github.com/MustardSeedNetworks/trellis/issues/298)
* **ui:** wire the survey page to the walk lifecycle the service already serves ([#268](https://github.com/MustardSeedNetworks/trellis/issues/268)) ([d2db80b](https://github.com/MustardSeedNetworks/trellis/commit/d2db80b9d40bab250e70507bf435e43a69f99ca8)), closes [#267](https://github.com/MustardSeedNetworks/trellis/issues/267)


### Bug Fixes

* **survey:** cap what one AirMapper archive entry may inflate to ([#272](https://github.com/MustardSeedNetworks/trellis/issues/272)) ([852bb0e](https://github.com/MustardSeedNetworks/trellis/commit/852bb0e2eacb1135578b080c04c2bc039e569da0)), closes [#271](https://github.com/MustardSeedNetworks/trellis/issues/271)
* **trellisd:** refuse a non-loopback TRELLIS_ADDR at startup ([#270](https://github.com/MustardSeedNetworks/trellis/issues/270)) ([0d2fbdb](https://github.com/MustardSeedNetworks/trellis/commit/0d2fbdb7f924c55fba82c650a2923f544a64a857))


### Performance Improvements

* **ui:** load each page lazily so the entry chunk stops carrying all four ([#281](https://github.com/MustardSeedNetworks/trellis/issues/281)) ([6027252](https://github.com/MustardSeedNetworks/trellis/commit/6027252cb4832bac8daac748375af1d0434e260c))


### Documentation

* state what exists as of 2026-09-03 and the Seed/Trellis Wi-Fi decision ([#280](https://github.com/MustardSeedNetworks/trellis/issues/280)) ([dc4937c](https://github.com/MustardSeedNetworks/trellis/commit/dc4937c3460be6568466b523aae2979d87424724))


### Tests

* **e2e:** import, threshold and report journeys through the daemon ([#274](https://github.com/MustardSeedNetworks/trellis/issues/274)) ([90e8051](https://github.com/MustardSeedNetworks/trellis/commit/90e8051fa7b017ff1208ba28afab8a3bfc4550cb))


### Continuous Integration

* block on token discipline and add the Storybook interaction/a11y gate ([#275](https://github.com/MustardSeedNetworks/trellis/issues/275)) ([ee5a0bd](https://github.com/MustardSeedNetworks/trellis/commit/ee5a0bd179e381c28a8aec14946a35cda703d7d3)), closes [#145](https://github.com/MustardSeedNetworks/trellis/issues/145)
* gate workflow changes on actionlint and zizmor ([#292](https://github.com/MustardSeedNetworks/trellis/issues/292)) ([80303ea](https://github.com/MustardSeedNetworks/trellis/commit/80303ea20f0d774763bb2803694df5d05b706e28)), closes [#291](https://github.com/MustardSeedNetworks/trellis/issues/291)
* give each main commit its own concurrency group ([#279](https://github.com/MustardSeedNetworks/trellis/issues/279)) ([fd2b20b](https://github.com/MustardSeedNetworks/trellis/commit/fd2b20bb1a97c572ca52c613b6fc76f8a92c2bef)), closes [#278](https://github.com/MustardSeedNetworks/trellis/issues/278)


### Miscellaneous

* **deps:** lock file maintenance ([#269](https://github.com/MustardSeedNetworks/trellis/issues/269)) ([9d2a90c](https://github.com/MustardSeedNetworks/trellis/commit/9d2a90cb0f3cb0a83b981af5bd7c7dcafbd2df7c))
* **deps:** lock file maintenance ([#277](https://github.com/MustardSeedNetworks/trellis/issues/277)) ([4a9201d](https://github.com/MustardSeedNetworks/trellis/commit/4a9201dea54e10f6e352a046e8c2145dcc81ddd0))
* **deps:** lock file maintenance ([#285](https://github.com/MustardSeedNetworks/trellis/issues/285)) ([7ddf706](https://github.com/MustardSeedNetworks/trellis/commit/7ddf706a47398a078b79b80f6f3d8c2cda0e43a8))
* **deps:** lock file maintenance ([#289](https://github.com/MustardSeedNetworks/trellis/issues/289)) ([ea39da0](https://github.com/MustardSeedNetworks/trellis/commit/ea39da0764b3fb10ea5ad41d2a44b6b710a3b355))
* **deps:** lock file maintenance ([#293](https://github.com/MustardSeedNetworks/trellis/issues/293)) ([63d842c](https://github.com/MustardSeedNetworks/trellis/commit/63d842c1da579a00563900d9e05e477f078e85a5))
* **deps:** lock file maintenance ([#302](https://github.com/MustardSeedNetworks/trellis/issues/302)) ([1061e00](https://github.com/MustardSeedNetworks/trellis/commit/1061e007e18795d37964eba7badcf2c791ea4a72))
* **deps:** update dependency @biomejs/biome to v2.5.11 ([#265](https://github.com/MustardSeedNetworks/trellis/issues/265)) ([09189a2](https://github.com/MustardSeedNetworks/trellis/commit/09189a2fb00ec3e8eea80fee2ab190c0f5cb133c))
* **deps:** update dependency @biomejs/biome to v2.5.11 ([#276](https://github.com/MustardSeedNetworks/trellis/issues/276)) ([ffdc941](https://github.com/MustardSeedNetworks/trellis/commit/ffdc9412825e9b76dbd8a0e59a1f384f3c6c6f44))
* **deps:** update dependency @vitejs/plugin-react to v6.1.1 ([#297](https://github.com/MustardSeedNetworks/trellis/issues/297)) ([33a2c3b](https://github.com/MustardSeedNetworks/trellis/commit/33a2c3bc1d16fd568638b0aaf0c1edcd498da05b))
* refresh Go dependencies and fix Biome schema drift ([#264](https://github.com/MustardSeedNetworks/trellis/issues/264)) ([f98e2b9](https://github.com/MustardSeedNetworks/trellis/commit/f98e2b94e5bc89247478d7ab20d24959115c6931)), closes [#263](https://github.com/MustardSeedNetworks/trellis/issues/263)

## [0.2.18](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.17...v0.2.18) (2026-09-01)


### Bug Fixes

* **ci:** make the issue-title gate clear itself and accept the house style ([#262](https://github.com/MustardSeedNetworks/trellis/issues/262)) ([aa1f70e](https://github.com/MustardSeedNetworks/trellis/commit/aa1f70edbd46f520431be0ae4519fee28cbb1e3f))


### Continuous Integration

* never cancel a CI run on main ([#260](https://github.com/MustardSeedNetworks/trellis/issues/260)) ([33c1595](https://github.com/MustardSeedNetworks/trellis/commit/33c1595e0cd78de1152ac760fb29bc2774ae9f4a))

## [0.2.17](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.16...v0.2.17) (2026-08-31)


### Bug Fixes

* **deps:** update dependency @tanstack/react-query to v5.102.5 ([#225](https://github.com/MustardSeedNetworks/trellis/issues/225)) ([090ab13](https://github.com/MustardSeedNetworks/trellis/commit/090ab1375c9f8d6099f4324e3f461a5ade204849))
* **deps:** update dependency @tanstack/react-query to v5.102.6 ([#233](https://github.com/MustardSeedNetworks/trellis/issues/233)) ([a652cbe](https://github.com/MustardSeedNetworks/trellis/commit/a652cbee8f6cbfe3efd4d6b88c750667bb4a7f54))
* **deps:** update dependency @tanstack/react-query to v5.102.7 ([#237](https://github.com/MustardSeedNetworks/trellis/issues/237)) ([06b268c](https://github.com/MustardSeedNetworks/trellis/commit/06b268cabf59a1b505b53f42af83c927bd04c7ee))
* **deps:** update dependency @tanstack/react-query to v5.102.8 ([#242](https://github.com/MustardSeedNetworks/trellis/issues/242)) ([5ffa551](https://github.com/MustardSeedNetworks/trellis/commit/5ffa5510b85932c1e857217ebb0c9c60c6e7cd8d))


### Documentation

* add a security policy that leads with the unauthenticated daemon ([#253](https://github.com/MustardSeedNetworks/trellis/issues/253)) ([ae6d9af](https://github.com/MustardSeedNetworks/trellis/commit/ae6d9afff0a23cc268316baeb36182a332d85863))
* reconcile CI.md with the pipeline it describes ([#255](https://github.com/MustardSeedNetworks/trellis/issues/255)) ([a165252](https://github.com/MustardSeedNetworks/trellis/commit/a16525279eb51c42abd2547173c55c7b041f01c0))


### Tests

* **capture:** fail a cadence run that completed one scan ([#251](https://github.com/MustardSeedNetworks/trellis/issues/251)) ([1aff054](https://github.com/MustardSeedNetworks/trellis/commit/1aff0548f35fd89d0db9a1f033981b6b52b56e23))
* **survey:** cover the .SurveyResult decoder and stop its oracle self-disabling ([#230](https://github.com/MustardSeedNetworks/trellis/issues/230)) ([dec4d92](https://github.com/MustardSeedNetworks/trellis/commit/dec4d92c51db143579aeac896a647f10324a29aa)), closes [#229](https://github.com/MustardSeedNetworks/trellis/issues/229)
* turn off Node's unused webstorage global instead of warning per file ([#228](https://github.com/MustardSeedNetworks/trellis/issues/228)) ([a198a93](https://github.com/MustardSeedNetworks/trellis/commit/a198a931df4105236b115f9c5ca327d6db59b01b)), closes [#227](https://github.com/MustardSeedNetworks/trellis/issues/227)
* **ui:** cover the survey pages and gate frontend coverage ([#232](https://github.com/MustardSeedNetworks/trellis/issues/232)) ([9b7648e](https://github.com/MustardSeedNetworks/trellis/commit/9b7648e773589b0d5b94c11dc43c7489f882c3da))


### Continuous Integration

* collect Go coverage and enforce an anti-regression floor ([#236](https://github.com/MustardSeedNetworks/trellis/issues/236)) ([5cae57f](https://github.com/MustardSeedNetworks/trellis/commit/5cae57fbe4dd292c49f1198c9626a21cc144aa74)), closes [#235](https://github.com/MustardSeedNetworks/trellis/issues/235)
* fail the build on open High CodeQL alerts ([#239](https://github.com/MustardSeedNetworks/trellis/issues/239)) ([cbfc530](https://github.com/MustardSeedNetworks/trellis/commit/cbfc53004904f6074b3c9169230f42c7c906fe0a)), closes [#238](https://github.com/MustardSeedNetworks/trellis/issues/238)
* make release builds reproducible ([#257](https://github.com/MustardSeedNetworks/trellis/issues/257)) ([34a3696](https://github.com/MustardSeedNetworks/trellis/commit/34a3696ed1f0693cc582c021bd1b394158db89aa))
* pin the dead-code analysis tools ([#244](https://github.com/MustardSeedNetworks/trellis/issues/244)) ([a8aaabb](https://github.com/MustardSeedNetworks/trellis/commit/a8aaabb99fa950e767626ec5f4bd98f2220cb407)), closes [#243](https://github.com/MustardSeedNetworks/trellis/issues/243)
* stop checkout persisting credentials into the workspace ([#241](https://github.com/MustardSeedNetworks/trellis/issues/241)) ([58c12ea](https://github.com/MustardSeedNetworks/trellis/commit/58c12eaa79a6ddb2f722504dfd132043140b4411)), closes [#240](https://github.com/MustardSeedNetworks/trellis/issues/240)


### Miscellaneous

* **deps:** lock file maintenance ([#245](https://github.com/MustardSeedNetworks/trellis/issues/245)) ([2f31550](https://github.com/MustardSeedNetworks/trellis/commit/2f315500c9b5abb49fc0281f35746f1969207b1f))
* **deps:** update dependency @biomejs/biome to v2.5.11 ([#249](https://github.com/MustardSeedNetworks/trellis/issues/249)) ([644d81f](https://github.com/MustardSeedNetworks/trellis/commit/644d81f8f9b946649958dfc5e1633dc2b78b58fe))
* **deps:** update dependency @testing-library/react to v16.3.3 ([#246](https://github.com/MustardSeedNetworks/trellis/issues/246)) ([98a0e33](https://github.com/MustardSeedNetworks/trellis/commit/98a0e339bed83a825efabfaf4e3d75f479bd7d94))
* **deps:** update dependency @types/node to v26.4.0 ([#234](https://github.com/MustardSeedNetworks/trellis/issues/234)) ([ca3d06d](https://github.com/MustardSeedNetworks/trellis/commit/ca3d06d6c1afe63777de409ec1fdb42f44356b8f))
* **npm:** soak package releases for seven days before resolving them ([#248](https://github.com/MustardSeedNetworks/trellis/issues/248)) ([9d6cbcf](https://github.com/MustardSeedNetworks/trellis/commit/9d6cbcfbfa0c7c592310e253a0d3faa6b8c17e8e))

## [0.2.16](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.15...v0.2.16) (2026-08-29)


### Tests

* **survey:** assert heatmap and dead-zone values, not just non-nil ([#223](https://github.com/MustardSeedNetworks/trellis/issues/223)) ([8add777](https://github.com/MustardSeedNetworks/trellis/commit/8add777861730db3a5a669b60b550c06589759d8)), closes [#222](https://github.com/MustardSeedNetworks/trellis/issues/222)

## [0.2.15](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.14...v0.2.15) (2026-08-28)


### Tests

* add browser coverage against the daemon-served UI ([#214](https://github.com/MustardSeedNetworks/trellis/issues/214)) ([d1334f2](https://github.com/MustardSeedNetworks/trellis/commit/d1334f2e146afe50a69176cbd83480f51f3e9ae5))

## [0.2.14](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.13...v0.2.14) (2026-08-28)


### Continuous Integration

* bring security scanning to parity with the sibling repos ([#217](https://github.com/MustardSeedNetworks/trellis/issues/217)) ([701973b](https://github.com/MustardSeedNetworks/trellis/commit/701973bf3631b9182db2acda53b03f73c07f7c98)), closes [#216](https://github.com/MustardSeedNetworks/trellis/issues/216)

## [0.2.13](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.12...v0.2.13) (2026-08-28)


### Tests

* split survey tests and assert what ListSurveys returns ([#211](https://github.com/MustardSeedNetworks/trellis/issues/211)) ([d342e75](https://github.com/MustardSeedNetworks/trellis/commit/d342e75ce4a6ee2721e5907c2cf629dc5f3ebb65)), closes [#210](https://github.com/MustardSeedNetworks/trellis/issues/210)

## [0.2.12](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.11...v0.2.12) (2026-08-28)


### Tests

* stop i18next debug logging in the test run ([#207](https://github.com/MustardSeedNetworks/trellis/issues/207)) ([d1abed0](https://github.com/MustardSeedNetworks/trellis/commit/d1abed0c2a061e9d5eba83c280a4376a592b38f9)), closes [#206](https://github.com/MustardSeedNetworks/trellis/issues/206)


### Miscellaneous

* **deps:** lock file maintenance ([#208](https://github.com/MustardSeedNetworks/trellis/issues/208)) ([c7713bc](https://github.com/MustardSeedNetworks/trellis/commit/c7713bc3fabc3db81104fd3e471a3b0f2f6483fe))

## [0.2.11](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.10...v0.2.11) (2026-08-28)


### Continuous Integration

* use the App Client ID instead of the deprecated app-id input ([#200](https://github.com/MustardSeedNetworks/trellis/issues/200)) ([42e30fb](https://github.com/MustardSeedNetworks/trellis/commit/42e30fbe1c1739ae34f524d6650dbf14f8bdfefe))


### Miscellaneous

* **deps:** lock file maintenance ([#204](https://github.com/MustardSeedNetworks/trellis/issues/204)) ([0b7b60d](https://github.com/MustardSeedNetworks/trellis/commit/0b7b60d13ff491dd0e2e2414e228ccd48098a36e))

## [0.2.10](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.9...v0.2.10) (2026-08-28)


### Miscellaneous

* **deps:** lock file maintenance ([#202](https://github.com/MustardSeedNetworks/trellis/issues/202)) ([020d83e](https://github.com/MustardSeedNetworks/trellis/commit/020d83e60f3cffd0c8c60d5625963d9b6cfe720c))
* **deps:** update github actions ([#201](https://github.com/MustardSeedNetworks/trellis/issues/201)) ([5182b55](https://github.com/MustardSeedNetworks/trellis/commit/5182b55e16d1315a380863af2ca9dadc53f1e7d5))

## [0.2.9](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.8...v0.2.9) (2026-08-27)


### Bug Fixes

* **deps:** update dependency @tanstack/react-query to v5.102.3 ([#195](https://github.com/MustardSeedNetworks/trellis/issues/195)) ([e00789d](https://github.com/MustardSeedNetworks/trellis/commit/e00789da71d1e5fa0c59d946e9a5ec43a8c5ba38))


### Miscellaneous

* **deps:** lock file maintenance ([#197](https://github.com/MustardSeedNetworks/trellis/issues/197)) ([9714003](https://github.com/MustardSeedNetworks/trellis/commit/97140030ce28e981c57744e16e38921dd3367926))
* **deps:** lock file maintenance ([#198](https://github.com/MustardSeedNetworks/trellis/issues/198)) ([f736ceb](https://github.com/MustardSeedNetworks/trellis/commit/f736cebfef2c8f19f2123400d953c7523a448552))
* **deps:** update dependency @types/node to v26.3.0 ([#196](https://github.com/MustardSeedNetworks/trellis/issues/196)) ([e3cd6e4](https://github.com/MustardSeedNetworks/trellis/commit/e3cd6e49ae03d408972576ca2a86c210dd95d6a1))

## [0.2.8](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.7...v0.2.8) (2026-08-27)


### Continuous Integration

* adopt shared-workflow v1.8.0 ([#186](https://github.com/MustardSeedNetworks/trellis/issues/186)) ([efd3046](https://github.com/MustardSeedNetworks/trellis/commit/efd30467eaba6df62ac3c2a795244624fe382866))
* make .nvmrc the only source for the Node version ([#188](https://github.com/MustardSeedNetworks/trellis/issues/188)) ([9322e7f](https://github.com/MustardSeedNetworks/trellis/commit/9322e7f5bfa424fe40f72b5ac117854136c7013b))
* refuse to release a tag whose commit never passed CI ([#190](https://github.com/MustardSeedNetworks/trellis/issues/190)) ([da8b21e](https://github.com/MustardSeedNetworks/trellis/commit/da8b21e09ba36928913d5bc9b929b214c6fffb9f))


### Miscellaneous

* **deps:** lock file maintenance ([#182](https://github.com/MustardSeedNetworks/trellis/issues/182)) ([8ce3621](https://github.com/MustardSeedNetworks/trellis/commit/8ce36217a5c8b326e4553954bb2859923e361f68))
* **deps:** move golang.org/x/mod off the advisory versions, stop passing on no tests ([#193](https://github.com/MustardSeedNetworks/trellis/issues/193)) ([cac7cc2](https://github.com/MustardSeedNetworks/trellis/commit/cac7cc2f35664b6f126fb4c59ae3137fc66461c8))

## [0.2.7](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.6...v0.2.7) (2026-08-27)


### Bug Fixes

* **deps:** update dependency lucide-react to v1.34.0 ([#180](https://github.com/MustardSeedNetworks/trellis/issues/180)) ([1b6a445](https://github.com/MustardSeedNetworks/trellis/commit/1b6a445edab1af8b007c673b1ad34f3baebf45b6))

## [0.2.6](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.5...v0.2.6) (2026-08-27)


### Bug Fixes

* **deps:** update dependency @tanstack/react-query to v5.102.1 ([#171](https://github.com/MustardSeedNetworks/trellis/issues/171)) ([02a2b5b](https://github.com/MustardSeedNetworks/trellis/commit/02a2b5b92cb54544a468e014c12be6d672c5833b))
* **deps:** update dependency @tanstack/react-query to v5.102.2 ([#176](https://github.com/MustardSeedNetworks/trellis/issues/176)) ([ef0daae](https://github.com/MustardSeedNetworks/trellis/commit/ef0daaea3917d1f5319c8e45cbd029aff0d72616))


### Continuous Integration

* **dead-code:** make one step actually gate, on reachability ([#175](https://github.com/MustardSeedNetworks/trellis/issues/175)) ([f8d4080](https://github.com/MustardSeedNetworks/trellis/commit/f8d40807e9db2d6a0554e206e97e13595d2e279d))


### Miscellaneous

* **deps:** lock file maintenance ([#173](https://github.com/MustardSeedNetworks/trellis/issues/173)) ([6ddb096](https://github.com/MustardSeedNetworks/trellis/commit/6ddb096e6659794e3a46c63e5fd3c0366e9e7fd7))
* **deps:** update dependency @types/react-dom to v19.2.5 ([#178](https://github.com/MustardSeedNetworks/trellis/issues/178)) ([59e0c04](https://github.com/MustardSeedNetworks/trellis/commit/59e0c0488100dcb79a40ebc488895d923065c2c0))
* **deps:** update node.js to v26.8.0 ([#172](https://github.com/MustardSeedNetworks/trellis/issues/172)) ([1a86337](https://github.com/MustardSeedNetworks/trellis/commit/1a863372635bf75d1612bfca7856a640bb519b71))
* **deps:** update node.js to v26.8.1 ([#179](https://github.com/MustardSeedNetworks/trellis/issues/179)) ([b208e70](https://github.com/MustardSeedNetworks/trellis/commit/b208e70817889dc21d2a8fd43a7f351ae76036e0))

## [0.2.5](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.4...v0.2.5) (2026-08-26)


### Bug Fixes

* **ci:** request the workflows scope for the release-please token ([#168](https://github.com/MustardSeedNetworks/trellis/issues/168)) ([af0ead4](https://github.com/MustardSeedNetworks/trellis/commit/af0ead4d49d2b6a0dbb75aab94a857379ac94746))


### Miscellaneous

* **deps:** lock file maintenance ([#169](https://github.com/MustardSeedNetworks/trellis/issues/169)) ([4e51037](https://github.com/MustardSeedNetworks/trellis/commit/4e51037849797de83bddb9ad8dd3c8e544446e27))

## [0.2.4](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.3...v0.2.4) (2026-08-25)


### Bug Fixes

* **deps:** update dependency @tanstack/react-query to v5.102.0 ([#165](https://github.com/MustardSeedNetworks/trellis/issues/165)) ([2d58260](https://github.com/MustardSeedNetworks/trellis/commit/2d582608923ae609ab05dbb2cc5a683f8c89c65a))

## [0.2.3](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.2...v0.2.3) (2026-08-25)


### Miscellaneous

* **trellisd:** take 8446 as the canonical port instead of generic 8080 ([#161](https://github.com/MustardSeedNetworks/trellis/issues/161)) ([bc79559](https://github.com/MustardSeedNetworks/trellis/commit/bc79559723f7628b0f23a0c404a4887c5f4073f2)), closes [#159](https://github.com/MustardSeedNetworks/trellis/issues/159)

## [0.2.2](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.1...v0.2.2) (2026-08-25)


### Bug Fixes

* **trellisd:** walk +1..+9 when the listening port is taken ([#156](https://github.com/MustardSeedNetworks/trellis/issues/156)) ([dd2e527](https://github.com/MustardSeedNetworks/trellis/commit/dd2e5278435050aaaf4aed03cc39ae53b1bce7ca)), closes [#151](https://github.com/MustardSeedNetworks/trellis/issues/151)

## [0.2.1](https://github.com/MustardSeedNetworks/trellis/compare/v0.2.0...v0.2.1) (2026-08-25)


### Features

* **capture:** read the radio on Linux and Windows ([#147](https://github.com/MustardSeedNetworks/trellis/issues/147)) ([7d2fc73](https://github.com/MustardSeedNetworks/trellis/commit/7d2fc73ab9d752df2f83ccdd0e252619ed53e35d))

## [0.2.0](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.29...v0.2.0) (2026-08-25)


### ⚠ BREAKING CHANGES

* **capture:** link Wi-Fi capture into trellisd and bundle the daemon ([#138](https://github.com/MustardSeedNetworks/trellis/issues/138))

### Code Refactoring

* **capture:** link Wi-Fi capture into trellisd and bundle the daemon ([#138](https://github.com/MustardSeedNetworks/trellis/issues/138)) ([2ceec49](https://github.com/MustardSeedNetworks/trellis/commit/2ceec4978ac35eec9a473ffd85d202a1d0ac6406))


### Documentation

* **capture:** state Tier 1 privilege per platform, not as a blanket ([#141](https://github.com/MustardSeedNetworks/trellis/issues/141)) ([d6939fe](https://github.com/MustardSeedNetworks/trellis/commit/d6939fef9455aa5046a60a4557d263c4a8a446fd))


### Miscellaneous

* **deps:** lock file maintenance ([#139](https://github.com/MustardSeedNetworks/trellis/issues/139)) ([47396a7](https://github.com/MustardSeedNetworks/trellis/commit/47396a72887fe8aff661c9c655abaaa9fde9fa94))

## [0.1.29](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.28...v0.1.29) (2026-08-24)


### Features

* **capture:** add the macOS host-NIC Wi-Fi capture backend ([#132](https://github.com/MustardSeedNetworks/trellis/issues/132)) ([3e02bd5](https://github.com/MustardSeedNetworks/trellis/commit/3e02bd5df260ccb7b63258489e41a8030698ca20))

## [0.1.28](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.27...v0.1.28) (2026-08-24)


### Miscellaneous

* **deps:** lock file maintenance ([#134](https://github.com/MustardSeedNetworks/trellis/issues/134)) ([9c73d36](https://github.com/MustardSeedNetworks/trellis/commit/9c73d36851b63de7411364e180ca4c19dafff620))

## [0.1.27](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.26...v0.1.27) (2026-08-23)


### Bug Fixes

* **ui:** name the chosen file on the import page, and give the app a favicon ([#128](https://github.com/MustardSeedNetworks/trellis/issues/128)) ([634999d](https://github.com/MustardSeedNetworks/trellis/commit/634999deeeb1cdac15644f1b445cac3034ba616e)), closes [#126](https://github.com/MustardSeedNetworks/trellis/issues/126)

## [0.1.26](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.25...v0.1.26) (2026-08-23)


### Bug Fixes

* **ui:** call the origin the page was served from, not a hardcoded 127.0.0.1:8080 ([#125](https://github.com/MustardSeedNetworks/trellis/issues/125)) ([acc3593](https://github.com/MustardSeedNetworks/trellis/commit/acc35933c798da2eac33a6864508d0e8b695165b)), closes [#124](https://github.com/MustardSeedNetworks/trellis/issues/124)


### Miscellaneous

* **deps:** move to Go 1.27.0 ([#119](https://github.com/MustardSeedNetworks/trellis/issues/119)) ([f140023](https://github.com/MustardSeedNetworks/trellis/commit/f1400231ef9c099442153a3a2d3368c4f3b175d8))

## [0.1.25](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.24...v0.1.25) (2026-08-23)


### Bug Fixes

* **report:** put the coverage map in the report ([#122](https://github.com/MustardSeedNetworks/trellis/issues/122)) ([25cc1e5](https://github.com/MustardSeedNetworks/trellis/commit/25cc1e575b10522ac0af1ac065e0dc32ad3dadda)), closes [#120](https://github.com/MustardSeedNetworks/trellis/issues/120)

## [0.1.24](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.23...v0.1.24) (2026-08-23)


### Bug Fixes

* **heatmap:** draw the coverage map on the floor plan ([#118](https://github.com/MustardSeedNetworks/trellis/issues/118)) ([237b040](https://github.com/MustardSeedNetworks/trellis/commit/237b04002bbe05455fd80aa21c5076346a46fab3)), closes [#117](https://github.com/MustardSeedNetworks/trellis/issues/117)

## [0.1.23](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.22...v0.1.23) (2026-08-23)


### Bug Fixes

* **survey:** order scanned networks strongest-first, so the heatmap reads the serving AP ([#114](https://github.com/MustardSeedNetworks/trellis/issues/114)) ([8a6960d](https://github.com/MustardSeedNetworks/trellis/commit/8a6960dbfb09c9906c60a66c97ff786e3a807b29))

## [0.1.22](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.21...v0.1.22) (2026-08-23)


### Features

* **import:** read active surveys too, and persist what they record ([#112](https://github.com/MustardSeedNetworks/trellis/issues/112)) ([3ac76c2](https://github.com/MustardSeedNetworks/trellis/commit/3ac76c22bfcc937afde9da2b33504919bc1e122e))

## [0.1.21](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.20...v0.1.21) (2026-08-23)


### Features

* **survey:** store surveys in SQLite instead of one JSON file each ([#109](https://github.com/MustardSeedNetworks/trellis/issues/109)) ([9b992f9](https://github.com/MustardSeedNetworks/trellis/commit/9b992f9176b9936830a8e36dcc1f1dd64c8c1c48))

## [0.1.20](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.19...v0.1.20) (2026-08-22)


### Miscellaneous

* **deps:** lock file maintenance ([#107](https://github.com/MustardSeedNetworks/trellis/issues/107)) ([92477c6](https://github.com/MustardSeedNetworks/trellis/commit/92477c625f9d52a8fcea9060f66083c84826165e))

## [0.1.19](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.18...v0.1.19) (2026-08-22)


### Continuous Integration

* serialise Release Please so concurrent runs stop racing the ref ([#105](https://github.com/MustardSeedNetworks/trellis/issues/105)) ([3a7d0cf](https://github.com/MustardSeedNetworks/trellis/commit/3a7d0cf614f70a902fcc4fb333474c1fbc410991))

## [0.1.18](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.17...v0.1.18) (2026-08-22)


### Bug Fixes

* **deps:** update dependency lucide-react to v1.33.0 ([#103](https://github.com/MustardSeedNetworks/trellis/issues/103)) ([7de9d25](https://github.com/MustardSeedNetworks/trellis/commit/7de9d25181b2b40d162a72717fa321c69a21a868))


### Miscellaneous

* **deps:** update dependency @biomejs/biome to v2.5.10 ([#101](https://github.com/MustardSeedNetworks/trellis/issues/101)) ([7e05895](https://github.com/MustardSeedNetworks/trellis/commit/7e058953295463e325d52f31a7d4ed8cdd05a703))
* **deps:** update dependency @vitejs/plugin-react to v6.1.0 ([#20](https://github.com/MustardSeedNetworks/trellis/issues/20)) ([6b053fc](https://github.com/MustardSeedNetworks/trellis/commit/6b053fc01ff025d0176ba292462e536439ca213d))
* **deps:** update frontend toolchain ([#102](https://github.com/MustardSeedNetworks/trellis/issues/102)) ([6b7abce](https://github.com/MustardSeedNetworks/trellis/commit/6b7abceb03a8b6bad1647907b1290234ab43d2a6))

## [0.1.17](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.16...v0.1.17) (2026-08-22)


### Bug Fixes

* **deps:** let npm regenerate the lockfile, which unblocks Renovate ([#98](https://github.com/MustardSeedNetworks/trellis/issues/98)) ([e523073](https://github.com/MustardSeedNetworks/trellis/commit/e52307321fa39a0871c3872c5fa029d9fdaec650))

## [0.1.16](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.15...v0.1.16) (2026-08-22)


### Continuous Integration

* exempt bots from the issue-title lint ([#96](https://github.com/MustardSeedNetworks/trellis/issues/96)) ([86e5a13](https://github.com/MustardSeedNetworks/trellis/commit/86e5a13a57ba3337e03557d963fd26c3ef1069c2))

## [0.1.15](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.14...v0.1.15) (2026-08-21)


### Miscellaneous

* **deps:** update module honnef.co/go/tools to v0.8.1 ([#94](https://github.com/MustardSeedNetworks/trellis/issues/94)) ([55b8797](https://github.com/MustardSeedNetworks/trellis/commit/55b8797ebf7a296ca9c958bfacbc77008b72a223))

## [0.1.14](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.13...v0.1.14) (2026-08-21)


### Continuous Integration

* refuse to start tests while orphaned test binaries are running ([#90](https://github.com/MustardSeedNetworks/trellis/issues/90)) ([4a1fb2f](https://github.com/MustardSeedNetworks/trellis/commit/4a1fb2f4ba17e2a3155af93d40540a329f43d0ee))
* stop the PR body lint cutting Testing Evidence at a fenced heading ([#93](https://github.com/MustardSeedNetworks/trellis/issues/93)) ([a26a072](https://github.com/MustardSeedNetworks/trellis/commit/a26a072bff54c0e773d2194d5bab6dbf1f7b2f1f))


### Miscellaneous

* **deps:** update dependency @babel/core to v8 ([#86](https://github.com/MustardSeedNetworks/trellis/issues/86)) ([3b62940](https://github.com/MustardSeedNetworks/trellis/commit/3b629404b11fee727aaff9c98f7ebabec4e040c1))

## [0.1.13](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.12...v0.1.13) (2026-08-21)


### Features

* **build:** enable the React Compiler, keeping every existing memo ([#85](https://github.com/MustardSeedNetworks/trellis/issues/85)) ([d88f823](https://github.com/MustardSeedNetworks/trellis/commit/d88f823ec729aad03d1723db3deee630d9c370e0))


### Bug Fixes

* **test:** run the React Compiler in vitest, so tests exercise what ships ([#88](https://github.com/MustardSeedNetworks/trellis/issues/88)) ([0ce6aff](https://github.com/MustardSeedNetworks/trellis/commit/0ce6affea912ef945a3d2c5e8ee8a53aad73e254))

## [0.1.12](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.11...v0.1.12) (2026-08-20)


### Features

* **reports:** let the operator choose what the report contains ([#84](https://github.com/MustardSeedNetworks/trellis/issues/84)) ([a4fc3f5](https://github.com/MustardSeedNetworks/trellis/commit/a4fc3f550fd8e1af782e2a2be76a97b40fbaf9d3)), closes [#83](https://github.com/MustardSeedNetworks/trellis/issues/83)


### Miscellaneous

* **deps:** update github actions ([#81](https://github.com/MustardSeedNetworks/trellis/issues/81)) ([13a027b](https://github.com/MustardSeedNetworks/trellis/commit/13a027bc6b5eae35fa902866f962267370439d43))
* **deps:** update module honnef.co/go/tools to v0.8.0 ([#25](https://github.com/MustardSeedNetworks/trellis/issues/25)) ([ad728c7](https://github.com/MustardSeedNetworks/trellis/commit/ad728c79879d87650d3e5f27888ada2f0b05aba2))
* **ts:** adopt isolatedModules and gate the fleet's strictness contract ([#80](https://github.com/MustardSeedNetworks/trellis/issues/80)) ([6fb15d9](https://github.com/MustardSeedNetworks/trellis/commit/6fb15d9595b8b631fe88e1f2937c8005ac16a5fb))

## [0.1.11](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.10...v0.1.11) (2026-08-20)


### Miscellaneous

* **ts:** turn on noUncheckedIndexedAccess and erasableSyntaxOnly ([#78](https://github.com/MustardSeedNetworks/trellis/issues/78)) ([192feeb](https://github.com/MustardSeedNetworks/trellis/commit/192feeb55c3fe991fbcc91bef442089b88c7d535)), closes [#54](https://github.com/MustardSeedNetworks/trellis/issues/54)

## [0.1.10](https://github.com/MustardSeedNetworks/trellis/compare/v0.1.9...v0.1.10) (2026-08-20)


### Bug Fixes

* **lint:** scope Biome's dist and coverage excludes to where they land ([#76](https://github.com/MustardSeedNetworks/trellis/issues/76)) ([b57cf00](https://github.com/MustardSeedNetworks/trellis/commit/b57cf00c7ce37c607ae66159ccf51ff2c549947f)), closes [#71](https://github.com/MustardSeedNetworks/trellis/issues/71)

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
