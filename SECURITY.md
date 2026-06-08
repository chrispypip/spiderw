# Security Policy

## Supported Versions

spiderw does not have a stable release series yet. Until the first tagged
release, security fixes are made on the `main` branch.

After stable releases begin, supported versions will be documented here.

| Version | Supported |
| ------- | --------- |
| main    | Yes       |

## Reporting a Vulnerability

Please do not report security vulnerabilities through public GitHub issues.

If you believe you have found a vulnerability in spiderw, please report it
privately using GitHub's private vulnerability reporting feature if it is
enabled for this repository.

If private vulnerability reporting is not available, open a minimal public issue
that says you would like to report a security vulnerability, but do not include
details, exploit code, logs, or proof-of-concept steps in the public issue.

## What to Include

When reporting a vulnerability, please include as much of the following as you
can safely provide:

- A description of the issue.
- The affected spiderw version, commit, or branch.
- The operating system and Go version.
- Whether the issue affects the library, CLI, mock service, tests, or docs.
- Minimal reproduction steps.
- Any relevant D-Bus/iwd environment details.
- The potential impact.

## Scope

Security issues may include, but are not limited to:

- Incorrect handling of untrusted D-Bus data.
- Panics or crashes triggered by malformed D-Bus responses or signals.
- Race conditions with security impact.
- CLI behavior that could unexpectedly affect adapter state.
- Dependency vulnerabilities that affect spiderw users.

General bugs, feature requests, documentation fixes, and test failures should be
reported as normal GitHub issues.

## Disclosure Process

I will try to acknowledge valid reports as soon as practical.

Security fixes may be developed privately before public disclosure. Once a fix is
available, the vulnerability may be disclosed through a GitHub security advisory
or a release note, depending on severity.

## Security Updates

Users should update to the latest commit or release containing a security fix.
Until stable releases exist, the `main` branch is the supported security update
target.
