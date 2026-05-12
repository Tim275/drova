# Changelog

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
