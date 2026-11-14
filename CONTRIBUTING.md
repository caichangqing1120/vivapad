# Contributing To VivaPad

VivaPad follows Motty's Issue-first contribution model. Keep VivaPad's product
behavior here and propose reusable editor/windowing capabilities separately
to [Viva](https://github.com/vivaui/viva). Do not claim complete Notepad++ parity
without the required platform and original-reference verification.

## From Requirement To PR

1. Open a feature or bug Issue with scope, expected behavior and acceptance
   criteria. For substantive changes, confirm unresolved decisions before
   implementation. Summarize the approved behavior and acceptance criteria in
   the Issue; keep detailed Specs, plans and work packages locally.
2. Comment `/claim`. Wait for `github-actions[bot]` to confirm the assignment.
   An existing assignment is not overwritten. Use `/unclaim` (or `/unclaimed`)
   to release your own claim.
3. Work in an isolated worktree on a focused branch such as
   `codex/issue-123-short-name`. Preserve unrelated work. Add a regression test
   before a behavior fix and keep implementation within the confirmed scope.
4. Run the checks below and update local plans and verification evidence.
   Keep headless, cross-compiled and native GUI results distinct. Report the
   commands and results in text without uploading local evidence files.
5. Open a PR containing `Closes #123`, `Fixes #123` or `Resolves #123` in its
   description. The Issue must be in this repository and assigned to the PR
   author. CI and Requirement policy must pass; only an authorized maintainer
   decides whether to merge. No bot auto-merges or publishes releases.
6. GitHub normally closes an Issue after its closing PR merges. When a linked
   PR merged without a closing keyword, its author/current assignee may use
   `/close`; the bot verifies author, repository and default branch first.

## Checks

Requires Go 1.26+, Node 22+ for automation tests, and Viva's platform build
prerequisites. The committed module replacement is a published, pinned fork,
not a sibling checkout. Do not silently replace it with upstream main.

```sh
test -z "$(gofmt -l $(git ls-files '*.go'))"
GOWORK=off go mod verify
GOWORK=off go vet ./...
GOWORK=off go test ./... -count=1
GOWORK=off go test -race ./... -count=1
node --test tools/github/*.test.cjs
node tools/github/check-public-files.cjs
GOWORK=off go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.12
```

## Local-Only Files

Do not commit or upload `docs/`, Specs, development plans, screenshots or
verification captures. Keep these files only in your local workspace,
including any evidence referenced by a plan. Do not attach them to Issues or
PRs. Public README, contribution rules, licenses and application resources
such as icons and fonts remain in Git.

The ignore rules prevent ordinary additions; never use `git add -f` to bypass
them. CI checks both the tracked files and reachable commit history, so merging
an old branch can also fail this policy even after deleting the files at its
tip. Rebase or recreate such a branch from the current clean default branch.
Before submitting, inspect `git diff --cached --name-only` and run the public
file check above. An image being placed under `assets/` does not make a
screenshot an application resource.

Native printer/trash tests that require explicit opt-in are not enabled in CI.
Never submit real print jobs or delete user files to satisfy a check. Remove
credentials and private documents from bug reports. Existing fixtures and
third-party licenses retain their bytes; original failure logs stay local.
