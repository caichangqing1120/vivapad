## Requirement

Closes #

## Scope

Describe the behavior change, non-goals and the Issue's confirmed acceptance
criteria. Keep detailed Specs and development plans local.

## Verification

List commands, results and platform evidence. State unverified behavior
explicitly; headless tests and cross-builds do not establish native GUI parity.

## Checklist

- [ ] The linked Issue is assigned to me after bot confirmation.
- [ ] Scope and acceptance criteria are confirmed; no blocking requirement decisions remain.
- [ ] Tests cover the changed behavior and relevant failure cases.
- [ ] Formatting, vet, full tests, race and CI are green.
- [ ] Local plans and verification records reflect the implementation without promoting unverified parity.
- [ ] No `docs/`, Specs, plans, screenshots or captures are included in commits or attachments.
- [ ] `node tools/github/check-public-files.cjs` passes, including commit history.
- [ ] No credentials, private documents or local build outputs are included.
- [ ] Reusable framework work is proposed separately to Viva; no unapproved dependency changes are included.
