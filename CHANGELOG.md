# Changelog

## [2.0.0](https://github.com/jdmcgrath/orgsync/compare/v1.7.0...v2.0.0) (2025-06-23)


### ⚠ BREAKING CHANGES

* Complete UI rewrite using ASCII-only characters for better terminal compatibility. Previous versions used Unicode characters that may not render correctly in all terminals.

### Features

* add version constant and complete UI rewrite ([6d0c658](https://github.com/jdmcgrath/orgsync/commit/6d0c6582087d3ab78684e99dd687a837aa98b6b1))


### Reverts

* restore manifest to 1.7.0 ([a9e663d](https://github.com/jdmcgrath/orgsync/commit/a9e663ddcbe43be0fedaadeac1428d249cc73162))

## [2.0.0](https://github.com/jdmcgrath/orgsync/compare/v1.7.0...v2.0.0) (2025-06-23)

### ⚠ BREAKING CHANGES

* UI now uses ASCII characters instead of Unicode for better compatibility

### Features

* enhance UI with professional animations and test mode ([f1bf4f5](https://github.com/jdmcgrath/orgsync/commit/f1bf4f5))
  - Add test mode (--test flag) to simulate operations without creating repos
  - Replace Unicode characters with ASCII for better terminal compatibility
  - Implement fixed-width columns with proper left alignment
  - Add professional animations: rotating spinner for pending, clean progress bars
  - Remove distracting emoji animations from header
  - Add elapsed time, ETA, and transfer speed indicators
  - Improve error messages with actionable hints
  - Add ability to toggle completed repos visibility (press 'c')
  - Create test scripts for easy UI demonstration

### Documentation

* add test mode note to CLAUDE.md ([b500256](https://github.com/jdmcgrath/orgsync/commit/b500256))

## [1.7.0](https://github.com/jdmcgrath/orgsync/compare/v1.6.0...v1.7.0) (2025-05-26)


### Features

* fix go sum ([3264fd5](https://github.com/jdmcgrath/orgsync/commit/3264fd505945216ad1919baf2e5a18793494c74d))

## [1.6.0](https://github.com/jdmcgrath/orgsync/compare/v1.5.0...v1.6.0) (2025-05-26)


### Features

* enhance release workflow with auto-merge and retry logic ([abff338](https://github.com/jdmcgrath/orgsync/commit/abff338162a9c62de4e19e7a3f11df843acd5d18))
* update release workflows to include issue and project permissions ([ddf6bad](https://github.com/jdmcgrath/orgsync/commit/ddf6bad29b73f392a35f6a549b0c6c952464610c))

## [1.5.0](https://github.com/jdmcgrath/orgsync/compare/v1.4.0...v1.5.0) (2025-05-26)


### Features

* enhance release workflow with auto-merge and retry logic ([abff338](https://github.com/jdmcgrath/orgsync/commit/abff338162a9c62de4e19e7a3f11df843acd5d18))
* update release workflows to include issue and project permissions ([ddf6bad](https://github.com/jdmcgrath/orgsync/commit/ddf6bad29b73f392a35f6a549b0c6c952464610c))
* enhance repository status tests and add failure count functionality ([5837467](https://github.com/jdmcgrath/orgsync/commit/5837467fb480f2741834768a3580dd126fb5cddd))
* Update go mod ([c10e0cf](https://github.com/jdmcgrath/orgsync/commit/c10e0cf9d864675875ad43f9442f0b6c28a2d93b))

## [1.4.0](https://github.com/jdmcgrath/orgsync/compare/v1.3.0...v1.4.0) (2025-05-26)


### Features

* enhance repository synchronization with status updates ([ee70b90](https://github.com/jdmcgrath/orgsync/commit/ee70b90fb9414bee44f687be7f310ecb6ef88f5d))
* enhance repository status tests and add failure count functionality ([5837467](https://github.com/jdmcgrath/orgsync/commit/5837467fb480f2741834768a3580dd126fb5cddd))
* Update go mod ([c10e0cf](https://github.com/jdmcgrath/orgsync/commit/c10e0cf9d864675875ad43f9442f0b6c28a2d93b))

## [1.3.0](https://github.com/jdmcgrath/orgsync/compare/v1.2.0...v1.3.0) (2025-05-26)


### Features

* implement automatic releases without manual PR merging ([14615b4](https://github.com/jdmcgrath/orgsync/commit/14615b4e9ddc84c9d1e323b131b9fa723aa24593))


### Bug Fixes

* properly extract PR number from release-please output for auto-merge ([efd0c91](https://github.com/jdmcgrath/orgsync/commit/efd0c9156ee86a955c89786ab6b095cf3aa3b709))

## [1.2.0](https://github.com/jdmcgrath/orgsync/compare/v1.1.0...v1.2.0) (2025-05-26)


### Features

* add unit tests and enhance repository synchronization model ([cfa3a6c](https://github.com/jdmcgrath/orgsync/commit/cfa3a6c5b4d3a35cc9ae01119d06a8336581a2c4))

## [1.1.0](https://github.com/jdmcgrath/orgsync/compare/v1.0.0...v1.1.0) (2025-05-26)


### Features

* add automated semantic versioning and releases ([4ebca6e](https://github.com/jdmcgrath/orgsync/commit/4ebca6e884a54819457b7efab3e00ca682cfcd0b))


### Bug Fixes

* update release workflow to use correct action and permissions ([1d875e6](https://github.com/jdmcgrath/orgsync/commit/1d875e62e4da54d597ff0d7e701f175e426aea8e))
