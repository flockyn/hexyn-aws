# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased](https://github.com/flockyn/hexyn-aws/compare/v0.6.0...main)

Please do not update the unreleased notes.

<!-- Content should be placed here -->

## [v0.6.0](https://github.com/flockyn/hexyn-aws/compare/v0.5.0...v0.6.0) - 2026-06-30

## Changelog
### Bug fixes
*  fix(tui): add window padding and soft-wrap long content
### Refactor
*  refactor(awsx): format smithy API errors and use standard waitgroup

## [v0.5.0](https://github.com/flockyn/hexyn-aws/compare/v0.4.0...v0.5.0) - 2026-06-12

## Changelog
### Features
*  feat(config): INI config file with env-overridable repo-name prefixes
*  feat(secrets): reconcile path exports against the task definition
*  feat(tui): PUT confirmation, version label, pinned footer, settings
*  feat(tui): wrap long values in the PUT confirmation table
### Bug fixes
*  fix(ssm): paginate GetByPath so exports include every parameter

## [v0.4.0](https://github.com/flockyn/hexyn-aws/compare/v0.3.0...v0.4.0) - 2026-06-12

## Changelog
### Features
*  feat(envfile): parse single- and multi-line JSON values
### Others
*  docs(landing): show only the latest release in changelog

## [v0.3.0](https://github.com/flockyn/hexyn-aws/compare/v0.2.1...v0.3.0) - 2026-06-11

## Changelog
### Features
*  feat(envfile): parse multi-line PEM values
### Bug fixes
*  fix(tui): render empty output subdirectory as output/ not output//
### Refactor
*  refactor(awsx): export SDK seam interfaces; simplify region listing
### Others
*  build(make): install mockery; exclude non-product code from coverage
*  build(test): add testify and mockery with auto-discovery config
*  test(infra): add centralized mocks and shared fixtures

## [v0.2.1](https://github.com/flockyn/hexyn-aws/compare/v0.2.0...v0.2.1) - 2026-06-11

## Changelog
### Bug fixes
*  fix(update): drop GITHUB_TOKEN requirement and handle permission errors

## [v0.2.0](https://github.com/flockyn/hexyn-aws/compare/v0.1.0...v0.2.0) - 2026-06-11

## Changelog
### Features
*  feat(website): redesign landing page with live changelog
### Refactor
*  refactor: restructure into idiomatic package-by-feature architecture
### Others
*  chore(install): drop GitHub token requirement for public repo

## [v0.1.0](https://github.com/flockyn/hexyn-aws/commits/v0.1.0) - 2026-06-07

## Changelog
### Features
*  feat(install): add cross-platform installation scripts
*  feat: implement self-update and refine TUI experience
### Others
*  ci: add github actions and goreleaser integration
*  docs(website): add documentation and project metadata
*  docs(website): add landing page and visual assets
*  initial commit
