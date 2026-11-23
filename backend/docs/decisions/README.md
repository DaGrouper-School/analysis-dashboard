# Decisions

Decisions taken while building this service are recorded here, one per file, numbered in order: `0001-short-title.md`, `0002-…`.

- A record is never edited afterwards to say something new. A later record supersedes it and says which record it supersedes. The only change to the superseded record is its status line, which names the record that replaces it.
- The design's starting decisions, with the alternatives considered, live in [`docs/design/`](../design/README.md). A record here is needed when a decision changes or adds to the design, and the design document is updated to match.

No decisions have been recorded yet. Each new record is added to a table here, with its number, title and status.

## Template

```markdown
# NNNN. The decision, as a sentence

Status: Accepted. (Or: Superseded by [NNNN](NNNN-title.md).)

## Context

What forces the decision: the problem, the constraints, what the design said before.

## Decision

What is decided, in numbered points where there are several.

## Consequences

What follows, good and bad, and how the design documents change.
```
