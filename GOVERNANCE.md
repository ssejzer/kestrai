# Governance

This document describes how decisions are made in the Kestrai project. It is intentionally short. As the project grows it will be amended in pull requests like any other change.

## Current model: BDFL

Until the criteria in the next section are met, Kestrai is governed by a single Benevolent Dictator For Life (BDFL): the project founder. The BDFL is responsible for:

- Setting and revising the technical roadmap.
- Resolving disputes between contributors when consensus cannot be reached on a pull request or issue.
- Approving the addition of new maintainers.
- Approving public releases.

In day-to-day work the BDFL operates as one reviewer among many. Maintainers can land changes via normal pull-request review; the BDFL only acts as a tiebreaker when reviewers disagree.

## Graduation to a maintainer council

Governance transitions to a maintainer council when **all** of the following are true:

- At least **5 maintainers** are active in the project.
- Those maintainers come from at least **3 different organizations** (where "organization" means any single employer, foundation, or self-funded individual).
- Each of those maintainers has been continuously active for at least **6 months**.

When this milestone is reached, the BDFL announces the transition in a public issue and amends this document with the council's procedures. The council makes technical decisions by majority vote of active maintainers. The BDFL retains a tiebreaker vote for **one additional year** after the transition, after which the council operates without a tiebreaker (ties default to "no change").

A foundation move (for example to CNCF or LF AI & Data) is explicitly **not** part of v1 and will be decided by the council, not by the BDFL.

## Becoming a maintainer

There is no application form. Maintainers are added by recognizing sustained contribution. A current maintainer (or the BDFL) opens an issue proposing the new maintainer and lists their notable contributions. Existing maintainers and the BDFL discuss and either accept or defer.

We expect new maintainers to have:

- Landed multiple non-trivial pull requests over at least 3 months.
- Reviewed others' pull requests substantively.
- Shown good judgment on contentious issues (technical, social, or both).
- Read and agreed to follow [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md).

A maintainer who has been inactive for **6 consecutive months** without notice may be moved to emeritus status by any other maintainer. Emeritus maintainers keep their name in the contributor list but no longer have merge rights; reactivation is automatic on their next merged PR.

## Licensing of contributions

Kestrai uses the GitHub default of **inbound = outbound**: by opening a pull request, the contributor licenses their contribution under the same [Apache 2.0](./LICENSE) license that covers the rest of the codebase. There is no CLA. There is no DCO sign-off requirement. CI does not check for `Signed-off-by` lines.

## Code of conduct enforcement

Code of conduct reports go to **conduct@kestrai.dev**. Enforcement decisions are made by the BDFL (or, after graduation, by a code-of-conduct committee appointed by the maintainer council). See [CODE_OF_CONDUCT.md](./CODE_OF_CONDUCT.md) for the full text and reporting process.

## Amendments

Changes to this document follow the normal pull-request process. Substantive changes (anything other than typos and formatting) require approval from the BDFL during the BDFL phase, and from a majority of the maintainer council after graduation.
