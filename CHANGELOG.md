# Changelog
All notable changes to this project will be documented in this file. See [conventional commits](https://www.conventionalcommits.org/) for commit guidelines.

- - -
## [v0.20.4](https://github.com/jeffwnelson/sodatracker/compare/500fecbf7cb4ccdf24daca16ba6233236ff1d52a..v0.20.4) - 2026-10-21
#### Bug Fixes
- (**wiring**) adopt rewire onto header pins 33–40 - ([8f41dd1](https://github.com/jeffwnelson/sodatracker/commit/8f41dd1c8c8db5317d93dac8c9a0956ca411e0bc)) - Jeff Nelson
#### Documentation
- (**pi**) add Wi-Fi watchdog and persistent journal host config - ([500fecb](https://github.com/jeffwnelson/sodatracker/commit/500fecbf7cb4ccdf24daca16ba6233236ff1d52a)) - Jeff Nelson
#### Continuous Integration
- build on GitHub-hosted arm64 runners - ([7bb7cfc](https://github.com/jeffwnelson/sodatracker/commit/7bb7cfc1bffe7b26c38211e5ac8a236e841b87ae)) - Jeff Nelson
#### Miscellaneous Chores
- move private network details out of the repo - ([ae53c54](https://github.com/jeffwnelson/sodatracker/commit/ae53c54a97b72a58de4bced5b2d3f0c66fbcfbc9)) - Jeff Nelson

- - -

## [v0.20.3](https://github.com/jeffwnelson/sodatracker/compare/19ff9996c14274d519b91dc2e03d8ff102e0a918..v0.20.3) - 2026-10-09
#### Bug Fixes
- (**security**) refuse dev secrets, throttle login, tighten auth and backups - ([19ff999](https://github.com/jeffwnelson/sodatracker/commit/19ff9996c14274d519b91dc2e03d8ff102e0a918)) - Jeff Nelson

- - -

## [v0.20.2](https://github.com/jeffwnelson/sodatracker/compare/3746c8970e62e2e4b969da27ef17b9083aa90cb3..v0.20.2) - 2026-10-09
#### Bug Fixes
- keep login after Reset All Data, guard Save Slots, close static files - ([3746c89](https://github.com/jeffwnelson/sodatracker/commit/3746c8970e62e2e4b969da27ef17b9083aa90cb3)) - Jeff Nelson

- - -

## [v0.20.1](https://github.com/jeffwnelson/sodatracker/compare/b454dc9d8678aad0e0ea7d12132bde163b753ba7..v0.20.1) - 2026-10-09
#### Bug Fixes
- (**piagent**) never count a dispense twice or miss one while sending - ([b454dc9](https://github.com/jeffwnelson/sodatracker/commit/b454dc9d8678aad0e0ea7d12132bde163b753ba7)) - Jeff Nelson
#### Performance Improvements
- load only the analytics history a view needs, refetch less on live events - ([4473acc](https://github.com/jeffwnelson/sodatracker/commit/4473acc668d6c9621caacd15dd6e357ad4363eba)) - Jeff Nelson
#### Documentation
- describe the agent outbox, windowed analytics and targeted live refreshes - ([7b513c4](https://github.com/jeffwnelson/sodatracker/commit/7b513c4de198148d104b4a4b0e0f17af1463d26c)) - Jeff Nelson

- - -

## [v0.20.0](https://github.com/jeffwnelson/sodatracker/compare/f22368981a8b940fa40e2062e5ff72e3cabe74a9..v0.20.0) - 2026-10-08
#### Features
- (**web**) recent dispenses show the event ID and expand to raw JSON - ([475181f](https://github.com/jeffwnelson/sodatracker/commit/475181f8d0df39c18b46b524451114a27c031cfc)) - Jeff Nelson
#### Documentation
- Diet Mountain Dew columns confirmed, drop unconfirmed marks from wiring - ([f223689](https://github.com/jeffwnelson/sodatracker/commit/f22368981a8b940fa40e2062e5ff72e3cabe74a9)) - Jeff Nelson

- - -

## [v0.19.2](https://github.com/jeffwnelson/sodatracker/compare/0057249c07f2f29a95bf0e794ef162bbd3cff315..v0.19.2) - 2026-10-07
#### Bug Fixes
- (**web**) wiring table and labels follow the diagram, say Slot, dots before product names - ([0057249](https://github.com/jeffwnelson/sodatracker/commit/0057249c07f2f29a95bf0e794ef162bbd3cff315)) - Jeff Nelson

- - -

## [v0.19.1](https://github.com/jeffwnelson/sodatracker/compare/2a5a28aac005f5852773e246594cea3a518ff2bb..v0.19.1) - 2026-10-07
#### Bug Fixes
- (**web**) proposed wiring uses T568B jumper colors - ([2a5a28a](https://github.com/jeffwnelson/sodatracker/commit/2a5a28aac005f5852773e246594cea3a518ff2bb)) - Jeff Nelson

- - -

## [v0.19.0](https://github.com/jeffwnelson/sodatracker/compare/aa164ffbecfe2195ff29c75ef67381397eef6e0f..v0.19.0) - 2026-10-07
#### Features
- (**web**) wiring diagram on Settings, with a proposed rewire - ([aa164ff](https://github.com/jeffwnelson/sodatracker/commit/aa164ffbecfe2195ff29c75ef67381397eef6e0f)) - Jeff Nelson

- - -

## [v0.18.0](https://github.com/jeffwnelson/sodatracker/compare/d04f110dc967ad58be1fd6e24c1d4cff28b05640..v0.18.0) - 2026-10-07
#### Features
- (**analytics**) product heatmap with product filter on the Products tab - ([d04f110](https://github.com/jeffwnelson/sodatracker/commit/d04f110dc967ad58be1fd6e24c1d4cff28b05640)) - Jeff Nelson

- - -

## [v0.17.2](https://github.com/jeffwnelson/sodatracker/compare/f4cd8934afc6f321b4874cc22f1c9c2394760668..v0.17.2) - 2026-10-03
#### Bug Fixes
- (**analytics**) lunch favorite counts 11:30 to 1 PM - ([f4cd893](https://github.com/jeffwnelson/sodatracker/commit/f4cd8934afc6f321b4874cc22f1c9c2394760668)) - Jeff Nelson

- - -

## [v0.17.1](https://github.com/jeffwnelson/sodatracker/compare/efffedd6cbb68a1c9a02ce3becb49c21d13271c7..v0.17.1) - 2026-10-02
#### Bug Fixes
- (**web**) show active alerts right under the status banner - ([efffedd](https://github.com/jeffwnelson/sodatracker/commit/efffedd6cbb68a1c9a02ce3becb49c21d13271c7)) - Jeff Nelson

- - -

## [v0.17.0](https://github.com/jeffwnelson/sodatracker/compare/582e65e0d890f71fcaa3f4213d3faed18f593c36..v0.17.0) - 2026-10-02
#### Features
- (**web**) Status tab replaces Alerts; drop not-registering alerts - ([582e65e](https://github.com/jeffwnelson/sodatracker/commit/582e65e0d890f71fcaa3f4213d3faed18f593c36)) - Jeff Nelson

- - -

## [v0.16.0](https://github.com/jeffwnelson/sodatracker/compare/88e0d66b33635a12c292dffb0aad6da6b6283af2..v0.16.0) - 2026-10-02
#### Features
- (**analytics**) status page on the Uptime tab - ([88e0d66](https://github.com/jeffwnelson/sodatracker/commit/88e0d66b33635a12c292dffb0aad6da6b6283af2)) - Jeff Nelson

- - -

## [v0.15.0](https://github.com/jeffwnelson/sodatracker/compare/02bb150bff780be1cda7893f31b042e14781ed2c..v0.15.0) - 2026-10-02
#### Features
- (**settings**) configure Slack from the Settings page - ([02bb150](https://github.com/jeffwnelson/sodatracker/commit/02bb150bff780be1cda7893f31b042e14781ed2c)) - Jeff Nelson

- - -

## [v0.14.0](https://github.com/jeffwnelson/sodatracker/compare/25597c6fdafba28c521caf4ebd96860096d5ee74..v0.14.0) - 2026-10-02
#### Features
- (**deploy**) serve the dashboard only over HTTPS - ([1949370](https://github.com/jeffwnelson/sodatracker/commit/194937019c1c05ef24dc72ffbef04f44ac4ffdb1)) - Jeff Nelson
#### Documentation
- HTTPS for the dashboard through Tailscale serve - ([25597c6](https://github.com/jeffwnelson/sodatracker/commit/25597c6fdafba28c521caf4ebd96860096d5ee74)) - Jeff Nelson

- - -

## [v0.13.0](https://github.com/jeffwnelson/sodatracker/compare/8d722852ff5cc127fc8b1eb44a645cf4a0ecfe48..v0.13.0) - 2026-10-02
#### Features
- (**web**) iPhone-ready layout and Add to Home Screen - ([8d72285](https://github.com/jeffwnelson/sodatracker/commit/8d722852ff5cc127fc8b1eb44a645cf4a0ecfe48)) - Jeff Nelson

- - -

## [v0.12.0](https://github.com/jeffwnelson/sodatracker/compare/5e2e91e46d0f22fbd5a0846a1b0efb0742538771..v0.12.0) - 2026-10-02
#### Features
- (**analytics**) fit the three highlight pies on one row - ([5e2e91e](https://github.com/jeffwnelson/sodatracker/commit/5e2e91e46d0f22fbd5a0846a1b0efb0742538771)) - Jeff Nelson
#### Refactoring
- (**web**) one Button for every action - ([6ad8d90](https://github.com/jeffwnelson/sodatracker/commit/6ad8d90af05b11e62864f9c3d808851f4f73b677)) - Jeff Nelson

- - -

## [v0.11.0](https://github.com/jeffwnelson/sodatracker/compare/22a63e2b098c2f3270913459377d632cd1489242..v0.11.0) - 2026-10-02
#### Features
- (**alerts**) alert when a slot or the machine stops registering - ([8f9e1d3](https://github.com/jeffwnelson/sodatracker/commit/8f9e1d3da98443c7b0ca74ad77b01e04defacc05)) - Jeff Nelson
- (**analytics**) who closes the day and lunch favorite highlights - ([6240c63](https://github.com/jeffwnelson/sodatracker/commit/6240c63e91f517cd464c6c04daf6341498233255)) - Jeff Nelson
- (**web**) recent dispenses log on the dashboard - ([a087af4](https://github.com/jeffwnelson/sodatracker/commit/a087af45b0aa73a5a48ee0b50e90099e0b9e837f)) - Jeff Nelson
#### Bug Fixes
- (**store**) actually enable WAL and the busy timeout - ([22a63e2](https://github.com/jeffwnelson/sodatracker/commit/22a63e2b098c2f3270913459377d632cd1489242)) - Jeff Nelson

- - -

## [v0.10.0](https://github.com/jeffwnelson/sodatracker/compare/6201bd25baa223a0dadeaafdb40e22c767a95754..v0.10.0) - 2026-10-02
#### Features
- alert on backup failures over Slack and the live stream - ([6201bd2](https://github.com/jeffwnelson/sodatracker/commit/6201bd25baa223a0dadeaafdb40e22c767a95754)) - Jeff Nelson

- - -

## [v0.9.0](https://github.com/jeffwnelson/sodatracker/compare/30ec45d92962724bb7b038440d80e95e1f4cd029..v0.9.0) - 2026-10-02
#### Features
- database backups to a GitHub repo - ([f6107f6](https://github.com/jeffwnelson/sodatracker/commit/f6107f62e08e57cfe25b052c1ddbec276c0ddfce)) - Jeff Nelson
#### Refactoring
- (**web**) drop the machine detail page; restock moves to Settings - ([3e94f4b](https://github.com/jeffwnelson/sodatracker/commit/3e94f4b06667fee4429844de95104ef81b952f23)) - Jeff Nelson
- (**web**) drop the machine name and location from the landing page - ([30ec45d](https://github.com/jeffwnelson/sodatracker/commit/30ec45d92962724bb7b038440d80e95e1f4cd029)) - Jeff Nelson

- - -

## [v0.8.0](https://github.com/jeffwnelson/sodatracker/compare/93948f71dac5923e0a31ce6323ec67a45e1d745f..v0.8.0) - 2026-10-01
#### Features
- (**web**) one machine, no selectors; alert count on the Alerts tab - ([70be452](https://github.com/jeffwnelson/sodatracker/commit/70be45226c2d17f12455b063d2cd2f6e706f9245)) - Jeff Nelson
#### Refactoring
- (**analytics**) trim Highlights to averages, range records and totals - ([93948f7](https://github.com/jeffwnelson/sodatracker/commit/93948f71dac5923e0a31ce6323ec67a45e1d745f)) - Jeff Nelson

- - -

## [v0.7.0](https://github.com/jeffwnelson/sodatracker/compare/6f409a9b20acf69d413bda07994b227168efd1cf..v0.7.0) - 2026-10-01
#### Features
- (**analytics**) soda, caffeine and aluminum totals and all-time can records - ([b32562a](https://github.com/jeffwnelson/sodatracker/commit/b32562a57ddeaf74ce2161d28916cffb5f92bc2e)) - Jeff Nelson
#### Refactoring
- (**web**) rename Setup to Settings; /admin/setup redirects - ([6f409a9](https://github.com/jeffwnelson/sodatracker/commit/6f409a9b20acf69d413bda07994b227168efd1cf)) - Jeff Nelson

- - -

## [v0.6.0](https://github.com/jeffwnelson/sodatracker/compare/6ae919467f549722fa0411d02c306ce95116c504..v0.6.0) - 2026-10-01
#### Features
- (**analytics**) date range picker with comparison - ([eb5f28e](https://github.com/jeffwnelson/sodatracker/commit/eb5f28ea76dbadf247c11d11fa89057bbbc95dc4)) - Jeff Nelson
#### Refactoring
- drop sales wording; nothing in the office machine is sold - ([6ae9194](https://github.com/jeffwnelson/sodatracker/commit/6ae919467f549722fa0411d02c306ce95116c504)) - Jeff Nelson

- - -

## [v0.5.1](https://github.com/jeffwnelson/sodatracker/compare/b7778f03bb72eb152f6e9de4d9aa8635e0c61bc0..v0.5.1) - 2026-10-01
#### Bug Fixes
- (**analytics**) report uptime from unrounded hours so a short tracked span isn't null - ([b7778f0](https://github.com/jeffwnelson/sodatracker/commit/b7778f03bb72eb152f6e9de4d9aa8635e0c61bc0)) - Jeff Nelson

- - -

## [v0.5.0](https://github.com/jeffwnelson/sodatracker/compare/1b1e344bc6056deecb9bfa6cc45c1866f2aef9dc..v0.5.0) - 2026-10-01
#### Features
- (**analytics**) highlights and Pi agent uptime tabs - ([53efcb1](https://github.com/jeffwnelson/sodatracker/commit/53efcb14899f88ce4bf953b61c12b3e59dea5b41)) - Jeff Nelson
#### Tests
- (**slacksocket**) don't fail on writes to a connection the client already dropped - ([7637ff2](https://github.com/jeffwnelson/sodatracker/commit/7637ff28dd72074e2938fa13b836614915352703)) - Jeff Nelson
#### Refactoring
- (**web**) drop the footer and show the version next to the brand - ([1b1e344](https://github.com/jeffwnelson/sodatracker/commit/1b1e344bc6056deecb9bfa6cc45c1866f2aef9dc)) - Jeff Nelson

- - -

## [v0.4.0](https://github.com/jeffwnelson/sodatracker/compare/e478c2da6914f60633ba6a113fdd7b4c0c85442e..v0.4.0) - 2026-10-01
#### Features
- real-time dashboards over SSE and a Pi agent heartbeat - ([e478c2d](https://github.com/jeffwnelson/sodatracker/commit/e478c2da6914f60633ba6a113fdd7b4c0c85442e)) - Jeff Nelson

- - -

## [v0.3.1](https://github.com/jeffwnelson/sodatracker/compare/7abb58363e72e696ad2dc46fd788936cfeb879d2..v0.3.1) - 2026-09-30
#### Bug Fixes
- (**web**) use one page frame so the header stays the same everywhere - ([f6c2234](https://github.com/jeffwnelson/sodatracker/commit/f6c2234a678162d7ffa04a4883b303d9b661ce38)) - Jeff Nelson
#### Refactoring
- (**web**) label the predicted stock-out date "Estimated restock" - ([5323199](https://github.com/jeffwnelson/sodatracker/commit/5323199b45dfd106cbf04f80b13526aff8393f49)) - Jeff Nelson
- (**web**) drop the Customer View link from the admin header - ([7abb583](https://github.com/jeffwnelson/sodatracker/commit/7abb58363e72e696ad2dc46fd788936cfeb879d2)) - Jeff Nelson

- - -

## [v0.3.0](https://github.com/jeffwnelson/sodatracker/compare/f1d844728df646f4f9361bf065ad1137d2e99409..v0.3.0) - 2026-09-30
#### Features
- per-product colors, set on the Setup page - ([eaf27df](https://github.com/jeffwnelson/sodatracker/commit/eaf27df6411d3ec95cec6624d24b3f520d8960f5)) - Jeff Nelson
#### Continuous Integration
- build the image once, with tests inside, for arm64 only - ([f1d8447](https://github.com/jeffwnelson/sodatracker/commit/f1d844728df646f4f9361bf065ad1137d2e99409)) - Jeff Nelson

- - -

## [v0.2.0](https://github.com/jeffwnelson/sodatracker/compare/af8486864b33c54d528cfede9740b060249e96ca..v0.2.0) - 2026-09-30
#### Features
- release with cocogitto and show the version in the page footer - ([af84868](https://github.com/jeffwnelson/sodatracker/commit/af8486864b33c54d528cfede9740b060249e96ca)) - Jeff Nelson

- - -

Changelog generated by [cocogitto](https://github.com/cocogitto/cocogitto).