# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Security

- `crypto.RandString` reads from `crypto/rand`. It used `math/rand` seeded
  with `time.Now().UnixNano()`, which Go reduces modulo 2³¹−1, so it could
  only return about 2.1 billion different strings at any length. Its output
  could be reproduced from roughly when it was called, and calls whose
  timestamps matched modulo 2³¹−1 returned identical strings. The alphabet
  (`a-z`, `A-Z`) and lengths are unchanged, so existing token formats stay
  valid. A zero or negative length returns an empty string.

### Added

- Tests for `crypto.RandString` covering length, alphabet, uniqueness and
  an even spread of letters.
