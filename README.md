# shipped

A personal mirror for your GitHub work: what you ship, and what you review.

`shipped` collects the pull requests you author and the ones you review, then
renders one self-contained HTML dashboard. No server, no account, no background
process. Nothing leaves your machine except GitHub API calls made through your
own `gh` login.

## Install

```sh
go install github.com/peterulsteen/shipped@latest
```

Or download a binary from [Releases](https://github.com/peterulsteen/shipped/releases).

Requires the [GitHub CLI](https://cli.github.com), authenticated with `gh auth login`.
`shipped` never handles a token; every request goes through `gh`.

## Quick start

```sh
shipped init       # finds your login, asks which orgs to include (blank = all of GitHub)
shipped collect    # the last 90 days on first run, incremental after that
shipped open       # render and open the dashboard
```

`shipped collect --backfill` reaches back three years. `shipped config` shows where everything lives.

## What it shows

| | |
|---|---|
| **Shipping** | PRs opened and merged per day, open backlog, time to merge (median and p90) |
| **Reviewing** | reviews given and received per day, how long your PRs wait for a first review, how long you make others wait |
| **Size** | authored vs. raw lines changed, median PR size |
| **Sustainability** | share of PRs opened evenings and weekends, age of your oldest open PR |

Every chart has a hover readout and a data table.

### What it won't let you misread

**Generated lines are not authored lines.** Lockfiles, build output, snapshots,
fixtures, and any single file over 2,000 changed lines are counted separately —
one large test fixture can otherwise outweigh a month of real work.

**A fast first review is not necessarily a human one.** GitHub reports every
reviewer as a user, so a review bot looks like a colleague. When one reviewer is
first on half or more of your PRs, the dashboard says so beside the wait time
rather than implying people engaged within minutes.

**A truncated list is not a complete one.** GitHub returns at most 100 files and
100 reviews per pull request. When a PR in view exceeds either, the page says so,
with the most its totals could be low by.

## Configuration

`~/.config/shipped/config.toml` (respects `$XDG_CONFIG_HOME`). Every key is optional.

```toml
author = ""                 # blank = the authenticated gh user
orgs   = ["my-org"]         # blank = all of GitHub
repos  = []                 # owner/name

# Files counted as generated rather than authored. The built-in list covers
# common lockfiles (npm, Cargo, Go, Terraform, Nix, Gradle and more), build
# output, vendored code, snapshots, and fixtures. Extend it:
extra_generated_paths = ["third_party/"]
# ...or replace it outright, and maintain the whole list yourself:
# generated_paths    = ["dist/", "vendor/"]
# generated_suffixes = [".snap", ".min.js"]

big_file_lines = 2000       # 0 disables the size rule
window_days    = 90
rolling_days   = 7
workday_start  = 8          # after hours = outside this range, plus weekends
workday_end    = 18
```

`shipped init` writes only `author`, `orgs`, and `repos`. Everything else falls
back to the built-in defaults, so upgrading shipped can improve them.

`collect` also takes `--org`, `--repo`, `--author`, and `--since` for one-off runs.
Data lives in `~/.local/share/shipped/` (respects `$XDG_DATA_HOME`).

## Scheduling

Nothing runs unless you run it. For a daily refresh, use your OS scheduler —
and make sure its `PATH` includes both `shipped` and `gh`, since schedulers start
with a minimal environment:

```sh
15 7 * * *  shipped collect && shipped render
```

## License

[MIT](LICENSE)
