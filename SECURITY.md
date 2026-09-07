# Security Policy

## Supported Versions

Security fixes are provided for the latest release and the current default branch. Older releases
and other branches are not supported. The default branch may contain unreleased changes; users who
need stable behavior should use the latest release.

## Report A Vulnerability

Report suspected vulnerabilities privately with GitHub's **private vulnerability reporting** for
this repository. Open the repository's **Security** tab, choose **Advisories**, and select **Report a
vulnerability**. Do not open a public issue, discussion, or pull request containing vulnerability
details, credentials, private keys, host names, fingerprints, catalog data, or exploit material.

Include only the information needed to reproduce and assess the problem safely:

- The affected Orza version or commit and operating system/architecture.
- A concise impact statement and the minimum synthetic reproduction steps.
- Relevant error text after removing credentials, host data, paths, and other personal information.
- Whether the issue is known to be reachable, and any suggested mitigation.
- A safe way to contact you through the private advisory if follow-up is needed.

Use synthetic `.invalid` hosts and placeholder credentials. Never submit a working secret. If the
private reporting control is unavailable, do not publish details; wait for it to be restored.

The maintainers will acknowledge the private report, assess impact and reachability, coordinate a
fix and release, and agree on coordinated disclosure with the reporter. Please allow that process to
finish before public disclosure. No response deadline or bounty is promised.

## Dependency Findings

Go vulnerability IDs use the form `GO-YYYY-NNNN`. Dependency presence alone does not establish that
Orza can invoke the vulnerable code, so reports should include reachability information when
possible. Reachable findings block validation by default. A temporary exception is permitted only
when the repository records the exact ID, owner, rationale, mitigation, tracking issue, and a future
expiration date in `.vulnerability-exceptions.json`; expired, stale, malformed, or broad exceptions
fail closed.
