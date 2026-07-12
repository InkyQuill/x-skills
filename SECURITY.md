# Security Policy

## Reporting a Vulnerability

If you discover a security vulnerability in x-skills, please report it
privately rather than opening a public issue.

**To report:** open a [GitHub Security Advisory](https://github.com/InkyQuill/x-skills/security/advisories/new)
or email the maintainer at `me@inkyquill.net`.

Please include:

- A description of the vulnerability
- Steps to reproduce
- Affected versions
- Any potential mitigations you've identified

## Scope

Security issues in scope:

- Unauthorized access to linked skill directories or archives
- Command injection via skill names, paths, or CLI arguments
- Unsafe symlink resolution that could escape the managed directory tree
- Information disclosure through `x-skills doctor` or other diagnostics

## Supported Versions

Only the latest tagged release receives security patches. The project is
pre-v1.0 — expect breaking changes until a stable release is declared.

## Disclosure Policy

Once a fix is ready, we will:

1. Release a patch version
2. Publish a security advisory on GitHub
3. Credit the reporter (unless they prefer to remain anonymous)
