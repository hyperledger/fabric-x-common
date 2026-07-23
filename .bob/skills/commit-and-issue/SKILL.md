---
name: commit-and-issue
description: Write a commit message (which becomes the GitHub PR description verbatim) or open a GitHub issue for the Fabric-X Common repo, following the repo's PR template and conventions. Use whenever the user wants to commit staged changes, write or fix a commit/PR message, prepare a PR description, or open/file/create a GitHub issue for this project — even for terse asks like "commit this", "write the PR description", or "file an issue for this bug".
---

# Commit messages and GitHub issues

This repo squash-merges PRs, and **the commit message body is used verbatim as the GitHub
PR description**. So a commit message here is not a throwaway line — it is the PR
description, reviewed as-is. Write it to the PR template and keep the exact wording the
user approves.

The repo for all `gh` operations is **`hyperledger/fabric-x-common`** (the `gh` default when
run from this working tree).

Pick the workflow that matches the request:

- **Committing changes / writing a commit or PR message** → [Writing a commit message](#writing-a-commit-message)
- **Filing / opening / creating a GitHub issue** → [Opening a GitHub issue](#opening-a-github-issue)

The two compose: a commit's `#### Related issues` often points at an issue you just opened,
and an issue often names the PR that will resolve it.

## Shared conventions

**Component / area tag** — Titles (commit subjects and most issue titles) usually start with
an area tag naming the affected package or topic. This repo is **not strict about one style** —
skim `git log --oneline -30` and match recent precedent for the area you're touching. Three
styles coexist:

- **Bracketed** — `[api]`, `[dependency]` / `[dependencies]`, `[cryptogen]`, `[configtxgen]`,
  `[doc]`, `[notify-api]`. Most common for dependencies, API, and tool changes.
- **Colon-prefixed** — `configtx:`, `committerpb:`, `fix:`. Common for package-scoped changes.
- **No prefix** — a plain sentence subject is also accepted (e.g. `Add snapshot/checkpoint
  MALFORMED status codes`).

`[BREAKING]` is a **required** prefix (per `.github/pull_request_template.md`) when the change
breaks compatibility with other components — it can combine with an area tag, e.g.
`[BREAKING] [dependency] ...`. Common areas seen in this repo: `api` / `applicationpb` /
`committerpb` / `ordererpb` / `msppb`, `configtx`, `channelconfig`, `policies`, `msp`,
`protoutil`, `protolator`, `cryptogen`, `configtxgen`, `configtxlator`, `proto`, `dependency`,
`doc`. Pick the primary affected area and match how that area is usually tagged.

**Formal headers in the commit/PR body** — A commit message body (which is the PR
description) uses **only** the headers from `.github/pull_request_template.md`:
`#### Type of change`, `#### Description`, `#### Additional details (Optional)`,
`#### Related issues`. Never invent others in a commit/PR — no `#### Context`, `#### The gap`,
`#### Problem`, `#### Proposal`, `#### Motivation`, `#### Changes`, `#### Success criteria`,
etc. If something needs saying, it goes inside a formal section (usually `#### Description`),
not under a new heading. (Issues are *not* bound by this — they're less formal; see
[Opening a GitHub issue](#opening-a-github-issue).)

**Issue references** — When referring to an issue or PR **in this repo**, use `#NNN`. Issues
in **other Fabric-X repos** (orderer, committer, …) are referenced by **full URL**, e.g.
`https://github.com/hyperledger/fabric-x-orderer/issues/956` — this is how cross-repo work is
tracked here. The keyword in front decides whether merging a PR **auto-closes** the issue, and
the keyword is **always lowercase** (`resolves`, not `Resolves`):

- **Closing** (auto-closes on merge to the default branch): `resolves` / `fixes` / `closes`.
  Default to lowercase `resolves`, e.g. `- resolves #131`. (Closing keywords only auto-close
  same-repo issues; a full-URL cross-repo reference does not auto-close.)
- **Non-closing** (references without closing): `- resolves partly #625`, `- address #622`,
  `- related to #625`, `- follows up on #642` — all lowercase too. Use these when the change
  advances an issue but does not fully close it, or is merely related.

## Writing a commit message

### 1. Inspect what is being committed

Run these to ground the message in the actual change and current style:

```bash
git status
git diff --staged        # what will be committed; add `git diff` if nothing is staged yet
git log --oneline -30    # match the current area tags and phrasing
```

If nothing is staged and the user asked to commit, confirm what to stage (or stage per their
instruction) before writing the message. Don't guess at scope from unstaged noise.

### 2. Compose the subject line

Format: `[area] Short description` (or `area: Short description`, or a plain subject — match
precedent per [Shared conventions](#shared-conventions)).

- Keep it concise (roughly ≤ 70 chars) and specific about what changed.
- **Do not append `(#NNN)`** — GitHub adds the PR number on squash-merge automatically.
- Prefix `[BREAKING]` when the change breaks cross-component compatibility.
- Sentence case after the tag is typical; don't stress capitalization — match recent commits.

### 3. Compose the body from the PR template

The body **is** the PR description. Follow `.github/pull_request_template.md`, and **use only
its headers — do not invent new ones** (no `#### Changes`, `#### Motivation`, `#### The gap`,
`#### Success criteria`, etc.). Strip every HTML comment (`<!-- ... -->`) — the template says
to delete them before submitting, so they must not appear in the final message.

The formal headers, in order (all but Type of change and Description are optional):

```
#### Type of change

- <one or more of: Bug fix / New feature / Improvement (improvement to code, performance, etc) / Test update / Documentation update / Breaking change>

#### Description

- <Concise — see below.>

#### Additional details (Optional)

<Optional: implementation notes, how it was tested, deferred follow-ups, notes to reviewers.>

#### Related issues

- resolves #NNN
```

**Description — keep it concise; it is not a pitch.** The linked issue is the pitch: it
carries the motivation, the problem, and the gap. The Description's only job is to tell a
reviewer/developer *what this change does* — what they'll find in the diff. So:

- Lead with what changed, as a bullet list (`-`) when there's more than one point.
- Include only what is **not already in the linked issue** — e.g. a concrete implementation
  choice or a decision worth flagging (a regenerated `.pb.go`, an ASN.1 schema bump, a policy
  path added).
- Describe only what *this* diff changes. Do **not** restate the motivation, the problem, or
  the gap; do **not** propose future work; do **not** list success criteria — all of that is
  the issue's job.
- No selling, no background essay. A few tight bullets usually suffice; a one-line change can
  be one line.

**Type of change**: pick from the template list. A close variant is fine when it fits better
and matches recent commits (e.g. `Dependency update`). List several as bullets when the change
spans categories (e.g. a bug fix that is also a test update).

**Related issues — expected, not an afterthought.** Most changes here track an issue, and
keeping the Description lean depends on that issue holding the context. Ask which issue this
resolves. If there isn't one, treat that as a gap: offer to open one first (see
[Opening a GitHub issue](#opening-a-github-issue)) and link it, rather than silently shipping
without. Use the keyword semantics from [Shared conventions](#shared-conventions). Omit the
section only for a genuinely untracked, trivial change (e.g. a routine dependency bump).

**Never put in the body:** a hand-written `Signed-off-by:` trailer (added by `git commit -s`,
next step), a `---------` separator (inserted by GitHub's squash when it concatenates
commits), or **labels** — labels live on the PR/issue and are applied via `gh`
(e.g. `gh pr edit --add-label ...`), never written into the message text.

### 4. Show the message, then commit with sign-off

Because the wording ships verbatim as the PR description, **show the full drafted message and
get the user's approval before committing.** Adjust until it's exactly what they want.

Then commit. Write the message to a temp file and use `-F` so the multi-line body and blank
lines are preserved, and `-s` to add the DCO `Signed-off-by:` trailer from the user's git
config (this repo enforces DCO — every commit shows a `Signed-off-by:`):

```bash
git commit -s -F /tmp/commit-msg.txt
```

Amend with `git commit -s --amend -F /tmp/commit-msg.txt` if refining an existing commit.

The same body is the PR description verbatim — if a PR already exists or the user wants to
open one, reuse it with `gh pr create --body-file ...` or `gh pr edit --body-file ...`. Apply
any labels to the PR through `gh` when applicable (e.g. `gh pr edit --add-label dependencies`) —
never in the message body.

## Opening a GitHub issue

### 1. Understand the issue

Clarify what the issue is: a bug, a feature/improvement, a design discussion, a task. If it
references code, read the relevant files so the body can cite `file.go:line` and be concrete.

### 2. Title

Format: `[area] Short description`, matching the tag conventions in
[Shared conventions](#shared-conventions). Cross-cutting issues sometimes omit the prefix —
that's acceptable when no single area fits.

### 3. Body — the issue is where the case is made

Unlike the commit/PR, **issues are less formal** — there's no fixed template, so use whatever
structure fits the issue and suits the author. The one job is to make the case clearly and
concretely: what's broken or missing, and what to do about it, grounded in the code
(`file.go:line`) where relevant.

- A bug or small task can be a couple of sentences: what's wrong, where, and the impact.
- A feature or design proposal can use whatever sections make it readable (context, the gap,
  a proposal, alternatives, open questions, …) — pick what serves the reader.

This is where the motivation and background live (the commit/PR later just points back here),
so the depth the commit avoids is welcome here. Don't force the issue into the PR template.

### 4. Suggest labels, then create

Propose labels from the repo's label set and apply them once the user approves. Typically a
**type** label, plus a component/topic label when one fits. This repo's label set is small and
type-oriented:

- **Type / kind**: `bug`, `enhancement`, `documentation`, `test-coverage`, `code-hygiene`,
  `breaking`, `question`, `duplicate`, `invalid`, `wontfix`, `good-first-issue`, `help-wanted`.
- **Component / topic**: `dependencies`, `go`, `logging`, `snapshot`.

Labels evolve — run `gh label list --repo hyperledger/fabric-x-common` to confirm current
names before applying, and don't invent labels that don't exist. Labels are applied through
the `gh` command (`--label` below), never written into the issue body.

Show the proposed `gh issue create` command (title, body, labels) for approval, then run it
with `--body-file` so the multi-line body is preserved:

```bash
gh issue create --repo hyperledger/fabric-x-common \
  --title "[area] Short description" \
  --body-file /tmp/issue-body.md \
  --label enhancement --label snapshot
```

Report the created issue URL back to the user. If it should be referenced by an upcoming
commit/PR, add it to that commit's `#### Related issues`.
