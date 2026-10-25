---
name: ticket
description: Work a Campfire ticket (an identifier such as DEV-120) from investigation to pull request, stopping after the plan for the maintainer's decision. Use when asked to implement, fix or work on a ticket.
argument-hint: <ticket identifier> [trivial]
---

# Work a ticket

Follow CLAUDE.md throughout. The phases run in order, and phase 2 ends with a stop.

The ticket is any Campfire identifier, `<CODE>-<n>`. Below, `<ID>` is that identifier and `<id>` is
the same in lower case.

## 1. Understand

- Read the ticket with the `campfire` MCP server: `lookup_item`, then `list_comments` and the
  related items it names.
- Read the `SPEC.md` and `DESIGN.md` sections it touches.
- Read the code end to end: where the behaviour lives, every caller of what you'll change, and
  the tests that cover it. `git log -S` and the related tickets show why it is the way it is.
- Reproduce a bug before planning a fix.

## 2. Plan, then stop

Reply with:

- **Ask:** the ticket in one or two lines, and what it leaves unclear or gets wrong.
- **Found:** what in the code matters for this change (files, constraints, surprises).
- **Approach:** what you'll change. When there's a real choice, give 2–3 options with their
  trade-offs and say which you recommend and why. Include the simplest option that would do.
- **Tests:** which, if any, per CLAUDE.md "Tests".
- **Questions:** the decisions you need.

Then stop and wait for an answer. Write no code before the maintainer replies. You skip the stop
only when the arguments include `trivial`; in that case, state the plan in two lines and go on.

## 3. Build

- Work in a worktree (`EnterWorktree`, named `<id>-<slug>`) unless you're already in one.
- Make the change the plan agreed. If the plan turns out wrong, stop and say so instead of
  improvising a different one.

## 4. Verify

- `go test ./...` and `npm test`; the pre-commit hook runs them with gofmt and vet.
- If the client changed: run e2e once, and look at the change in the running app (the `run`
  skill). Check browser-dependent CSS or JS in Firefox.

## 5. Review your own diff

- Read all of `git diff origin/main...`.
- Run the `code-review` skill on it and fix what holds up.
- Check:
  - Every point of the ticket is done.
  - Nothing was added that the ticket didn't ask for.
  - No dead code or debug output is left.
  - The user-facing text reads plainly.
  - Nothing simpler would do.

## 6. Open the PR

- Add a change file, or use the `skip-changeset` label.
- Commit as `<ID>: <what changed>`, push, and run `gh pr create`.
- Keep the body short: what changed and why, how it was verified, what is left.
- Check CI once it finishes. If a failure isn't from your change, say so.

## 7. Report

Give the PR link, what you verified, what the review found, and anything the reviewer should look
at or decide.
