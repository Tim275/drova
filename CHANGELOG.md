# Changelog

## [0.18.12](https://github.com/Tim275/drova/compare/v0.18.11...v0.18.12) (2026-05-18)


### Bug Fixes

* driver ws auth state reset on reconnect ([#197](https://github.com/Tim275/drova/issues/197)) ([ded14ff](https://github.com/Tim275/drova/commit/ded14ff03634ee62bb12a266f7fe88bb3be1d184))

## [0.18.11](https://github.com/Tim275/drova/compare/v0.18.10...v0.18.11) (2026-05-18)


### Bug Fixes

* opentelemetry logging pipeline ([#195](https://github.com/Tim275/drova/issues/195)) ([47c0ad8](https://github.com/Tim275/drova/commit/47c0ad87b73ece550ddcdbb2c7ddc42da7570057))

## [0.18.10](https://github.com/Tim275/drova/compare/v0.18.9...v0.18.10) (2026-05-17)


### Bug Fixes

* Go adjustments ([#191](https://github.com/Tim275/drova/issues/191)) ([56983ac](https://github.com/Tim275/drova/commit/56983ac6b3575f02f2ca7c8c877b802f4b78ace5))

## [0.18.9](https://github.com/Tim275/drova/compare/v0.18.8...v0.18.9) (2026-05-16)


### Bug Fixes

* chat and some hardening ([#189](https://github.com/Tim275/drova/issues/189)) ([3f92449](https://github.com/Tim275/drova/commit/3f92449d0911e2ff497cbb969108b7e47952cc73))

## [0.18.8](https://github.com/Tim275/drova/compare/v0.18.7...v0.18.8) (2026-05-16)


### Bug Fixes

* WS subscriber reconnect + route cache + driver is_active ([2bee124](https://github.com/Tim275/drova/commit/2bee12484cf6ba2551cb50d57fbf7ac6fcd05c6a))

## [0.18.7](https://github.com/Tim275/drova/compare/v0.18.6...v0.18.7) (2026-05-15)


### Bug Fixes

* jsonb cast for ride fare route field ([21b1b20](https://github.com/Tim275/drova/commit/21b1b208d3f0ec6c19bb9a69e4324a194e7c11c6))

## [0.18.6](https://github.com/Tim275/drova/compare/v0.18.5...v0.18.6) (2026-05-15)


### Bug Fixes

* kafka consumer backoff ([276d49c](https://github.com/Tim275/drova/commit/276d49c78a743e6a73e32642a9e758dbc8ce802e))

## [0.18.5](https://github.com/Tim275/drova/compare/v0.18.4...v0.18.5) (2026-05-15)


### Bug Fixes

* remove broken app-layer rate limit ([#100](https://github.com/Tim275/drova/issues/100)) ([1896e37](https://github.com/Tim275/drova/commit/1896e37efcd1c5050333ca8d9150ea411abb26bb))

## [0.18.4](https://github.com/Tim275/drova/compare/v0.18.3...v0.18.4) (2026-05-14)


### Bug Fixes

* yq v4.53.2 + sha256 ([80ca41b](https://github.com/Tim275/drova/commit/80ca41be2498299e6b16b1923f3bed2eada30084))

## [0.18.3](https://github.com/Tim275/drova/compare/v0.18.2...v0.18.3) (2026-05-14)


### Bug Fixes

* rebuild wenn image fehlt ([23a9220](https://github.com/Tim275/drova/commit/23a9220fb035c64625baed70e14a43df943ecda6))

## [0.18.2](https://github.com/Tim275/drova/compare/v0.18.1...v0.18.2) (2026-05-14)


### Bug Fixes

* crane checksums.txt statt sha256 ([f611181](https://github.com/Tim275/drova/commit/f6111814951dcfc2d6b32f9fdbf88d3e9bd31f7c))

## [0.18.1](https://github.com/Tim275/drova/compare/v0.18.0...v0.18.1) (2026-05-14)


### Bug Fixes

* crane sha check + cosign v4 ([2985bef](https://github.com/Tim275/drova/commit/2985befedfddfd9ce2873235ea366d63daf9b2ff))

## [0.18.0](https://github.com/Tim275/drova/compare/v0.17.1...v0.18.0) (2026-05-14)


### Features

* crane to prevent building images... ([#84](https://github.com/Tim275/drova/issues/84)) ([d8d7edb](https://github.com/Tim275/drova/commit/d8d7edb4e6e7d0471579810e6c5094463871a9fe))


### Bug Fixes

* **deps:** update module github.com/segmentio/kafka-go to v0.4.51 ([#83](https://github.com/Tim275/drova/issues/83)) ([da2b3de](https://github.com/Tim275/drova/commit/da2b3deeb205add7a7446b33a63af89e454f331e))
* **renovate:** use real renovate[bot] commit-author email ([17f8294](https://github.com/Tim275/drova/commit/17f8294d1dd94af23fd24ebf9e2bbbadde2d90c4))

## [0.17.1](https://github.com/Tim275/drova/compare/v0.17.0...v0.17.1) (2026-05-14)


### Bug Fixes

* **renovate:** add gitIgnoredAuthors to unblock v43 abort ([#79](https://github.com/Tim275/drova/issues/79)) ([45056c8](https://github.com/Tim275/drova/commit/45056c883132b99b301e069fcba2af08285d1a24))
* **renovate:** add real commit-author email to gitIgnoredAuthors ([ba8ecbd](https://github.com/Tim275/drova/commit/ba8ecbd8f2aff47be9711d5f28557803db375107))

## [0.17.0](https://github.com/Tim275/drova/compare/v0.16.2...v0.17.0) (2026-05-13)


### Features

* use POOL_URL + SimpleProtocol for PgBouncer ([#76](https://github.com/Tim275/drova/issues/76)) ([b3e3904](https://github.com/Tim275/drova/commit/b3e39042787aac4c5518618500081d57da4a590f))

## [0.16.2](https://github.com/Tim275/drova/compare/v0.16.1...v0.16.2) (2026-05-12)


### Bug Fixes

* strip CRLF from email recipient to prevent header injection ([58aa9a2](https://github.com/Tim275/drova/commit/58aa9a29017c33c093973634cc587cf282fdb633))

## [0.16.1](https://github.com/Tim275/drova/compare/v0.16.0...v0.16.1) (2026-05-12)


### Bug Fixes

* complete e2e-overlay for k3d CI ([06bc5ac](https://github.com/Tim275/drova/commit/06bc5ac1fc9468b8186e017ddf16794a809eb44d))
* declare GITOPS_PAT secret in e2e-tests workflow_call ([2692226](https://github.com/Tim275/drova/commit/269222635fde516e10437aff3cba4f1576234f0c))
* workflow_dispatch tag handling ([d5e6aa7](https://github.com/Tim275/drova/commit/d5e6aa72ee9696501f3803a45b9ce4d0ce7ee325))
* workflow-level permissions for reusable workflow ([cced5d9](https://github.com/Tim275/drova/commit/cced5d967122836ce5c91db743e86ba2f3eb4fa1))

## [0.16.0](https://github.com/Tim275/drova/compare/v0.15.0...v0.16.0) (2026-05-11)


### Features

* driver reconnect, refresh token, tests, pipeline hardening ([679eee9](https://github.com/Tim275/drova/commit/679eee97ff337dad08917164ad06ce333188e57a))

## [0.15.0](https://github.com/Tim275/drova/compare/v0.14.0...v0.15.0) (2026-05-04)


### Features

* ADJUST frontend setting ([74849e3](https://github.com/Tim275/drova/commit/74849e340b24670f1376cd127863dae68f5d1a84))
* Redis GEOSEARCH + 12-Factor fixes ([5cbcbbc](https://github.com/Tim275/drova/commit/5cbcbbc1696de2724d74b8b1e2bb3e7b3de7c23b))


### Bug Fixes

* checkout main branch to avoid detached HEAD ([cdac389](https://github.com/Tim275/drova/commit/cdac38994704baca0956d8c714d7160baef8613f))
* frontend error ([1763e09](https://github.com/Tim275/drova/commit/1763e09c0826c7ef73b1abb7e2acbab2aeb24518))
* phone validation + map extrapolation + activation redirect ([e2d4602](https://github.com/Tim275/drova/commit/e2d46020f0e36a3be62fe05f07f4479b70379150))
* restore working pipeline config ([5e86d52](https://github.com/Tim275/drova/commit/5e86d52293a8cb0a9221713029c0cdd3400674d6))
* test password uppercase ([94d12b1](https://github.com/Tim275/drova/commit/94d12b1e86e041dd595bba724dfb652d11550bb6))
* tidy exit code capture ([14fd40d](https://github.com/Tim275/drova/commit/14fd40d6d4669035d4c51e4a6c7229d7a00259b0))
* trivy non-blocking, decouple deploy from scan ([c4a89c8](https://github.com/Tim275/drova/commit/c4a89c89f907e9e265a5746d2e37871289705dfe))

## [0.12.0](https://github.com/Tim275/drova/compare/v0.11.2...v0.12.0) (2026-05-04)


### Features

* ADJUST frontend setting ([74849e3](https://github.com/Tim275/drova/commit/74849e340b24670f1376cd127863dae68f5d1a84))
* Redis GEOSEARCH + 12-Factor fixes ([5cbcbbc](https://github.com/Tim275/drova/commit/5cbcbbc1696de2724d74b8b1e2bb3e7b3de7c23b))


### Bug Fixes

* checkout main branch to avoid detached HEAD ([cdac389](https://github.com/Tim275/drova/commit/cdac38994704baca0956d8c714d7160baef8613f))
* frontend error ([1763e09](https://github.com/Tim275/drova/commit/1763e09c0826c7ef73b1abb7e2acbab2aeb24518))
* phone validation + map extrapolation + activation redirect ([e2d4602](https://github.com/Tim275/drova/commit/e2d46020f0e36a3be62fe05f07f4479b70379150))
* restore working pipeline config ([5e86d52](https://github.com/Tim275/drova/commit/5e86d52293a8cb0a9221713029c0cdd3400674d6))
* test password uppercase ([94d12b1](https://github.com/Tim275/drova/commit/94d12b1e86e041dd595bba724dfb652d11550bb6))
* tidy exit code capture ([14fd40d](https://github.com/Tim275/drova/commit/14fd40d6d4669035d4c51e4a6c7229d7a00259b0))
* trivy non-blocking, decouple deploy from scan ([c4a89c8](https://github.com/Tim275/drova/commit/c4a89c89f907e9e265a5746d2e37871289705dfe))

## [0.11.2](https://github.com/Tim275/drova/compare/v0.11.1...v0.11.2) (2026-05-01)


### Bug Fixes

* add trip→user gRPC arrow in diagram ([b8ee9f3](https://github.com/Tim275/drova/commit/b8ee9f373592356008f662c9f4b4137cdd64e449))
* driver geo + rider history ([9d12e56](https://github.com/Tim275/drova/commit/9d12e56005b196a2c81d10ffa6211d60d9c314e2))
* readme cleanup ([a7503e4](https://github.com/Tim275/drova/commit/a7503e41dfba1acfd76ce55b99eab064708c2c62))

## [0.11.1](https://github.com/Tim275/drova/compare/v0.11.0...v0.11.1) (2026-05-01)


### Bug Fixes

* driver geo + rider history ([9d12e56](https://github.com/Tim275/drova/commit/9d12e56005b196a2c81d10ffa6211d60d9c314e2))
* password regex + kafka TLS support ([b0bb30b](https://github.com/Tim275/drova/commit/b0bb30b1ee1175350aae31c4963fdf7e30fac55d))

## [0.11.0](https://github.com/Tim275/drova/compare/v0.10.0...v0.11.0) (2026-05-01)


### Features

* Redis GEOSEARCH + 12-Factor fixes ([4f8f677](https://github.com/Tim275/drova/commit/4f8f6777f6fa755b5d47e4d0808b970482fb7c21))
* SASL SCRAM-512, CSP fix, autocomplete, lerp animation, validation ([6af931b](https://github.com/Tim275/drova/commit/6af931b308a4ec115d1ce665aaabccc1bbea0860))


### Bug Fixes

* driver geo + rider history ([9d12e56](https://github.com/Tim275/drova/commit/9d12e56005b196a2c81d10ffa6211d60d9c314e2))
* password regex + kafka TLS support ([b0bb30b](https://github.com/Tim275/drova/commit/b0bb30b1ee1175350aae31c4963fdf7e30fac55d))
* phone validation + map extrapolation + activation redirect ([3c15e99](https://github.com/Tim275/drova/commit/3c15e99653fef5b71abd7fcd54b3a715660b430d))

## [0.10.0](https://github.com/Tim275/drova/compare/v0.9.9...v0.10.0) (2026-04-30)


### Features

* Kafka 3-broker KRaft cluster (RF=3) ([afa630f](https://github.com/Tim275/drova/commit/afa630fc0abf9e3b24d9c5e995197cb077e1d477))
* Kafka 3-broker KRaft cluster (RF=3) ([afa630f](https://github.com/Tim275/drova/commit/afa630fc0abf9e3b24d9c5e995197cb077e1d477))
* otel log bridge in shared logger + gRPC user-service migration ([54dc0d3](https://github.com/Tim275/drova/commit/54dc0d3914ffac0b570f79b37c4ee5c912cd12ac))
* otel logger bridge ([5f8ecac](https://github.com/Tim275/drova/commit/5f8ecac172fbd7d5972736b45c9bcb2669c2d008))
* user-service gRPC migration ([bb44afd](https://github.com/Tim275/drova/commit/bb44afde6ee88e288e49ecdc82c6bb0d29ec5900))
* user-service gRPC migration ([bb44afd](https://github.com/Tim275/drova/commit/bb44afde6ee88e288e49ecdc82c6bb0d29ec5900))


### Bug Fixes

* circuit breaker + ci lint version ([db3e86f](https://github.com/Tim275/drova/commit/db3e86f8b7a967094659b4926308debacf9cfebe))
* login supports Basic Auth + JSON body ([2033234](https://github.com/Tim275/drova/commit/20332342a3f366fcaad1c68cd87cfa7ae9c9d427))

## [0.9.1](https://github.com/Tim275/drova/compare/v0.9.0...v0.9.1) (2026-04-29)


### Bug Fixes

* path filters on trigger, remove broken trivy ([eaefe8b](https://github.com/Tim275/drova/commit/eaefe8b1c026920d6a9a89b1a1faad36acb68139))
* trivy action version ([e4f33fb](https://github.com/Tim275/drova/commit/e4f33fb4916dfe2335e16b5d596233bf7b48567c))
* use net.JoinHostPort for IPv6 compat ([2a3089c](https://github.com/Tim275/drova/commit/2a3089cd9a92de5563488ec11d0174403d1e7430))
