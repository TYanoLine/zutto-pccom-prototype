# Codex handoff: Mapletown Network authenticated research collection

## Context

This task continues the Mapletown Network research started in PR #228.

Read first:

- `docs/HISTORICAL_ACCURACY.md`
- `docs/research/MAPLETOWN_NETWORK_EVIDENCE.md`
- `docs/research/BBS_SUBJECT_CORPUS.md`
- `docs/research/BBS_BODY_CORPUS.md`
- `scripts/research/sources.yaml`
- `scripts/research/archive_pccom_sources.py`
- repository `AGENTS.md`

The repository and current `main` are the source of truth. Before changing code, fetch the latest `main` and inspect whether the research tooling has changed since this handoff was written.

## Goal

Collect and analyze the HTTP-authenticated, read-only preserved Mapletown Network BBS material at <https://www.maple.town/> so it can be used as historical calibration evidence for Zutto PC Communication.

The immediate goal is **not** to clone Mapletown. It is to extract evidence about:

- article subject/body style;
- board and article structure;
- quoting and reply conventions;
- handles / IDs as displayed in preserved articles;
- timestamps and article numbering;
- ordinary station social behavior;
- greetings / absences / offline meetings / viewing parties / announcements;
- technical discussion vocabulary;
- 1995-1996 period-specific discussion;
- station administration and board evolution when visible.

Keep Mapletown-specific observations separate from generic host-program behavior.

## Rights and access

The project owner stated on 2026-09-27 that copyright/usage confirmation for this research collection has been obtained.

The preserved BBS area is read-only and uses shared HTTP authentication. The shared read-only credentials were provided by the project owner and are also intended for access to the archive.

**Do not commit credentials to Git.**

Prefer one of these approaches:

1. obtain the currently published shared read-only credentials from the public Mapletown top page at runtime; or
2. receive them through environment variables / secure task context.

If adding authenticated acquisition support to repository tooling, use environment variables such as:

- `MAPLETOWN_USER`
- `MAPLETOWN_PASSWORD`

Never print the password in logs, test snapshots, fixtures, PR descriptions, screenshots, or error messages.

Do not register, post, edit, delete, send mail, trigger interactive BBS actions, or perform any write operation. Collection must remain read-only.

## Existing state

As of PR #228:

- the public site description has been reviewed;
- the public board directory exposing `#1` through `#272` has been reviewed;
- derived station-ecology observations are in `docs/research/MAPLETOWN_NETWORK_EVIDENCE.md`;
- `scripts/research/sources.yaml` already contains the Mapletown top-level entry point;
- authenticated article bodies have **not** yet been archived;
- repository policy says third-party raw bodies belong outside Git.

Do not claim authenticated material has been collected until you have actually fetched and inspected it.

## Acquisition policy

Follow the existing conservative acquisition style in `scripts/research/archive_pccom_sources.py`.

Required defaults:

- single concurrent request;
- minimum 2 seconds between requests unless the site explicitly documents a stricter policy;
- descriptive user agent;
- preserve raw response bytes;
- preserve response metadata / useful HTTP headers;
- compute SHA-256 for acquired artifacts;
- record source URL and retrieval timestamp;
- detect/preserve original encoding before creating UTF-8 normalized derivatives;
- do not place third-party article bodies in Git;
- do not blindly recurse across the site;
- initially follow only URLs observed from fetched index/board pages.

If the existing collector is extended for Mapletown, keep authentication generic enough that secrets remain external to the repository. Do not hard-code Basic Auth strings.

## Recommended work sequence

### Phase 1: understand the authenticated URL and HTML structure

Use a browser or `curl` with read-only HTTP Basic authentication.

Inspect a very small number of pages first and determine:

- board URL shape;
- article-list URL shape;
- individual article URL shape, if separate;
- pagination;
- whether article content is one page or multiple pages;
- character encoding;
- whether links are stable;
- whether board/article numbers are explicit in URLs or markup;
- whether dates, handles, IDs, subjects, reply references, and quoted text can be parsed reliably.

Document the discovered structure before attempting broad collection.

### Phase 2: bounded representative acquisition

Do **not** start with all 272 boards.

Prioritize representative boards from `MAPLETOWN_NETWORK_EVIDENCE.md`, especially:

1. system information / system Q&A / proposal boards;
2. `サロン「メイプル」`, `フリー`, and `フリー'96`;
3. greetings / offline-meeting / viewing-party / announcement boards;
4. technical areas: computer, programs, CG, computer music, Internet;
5. several work-specific boards active around 1995-1996.

Aim first for enough material to validate the parser and article semantics, not maximum volume.

Where possible, deliberately include material near the project's target world year (1996).

### Phase 3: derived analysis

From the privately archived material, produce repository-safe observations and aggregate statistics. Useful outputs include:

- article counts by sampled board and year;
- subject length distribution;
- reply-prefix / continuation conventions;
- question-mark rates only as descriptive statistics, never generation quotas;
- common quoting forms;
- article-body length distribution;
- paragraph / line-break style;
- signatures and greetings;
- use of emoticons and period expressions;
- direct-address patterns;
- frequency and form of replies versus root posts;
- evidence of offline events and station social practices;
- technical vocabulary actually used around 1995-1996;
- visible board lifecycle / replacement / renaming evidence.

Avoid user profiling. Do not create dossiers on historical participants.

If raw material contains personal contact details, addresses, phone numbers, or other unnecessary personally identifying information, do not reproduce those details in repository documents.

## Historical evidence rules

Use the evidence classes from `docs/HISTORICAL_ACCURACY.md`:

- **Confirmed**: directly visible in preserved Mapletown material.
- **Station-specific**: behavior/configuration specific to Mapletown.
- **Likely / inferred**: reasonable reconstruction supported by several observations but not directly documented.
- **Fictional reconstruction**: service-side invention; do not blur it with source evidence.

Important: the Mapletown web preservation frontend is not evidence of the original BBS host-program UI or command/state-machine semantics.

Do not infer a host-program default from a Mapletown-specific board name, menu, or preservation-site layout.

## Expected repository changes

Prefer extending the existing research infrastructure rather than creating an unrelated one-off scraper.

Likely acceptable changes include:

- authenticated Mapletown support in `scripts/research/archive_pccom_sources.py` or a narrowly scoped companion script;
- repository-safe source metadata/configuration;
- derived analysis scripts that consume a local/private archive;
- updates to `docs/research/MAPLETOWN_NETWORK_EVIDENCE.md`;
- updates to `BBS_SUBJECT_CORPUS.md` / `BBS_BODY_CORPUS.md` if the evidence materially improves those corpora.

Raw HTML/article bodies, authenticated page dumps, screenshots containing article text, and credentials must remain outside Git.

If a local archive layout is needed, keep it consistent with the existing private archive convention, e.g. a `grassroots/mapletown/` subtree below an explicit `--output` directory.

## Parser/data model guidance

For each preserved article, capture only fields that are actually present. A derived record may include:

- source URL;
- board number and board name;
- article number;
- date/time as shown;
- displayed handle;
- displayed user/account ID if shown;
- subject;
- parent/reply reference if explicit;
- raw-body archive path;
- detected encoding;
- normalized text path;
- SHA-256;
- retrieval timestamp.

Do not invent missing metadata.

Keep raw captured facts separate from later interpretation.

## Validation

Before claiming success:

- manually inspect several archived raw pages and normalized outputs;
- compare parsed fields against the visible source;
- verify Japanese text round-trips correctly;
- verify credentials are absent from `git diff`, generated manifests committed to Git, and test fixtures;
- verify no third-party article body has been accidentally staged;
- confirm acquisition remains read-only;
- run relevant repository tests / script smoke tests and report exactly what was actually run.

If authentication or site behavior prevents collection, document the exact boundary and do not fabricate results.

## Deliverable

Open or update a PR containing:

1. safe/reproducible acquisition tooling or documented manual acquisition procedure;
2. repository-safe derived observations;
3. provenance and evidence classification;
4. a concise acquisition report: sampled boards, date ranges, number of pages/articles captured, failures/blocked areas;
5. implications for Zutto PC Communication, clearly separated from Mapletown-specific facts.

Do not merge to `main` unless the project owner's current instructions explicitly allow it.

## Current handoff branch

This handoff was added to:

- branch: `research/mapletown-evidence-20260927`
- PR: #228

Continue there if it is still open and current. If `main` has advanced materially, rebase/update carefully before further changes.
