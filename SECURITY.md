# Security Policy

## Reporting a vulnerability

**Please do not file public GitHub issues for security problems.**

Email **security@kestrai.dev** with:

- A description of the issue and the impact you believe it has.
- Steps to reproduce (a minimal proof-of-concept if you have one).
- The version, commit SHA, or release where you observed the issue.
- Whether the report is under embargo and any disclosure timeline you would like us to follow.

If you would prefer encrypted reporting, request our PGP key in your first message.

## Response targets

We will:

- Acknowledge your report within **2 business days**.
- Provide an initial triage assessment within **5 business days**.
- Aim to ship a fix or mitigation for confirmed high-severity issues within **30 days** of triage.

We follow [coordinated disclosure](https://en.wikipedia.org/wiki/Coordinated_vulnerability_disclosure). If you ask for an embargo, we will keep the issue private until a patched release is available or the agreed deadline passes — whichever comes first.

## Supported versions

Kestrai is **pre-1.0 (`v1alpha1`)**. Only the latest release on `main` receives security fixes during the alpha period. Once `v1` ships, this section will list the supported branches and their end-of-support dates.

## Scope

In scope:

- The `kestrai` binary (control plane, CLI, TUI).
- The Python agent SDK published from this repository.
- Official container images, Helm chart, and install script published from this repository.
- The reference `ModelProvider` and `Tool` implementations bundled with the release.

Out of scope:

- Third-party plugins, including `ModelProvider` and `Tool` plugins not maintained in this repository.
- Vulnerabilities in user-authored workflows, agents, or prompts.
- Issues that require already-compromised host access to exploit.
- Denial-of-service caused solely by user-controlled workload resource consumption (this is the operator's responsibility to bound via `Policy`).

## Safe harbor

Good-faith security research conducted under this policy will not be pursued under the [Computer Fraud and Abuse Act](https://www.law.cornell.edu/uscode/text/18/1030) or equivalent local statutes. "Good faith" means: you avoid privacy violations and service degradation, you do not exfiltrate user data beyond what is necessary to demonstrate the issue, and you give us a reasonable window to remediate before public disclosure.
