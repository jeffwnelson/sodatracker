# Changelog
All notable changes to this project will be documented in this file. See [conventional commits](https://www.conventionalcommits.org/) for commit guidelines.

- - -
## [v0.10.0](https://github.com/jeffwnelson/sodatracker/compare/e2a10b89e65f259523500d86b4d87e0c306cfc4b..v0.10.0) - 2026-10-02
#### Features
- alert on backup failures over Slack and the live stream - ([e2a10b8](https://github.com/jeffwnelson/sodatracker/commit/e2a10b89e65f259523500d86b4d87e0c306cfc4b)) - Jeff Nelson

- - -

## [v0.9.0](https://github.com/jeffwnelson/sodatracker/compare/a0fc5cb601b72ba8370ea0338b7a5e3f5d493780..v0.9.0) - 2026-10-02
#### Features
- database backups to a GitHub repo - ([4bd5efa](https://github.com/jeffwnelson/sodatracker/commit/4bd5efac51d7a903de959227e2c0c152ee5b551a)) - Jeff Nelson
#### Refactoring
- (**web**) drop the machine detail page; restock moves to Settings - ([6ee3a96](https://github.com/jeffwnelson/sodatracker/commit/6ee3a96217c31218d67989e07cafab23eb4d2386)) - Jeff Nelson
- (**web**) drop the machine name and location from the landing page - ([a0fc5cb](https://github.com/jeffwnelson/sodatracker/commit/a0fc5cb601b72ba8370ea0338b7a5e3f5d493780)) - Jeff Nelson

- - -

## [v0.8.0](https://github.com/jeffwnelson/sodatracker/compare/69dcf85628310cab32ac4f17ab4fa02b684ecb52..v0.8.0) - 2026-10-01
#### Features
- (**web**) one machine, no selectors; alert count on the Alerts tab - ([f46c431](https://github.com/jeffwnelson/sodatracker/commit/f46c4317dd79790254892e67a90fc0ae33b2e87b)) - Jeff Nelson
#### Refactoring
- (**analytics**) trim Highlights to averages, range records and totals - ([69dcf85](https://github.com/jeffwnelson/sodatracker/commit/69dcf85628310cab32ac4f17ab4fa02b684ecb52)) - Jeff Nelson

- - -

## [v0.7.0](https://github.com/jeffwnelson/sodatracker/compare/7755336eb64bcc905f0fef13aa662b20091e9003..v0.7.0) - 2026-10-01
#### Features
- (**analytics**) soda, caffeine and aluminum totals and all-time can records - ([528405a](https://github.com/jeffwnelson/sodatracker/commit/528405a61d02b73c9b31a91851a1b4588dedf231)) - Jeff Nelson
#### Refactoring
- (**web**) rename Setup to Settings; /admin/setup redirects - ([7755336](https://github.com/jeffwnelson/sodatracker/commit/7755336eb64bcc905f0fef13aa662b20091e9003)) - Jeff Nelson

- - -

## [v0.6.0](https://github.com/jeffwnelson/sodatracker/compare/0e348a6d13b3c161b3dc8783ae12f4c99a33ee7a..v0.6.0) - 2026-10-01
#### Features
- (**analytics**) date range picker with comparison - ([d7b10b6](https://github.com/jeffwnelson/sodatracker/commit/d7b10b62192b18fbb7206adfe50bdc01b6f791cc)) - Jeff Nelson
#### Refactoring
- drop sales wording; nothing in the office machine is sold - ([0e348a6](https://github.com/jeffwnelson/sodatracker/commit/0e348a6d13b3c161b3dc8783ae12f4c99a33ee7a)) - Jeff Nelson

- - -

## [v0.5.1](https://github.com/jeffwnelson/sodatracker/compare/8e59d37562d5c15bc2f8c0d72378fd9f54722463..v0.5.1) - 2026-10-01
#### Bug Fixes
- (**analytics**) report uptime from unrounded hours so a short tracked span isn't null - ([8e59d37](https://github.com/jeffwnelson/sodatracker/commit/8e59d37562d5c15bc2f8c0d72378fd9f54722463)) - Jeff Nelson

- - -

## [v0.5.0](https://github.com/jeffwnelson/sodatracker/compare/cab0e11bbd01373db269786d9da872dd528b10a5..v0.5.0) - 2026-10-01
#### Features
- (**analytics**) highlights and Pi agent uptime tabs - ([e161181](https://github.com/jeffwnelson/sodatracker/commit/e161181892dec257ab07b650c42f8ff77945167a)) - Jeff Nelson
#### Tests
- (**slacksocket**) don't fail on writes to a connection the client already dropped - ([583d2ab](https://github.com/jeffwnelson/sodatracker/commit/583d2ab8863e088e3e2ddc0b0a43859475404bb1)) - Jeff Nelson
#### Refactoring
- (**web**) drop the footer and show the version next to the brand - ([cab0e11](https://github.com/jeffwnelson/sodatracker/commit/cab0e11bbd01373db269786d9da872dd528b10a5)) - Jeff Nelson

- - -

## [v0.4.0](https://github.com/jeffwnelson/sodatracker/compare/5e854ba18afd9f858fb0d60726907364d39da6b1..v0.4.0) - 2026-10-01
#### Features
- real-time dashboards over SSE and a Pi agent heartbeat - ([5e854ba](https://github.com/jeffwnelson/sodatracker/commit/5e854ba18afd9f858fb0d60726907364d39da6b1)) - Jeff Nelson

- - -

## [v0.3.1](https://github.com/jeffwnelson/sodatracker/compare/4c8e583c04ac27fc5443ca16abf981d8e62e1632..v0.3.1) - 2026-09-30
#### Bug Fixes
- (**web**) use one page frame so the header stays the same everywhere - ([d34168e](https://github.com/jeffwnelson/sodatracker/commit/d34168ebff9c0b2441f8b1efd4f44da65855b1e3)) - Jeff Nelson
#### Refactoring
- (**web**) label the predicted stock-out date "Estimated restock" - ([86b168d](https://github.com/jeffwnelson/sodatracker/commit/86b168d6257ec13bd7993a1ca800da1844c1b3c0)) - Jeff Nelson
- (**web**) drop the Customer View link from the admin header - ([4c8e583](https://github.com/jeffwnelson/sodatracker/commit/4c8e583c04ac27fc5443ca16abf981d8e62e1632)) - Jeff Nelson

- - -

## [v0.3.0](https://github.com/jeffwnelson/sodatracker/compare/ac99172a6416baf3172b4deae4e4d8efd624d334..v0.3.0) - 2026-09-30
#### Features
- per-product colors, set on the Setup page - ([6eac56e](https://github.com/jeffwnelson/sodatracker/commit/6eac56e6dc2e3311b6bc869428924dce629ceb7a)) - Jeff Nelson
#### Continuous Integration
- build the image once, with tests inside, for arm64 only - ([ac99172](https://github.com/jeffwnelson/sodatracker/commit/ac99172a6416baf3172b4deae4e4d8efd624d334)) - Jeff Nelson

- - -

## [v0.2.0](https://github.com/jeffwnelson/sodatracker/compare/02896fa6c895fb753120a713334d390dda4e9910..v0.2.0) - 2026-09-30
#### Features
- release with cocogitto and show the version in the page footer - ([02896fa](https://github.com/jeffwnelson/sodatracker/commit/02896fa6c895fb753120a713334d390dda4e9910)) - Jeff Nelson

- - -

Changelog generated by [cocogitto](https://github.com/cocogitto/cocogitto).