# Implementation Plan: `dots` — standalone, repo-agnostic dotfiles manager

**Status:** Draft · **Author:** @nox456 · **Issue:** `none` · **PRD:** `none` (requirements settled by interview, recorded in Design decisions) · **Updated:** 2026-09-12

## Context

`bin/dots` in the omarchy dotfiles repo is a 331-line bash script that keeps config under git by
holding the real files in the repo and symlinking them into place. The mechanism is sound and
repo-agnostic; the implementation is not — `REPO` is derived from the script's own location
(`bin/dots:16`), so the tool can only ever manage the one repo it lives inside, and three
behaviors are hardcoded to Omarchy (`scan_plugins` at `bin/dots:148`, the `~/Pictures/bg` →
`wallpapers` mapping at `bin/dots:262`, and `manifest/plugins.txt`).

Three repos on this machine want the same mechanism and currently each solve it differently:

| Repo | Path | Remote | Today's mechanism |
| --- | --- | --- | --- |
| omarchy | `~/Documents/Projects/omarchy` | `nox456/omarchy` | `bin/dots` + `links.conf` |
| CLAUDE.md | `~/Documents/Projects/CLAUDE.md` | `nox456/CLAUDE.md` | `install.sh`, a second hand-rolled linker |
| neovim | `~/.config/nvim` | `nox456/neovim-config` | plain `git clone` in place, no linking |

This plan extracts the mechanism into a standalone Go CLI, `dots`, that manages all three from a
central registry.

**Current behavior:**

- `bin/dots:53` — `each_link` reads `links.conf` (two whitespace-separated columns) and dispatches
  a callback per entry. `link` / `status` / `doctor` are three callbacks over that one driver.
- `bin/dots:75` — `links_to_repo` compares `readlink -f` on both sides; this is the health test.
- `bin/dots:67` — `backup` copies anything about to be touched to
  `~/.local/state/omarchy-dots/backups/<timestamp>/`, deliberately outside `~/.config`.
- `bin/dots:119` — `status_one` classifies each entry as linked / foreign / drifted / unadopted /
  unlinked / missing, printing healthy entries only under `-v`.
- `bin/dots:165` — `git_changes` parses `git status --porcelain`, sorting working-tree changes
  before staged ones (`bin/dots:186-188`) and capping output at 12 rows.
- `bin/dots:213` — `doctor_one` repairs the specific damage Omarchy migrations cause: `sed -i`
  replaces a symlink with a regular file, so the migrated content is taken back into the repo and
  the symlink restored. Driven by `bin/relink-dotfiles.hook`, installed as a `post-update` hook.
- `bin/dots:331` — `exit $DRIFT`: non-zero only for a broken *link*; pending commits exit 0.
- `~/Documents/Projects/CLAUDE.md/install.sh:33` — links `skills/*/` one symlink per directory and
  prunes links whose repo source was deleted. **Today's `links.conf` cannot express this**, which is
  why that repo has a second linker at all.
- `~/.claude/skills/` currently holds 13 symlinks: 11 into the CLAUDE.md repo, and 2
  (`diagnose-crash`, `omarchy`) into `/usr/share/omarchy/default/agents/skills/`. Any prune logic
  must leave the latter two alone.

## Technical requirements

| ID | Requirement | Why it matters |
| --- | --- | --- |
| TR-1 | Every path `dots` deletes or overwrites is copied to `$XDG_STATE_HOME/dots/backups/<RFC3339>/<repo>/` **before** the mutation. If the backup fails, the mutation does not run. | `unlink` deletes the repo copy and `doctor` overwrites it; a failed copy with a completed delete is unrecoverable data loss. |
| TR-2 | `link`, `doctor`, `bootstrap` and `manifest install` are idempotent: a second consecutive run performs no filesystem mutation and exits 0. | The post-update hook runs `doctor` after every `omarchy update`; a non-idempotent run would churn backups on every system update. |
| TR-3 | A symlink at a managed system path whose target does **not** resolve inside the owning repo is reported and never modified, deleted, or followed. | `~/.claude/skills/omarchy` and `diagnose-crash` point into `/usr/share/omarchy/`. Preserves `bin/dots:89` and `bin/dots:218` behavior. |
| TR-4 | Glob prune deletes a path only when all three hold: it is a symlink; its target resolves inside the owning repo's root; that target does not exist. Never a regular file, never a directory, never a foreign link. | Prune is the only delete-by-default operation in the tool. TR-3 is the specific case that must survive it. |
| TR-5 | Exit code is non-zero if and only if at least one link is in a non-healthy state. Uncommitted git changes, untracked scan hits, and manifest gaps all exit 0. | Preserves the `bin/dots:331` contract so `status` stays usable in scripts; pending commits are the normal steady state. |
| TR-6 | A command string from repo config (`hooks.post_link`, `manifest.install`) is printed in full and requires an interactive `y/N` confirmation before execution. With no TTY it is skipped and reported, never executed. | Arbitrary code execution sourced from a file in a cloned git repo. Without the TTY rule, `bootstrap` in CI or over SSH silently executes it. |
| TR-7 | A failure on one link is reported and processing continues to the next; the command's exit code reflects the failure. | Matches `bin/dots:103,105` (`err` then `return`, loop continues). One unreadable path must not abort a 40-link restore. |
| TR-8 | Path resolution expands a leading `~`, resolves to absolute, and does **not** resolve the final symlink component. Every glob match must resolve inside the owning repo root; a match escaping it is an error. | `bin/dots:45` `abs()` deliberately preserves the final symlink so state can be classified. The containment rule stops `repo = "../../etc/*"` from making the prune rule dangerous. |
| TR-9 | No ANSI escape sequences are emitted when stdout is not a terminal. | `bin/dots:27`. `relink-dotfiles.hook` captures output into a notification body. |
| TR-10 | `dots doctor --quiet` exits 0 and prints nothing when every link is healthy. | `bin/relink-dotfiles.hook:14` tests the captured output for emptiness to decide whether to notify. Breaking this fires a desktop notification after every `omarchy update`. |
| TR-11 | Registry and repo config parse failures name the file and the line, and abort that repo only — other registered repos still process. | A malformed `dots.toml` in one repo must not make `dots status` useless for the other two. |
| TR-12 | `dots` never writes to git: no `add`, `commit`, `push`, or `checkout` in any code path except `bootstrap`'s `git clone` of a missing repo. | Explicitly rejected in interview. `status` reads `git status --porcelain` only. |

## Design decisions

| # | Fork | Chosen | Rejected | Why |
| --- | --- | --- | --- | --- |
| D-1 | How does `dots` find the repo it acts on? | Central registry at `$XDG_CONFIG_HOME/dots/config.toml`, repos addressed by name, no argument = all | cwd walk-up (git-style); registry + cwd; explicit `--repo` | Three repos in three unrelated locations, one of which (`~/.config/nvim`) you never `cd` into. Name-addressing makes `dots status` a whole-machine check. |
| D-2 | Config format | One `dots.toml` per repo | Two-column `links.conf`; columns + separate settings file | Declarative per-repo settings (globs, adopt map, manifest, hooks) do not fit two columns, and a second file is two things to keep in sync. |
| D-3 | Language | Go | Bash + hand-rolled TOML subset; bash + `yq`; Rust | TOML needs a real parser; a hand-rolled one silently mis-parses. Go matches `forgectl` and `forgesync` (`cmd/` + `internal/`, cobra), and its stdlib covers the whole problem: `os.Lstat`, `os.Readlink`, `os.Symlink`, `filepath.EvalSymlinks`, `filepath.Glob`, `exec.Command`. |
| D-4 | TOML library | `github.com/pelletier/go-toml/v2` used directly | `spf13/viper` (as `forgesync/internal/config/config.go:6` does) | Viper models *one* global config with env overlay; here there are N per-repo files and no env layering. `go-toml/v2` is already in the `forgesync` dependency graph, so it is not a new vendor. Registry and repo config use the same parser. |
| D-5 | Omarchy-specific logic | Declarative config: `[settings].scan`, `[adopt].map`, `[manifest]`, `[hooks]` | Per-repo hook scripts; drop entirely; config + hooks | Every Omarchy behavior is data (a glob, a mapping, a command template). Hook scripts would reintroduce an executable-per-repo, which is what this plan is removing. |
| D-6 | One-to-many links | Globs on the repo side, with prune | Globs without prune; explicit line per skill | `install.sh` exists purely because `links.conf` cannot express `skills/*`. Without prune the tool cannot replace it. Prune is constrained by TR-4. |
| D-7 | Reverse of `adopt` | `unlink` restores the real file to the system path, removes the entry, deletes the repo copy, backs up first | Keep repo copy; split `unlink` / `untrack` | One verb, fully reversing `adopt`. Deletion is visible as a git deletion and recoverable from both git history and the TR-1 backup. |
| D-8 | `bootstrap` scope | Clone when missing, link, then offer `hooks.post_link`; no-arg form walks the whole registry | Manifest install as part of bootstrap | Manifest install is a separate explicit command (D-9) so a fresh-machine restore never installs third-party plugins without being asked. |
| D-9 | Manifest | `dots manifest install <repo>` as its own command; `status` reports gaps but never executes | Report-only; drop entirely | Keeps the README copy-paste pipeline as a first-class command while keeping execution out of `bootstrap`. |
| D-10 | `hooks.post_link` execution | Print, then `y/N` confirm (TR-6) | Run automatically; opt-in `--post-link` flag | The command comes from a file inside a cloned repo. Confirmation costs one keystroke per repo, on fresh-machine setup only. |
| D-11 | `links.conf` migration | Hand-converted once to `dots.toml` as part of step 13; the tool never reads the old format | `dots migrate` command; read both formats | Three conversions total, then the code path is dead weight forever. |
| D-12 | Neovim's model | Relocate the clone to `~/Documents/Projects/nvim` and link `~/.config/nvim` → it | Clone-in-place mode in the registry; leave nvim unmanaged | One model for all three repos. A clone-in-place mode would be a second state machine (git-state-only, no links) for exactly one repo. Carries the relocation risk in R-1. |
| D-13 | Distribution | Own repo `github.com/nox456/dots`, installed by symlinking the built binary into `~/.local/bin` (already on `PATH`) | Public + AUR + semver + CI; keep it inside the omarchy repo | Personal tool. Packaging can be added later without changing the tool. |
| D-14 | Registry bootstrapping on a bare machine | `dots bootstrap <git-url>` clones one repo by URL; that repo's `dots.toml` links the registry file into `$XDG_CONFIG_HOME/dots/config.toml`; subsequent `dots bootstrap` reads it | Registry created by hand on every new machine | Resolves the chicken-and-egg: the registry is itself dotfiles. The omarchy repo owns `config/dots/config.toml`. |

## Contracts

These two schemas are what every other step depends on; they land first (build step 2).

```toml
# $XDG_CONFIG_HOME/dots/config.toml — NEW (registry)
# On this machine, a symlink into the omarchy repo (D-14).

[repo.omarchy]
path = "~/Documents/Projects/omarchy"
url  = "https://github.com/nox456/omarchy.git"

[repo.claude]
path = "~/Documents/Projects/CLAUDE.md"
url  = "https://github.com/nox456/CLAUDE.md.git"

[repo.nvim]
path = "~/Documents/Projects/nvim"
url  = "https://github.com/nox456/neovim-config.git"
```

```toml
# <repo>/dots.toml — NEW (per-repo config). Every section below [[link]] is optional.

# repo   — path relative to the repo root. A trailing /* makes it a glob (D-6).
# system — absolute or ~-prefixed. For a glob, a trailing / marks the parent
#          directory each match is linked into, by basename.
[[link]]
repo   = "config/hypr/bindings.lua"
system = "~/.config/hypr/bindings.lua"

[[link]]
repo   = "config/omarchy/plugins/*"
system = "~/.config/omarchy/plugins/"

[settings]
# Directories whose unexpected children `status` should flag. Replaces scan_plugins
# (bin/dots:148). A match carrying its own .git is reported as informational only.
scan = ["~/.config/omarchy/plugins/*/"]

[adopt]
# First match wins. Replaces the hardcoded case block at bin/dots:261-269.
map = [
  { system = "~/.config/*",   repo = "config/*" },
  { system = "~/Pictures/bg", repo = "wallpapers" },
]

[manifest]
file    = "manifest/plugins.txt"   # one item per line, # comments ignored
install = "omarchy plugin add --yes {}"   # {} substituted per line

[hooks]
post_link = "hyprctl reload && omarchy restart shell"
```

```go
// internal/link/state.go — NEW
// The classification every command branches on. Extends bin/dots:119-144 with Orphan,
// which only a glob link can produce (D-6).
type State int

const (
    Linked    State = iota // symlink resolving to the repo source
    Foreign                // symlink resolving outside the repo — never touched (TR-3)
    Drifted                // real file at system AND a source in the repo — doctor's case
    Unadopted              // real file at system, nothing in the repo
    Unlinked               // source in the repo, nothing at system
    Missing                // neither side exists
    Orphan                 // symlink into this repo whose source is gone — prune candidate (TR-4)
)

// internal/link/resolve.go — NEW
// Expands globs and classifies. Pure: no filesystem mutation, so `status` and the
// dry-run path share exactly the code the mutating commands use.
type Resolved struct {
    RepoPath   string // repo-relative, glob already expanded
    SystemPath string // absolute
    Source     string // absolute path inside the repo
    State      State
    LinkTarget string // set when State is Foreign or Orphan
}

func Resolve(repoRoot string, links []config.Link) ([]Resolved, error)
```

## Scoped changes

Buckets are the Go packages of the new `dots` module, plus the three managed repos and the
machine-level install.

### CLI surface — `cmd/dots`, `internal/cli`

| Artifact | Change | What & why |
| --- | --- | --- |
| `cmd/dots/main.go` | NEW | Entry point; calls `cli.Execute()`. Mirrors `forgesync/cmd/forgesync/main.go`. |
| `internal/cli/root.go` | NEW | Cobra root, persistent flags `-v/--verbose`, `--quiet`, `--dry-run`, `--yes`. `PersistentPreRunE` loads the registry and resolves the repo argument to a set. |
| `internal/cli/status.go` | NEW | `dots status [repo]`. Composes resolve + scan + git report. Exit code per TR-5. |
| `internal/cli/link.go` | NEW | `dots link [repo]`. Adopts unadopted paths, creates missing links, prunes orphans. |
| `internal/cli/doctor.go` | NEW | `dots doctor [repo]`. Repairs Drifted and Unlinked; ports `bin/dots:213-240`. Must satisfy TR-10. |
| `internal/cli/adopt.go` | NEW | `dots adopt <repo> <path> [repo-path]`. Applies `[adopt].map`, appends a `[[link]]`, links it. |
| `internal/cli/unlink.go` | NEW | `dots unlink <repo> <path>`. Restore, remove entry, delete repo copy, backup first (D-7, TR-1). |
| `internal/cli/bootstrap.go` | NEW | `dots bootstrap [repo\|<git-url>]`. Clone-if-missing → link → confirm `post_link` (D-8, D-14). |
| `internal/cli/manifest.go` | NEW | `dots manifest install <repo>`. Per-line confirm, `{}` substitution (D-9, TR-6). |
| `internal/cli/repo.go` | NEW | `dots repo list\|add\|remove`. Minimum registry editing so the file need not be hand-written. |
| `internal/cli/version.go` | NEW | `dots version`, matching `forgesync/internal/cli/version.go`. |

### Config parsing — `internal/config`

| Artifact | Change | What & why |
| --- | --- | --- |
| `internal/config/registry.go` | NEW | Load/save `$XDG_CONFIG_HOME/dots/config.toml`; resolve a name (or all) to repo roots. |
| `internal/config/repoconf.go` | NEW | Load `<repo>/dots.toml` into typed structs; append a `[[link]]` on adopt, remove one on unlink, preserving comments and key order. |
| `internal/config/path.go` | NEW | `~` expansion, absolute-without-resolving-final-symlink, repo-containment check (TR-8). Ports `bin/dots:38-50`. |
| `internal/config/errors.go` | NEW | Parse errors carrying file and line, scoped to one repo (TR-11). |

### Link engine — `internal/link`

| Artifact | Change | What & why |
| --- | --- | --- |
| `internal/link/state.go` | NEW | The `State` enum from Contracts. |
| `internal/link/resolve.go` | NEW | Glob expansion and classification. Pure, side-effect free. |
| `internal/link/apply.go` | NEW | The mutations: adopt-into-repo, create symlink, restore-to-system, prune. Honours `--dry-run`. |
| `internal/link/prune.go` | NEW | Orphan detection under the three TR-4 conditions. Isolated because it is the only default-delete path. |

### Filesystem primitives — `internal/fsop`

| Artifact | Change | What & why |
| --- | --- | --- |
| `internal/fsop/backup.go` | NEW | Timestamped backup under `$XDG_STATE_HOME/dots/backups/` (TR-1). Ports `bin/dots:67-73`, adding the repo name to the path now that repos are plural. |
| `internal/fsop/copy.go` | NEW | Recursive copy preserving mode and symlinks — the `cp -a` semantics `bin/dots:105` relies on. |

### Reporting — `internal/report`, `internal/scan`, `internal/gitstat`

| Artifact | Change | What & why |
| --- | --- | --- |
| `internal/report/printer.go` | NEW | `✓ ! ✗ · ~` markers, TTY colour detection (TR-9), `--quiet`/`-v` gating. Follows the `forgesync/internal/output` printer shape. |
| `internal/scan/scan.go` | NEW | `[settings].scan` globs → untracked children, `.git` presence demoting to informational. Ports `bin/dots:148-161`. |
| `internal/gitstat/gitstat.go` | NEW | `git status --porcelain --untracked-files=all`, working-tree before staged, 12-row cap. Ports `bin/dots:165-209`. |

### Managed repo — omarchy

| Artifact | Change | What & why |
| --- | --- | --- |
| `dots.toml` | NEW | Hand-converted from `links.conf` (D-11); the 4 plugin lines collapse to one `config/omarchy/plugins/*` glob. Carries `[settings].scan`, `[adopt].map`, `[manifest]`, `[hooks].post_link`. |
| `config/dots/config.toml` | NEW | The registry itself, tracked here and linked to `$XDG_CONFIG_HOME/dots/config.toml` (D-14). |
| `links.conf` | DELETE | Superseded by `dots.toml`. |
| `bin/dots` | DELETE | Replaced by the standalone binary. |
| `bin/relink-dotfiles.hook` | MODIFY | Point `DOTS` at `~/.local/bin/dots` and call `dots doctor omarchy --quiet`, removing the hardcoded repo path noted in `README.md`. |
| `README.md` | MODIFY | Rewrite the `bin/dots` and "Restoring on a fresh install" sections around `dots bootstrap`. |

### Managed repo — CLAUDE.md

| Artifact | Change | What & why |
| --- | --- | --- |
| `dots.toml` | NEW | `CLAUDE.md` → `~/.claude/CLAUDE.md`, plus `skills/*` → `~/.claude/skills/` as a glob link. |
| `install.sh` | DELETE | Its whole reason for existing was `skills/*` and prune (D-6), now both in the tool. |
| `README.md` | MODIFY | Replace `./install.sh` instructions with `dots link claude`. |

### Managed repo — neovim

| Artifact | Change | What & why |
| --- | --- | --- |
| `~/.config/nvim` → `~/Documents/Projects/nvim` | MODIFY | Relocate the clone and link it back, per D-12. Carries R-1. |
| `dots.toml` | NEW | Single link: `.` → `~/.config/nvim`. |

### Config & infra

| Artifact | Change | What & why |
| --- | --- | --- |
| `go.mod` | NEW | `module github.com/nox456/dots`, Go 1.27 (installed), requiring `spf13/cobra` and `pelletier/go-toml/v2` — both already used across your Go repos. |
| `Makefile` | NEW | `build`, `install` (symlink into `~/.local/bin`), `fmt`, `vet`. Gives the verification steps a stable command name. |
| `README.md` | NEW | Config reference for both schemas, and the command list. |
| `.gitignore` | NEW | Ignore the built `dots` binary. |

## Build sequence

1. **Scaffold** — `go.mod`, `cmd/dots/main.go`, `internal/cli/root.go` + `version.go`, `Makefile`, `.gitignore`. Covers: the Config & infra bucket. Satisfies: TR-9 (printer wiring only).
   **Verify:** `make build && ./dots version` prints a version; `./dots --help` lists the commands.
2. **Config contracts** — `internal/config/*`. Both schemas parse into typed structs; path handling and repo containment land here. Covers: `internal/config`. Satisfies: TR-8, TR-11.
   **Verify:** `go vet ./...`; `./dots repo list` against a hand-written registry naming all three real repos prints three rows with correct paths.
3. **Resolve + state machine** — `internal/link/state.go`, `resolve.go`. Pure classification including glob expansion. Covers: those two files. Satisfies: TR-3, TR-8.
   **Verify:** `./dots status omarchy` (read-only, safe against live config) classifies all 17 current `links.conf` entries as `Linked`, and `./dots status claude` reports 12 `Linked` (the `CLAUDE.md` file plus 11 skill directories) and the 2 `/usr/share/omarchy/` entries as `Foreign` — never as `Orphan`.
4. **Backup + copy primitives** — `internal/fsop/*`. Covers: that bucket. Satisfies: TR-1.
   **Verify:** run `./dots adopt` (step 8) against a throwaway file in `/tmp` and confirm a copy lands under `$XDG_STATE_HOME/dots/backups/<ts>/`.
5. **`status`** — `internal/report/printer.go`, `internal/scan`, `internal/gitstat`, `internal/cli/status.go`. Covers: the reporting bucket. Satisfies: TR-5, TR-9, TR-12.
   **Verify:** `./dots status` reports all three repos; `./dots status omarchy; echo $?` exits 0 with the tree clean; `./dots status omarchy | cat` contains no ESC bytes (`./dots status omarchy | cat -v | grep -c '\^\['` is 0).
6. **`link`** — `internal/link/apply.go`, `internal/cli/link.go`, with `--dry-run`. Covers: those files. Satisfies: TR-2, TR-7.
   **Verify:** `./dots link omarchy --dry-run` reports zero mutations against the already-linked live config; `./dots link omarchy` twice in a row produces no new backup directory.
7. **`doctor`** — `internal/cli/doctor.go`. Covers: that file. Satisfies: TR-2, TR-10.
   **Verify:** `./dots doctor omarchy --quiet` prints nothing and exits 0. Then `cp --remove-destination ~/Documents/Projects/omarchy/config/foot/foot.ini ~/.config/foot/foot.ini` to simulate a `sed -i` migration, re-run, and confirm the file is a symlink again and `git -C ~/Documents/Projects/omarchy status` is clean.
8. **`adopt`** — `internal/cli/adopt.go`, `[adopt].map` application, `dots.toml` append. Covers: that file. Satisfies: TR-1.
   **Verify:** adopt a scratch file under `~/.config/`, confirm the `[[link]]` appended to `dots.toml` and the symlink created; then `./dots unlink` it back in step 9.
9. **`unlink`** — `internal/link/prune.go`, `internal/cli/unlink.go`. Covers: those files. Satisfies: TR-1, TR-4, TR-7.
   **Verify:** unlink the step-8 scratch file: real file back at the system path, entry gone from `dots.toml`, repo copy gone, backup present. Then confirm `./dots link claude --dry-run` lists **no** prune action for `~/.claude/skills/omarchy` or `diagnose-crash` (TR-3/TR-4 regression check).
10. **`manifest install`** — `internal/cli/manifest.go`. Covers: that file. Satisfies: TR-6.
    **Verify:** `./dots manifest install omarchy` against the current all-comments `manifest/plugins.txt` runs nothing and exits 0; piping stdin from `/dev/null` shows the skip-without-TTY path.
11. **`bootstrap`** — `internal/cli/bootstrap.go`. Covers: that file. Satisfies: TR-2, TR-6.
    **Verify:** `./dots bootstrap omarchy` on the already-cloned repo skips the clone, reports links healthy, prints `hyprctl reload && omarchy restart shell` and waits for confirmation; answering `n` exits 0 having run nothing.
12. **Migration — write the configs** — `omarchy/dots.toml`, `omarchy/config/dots/config.toml`, `CLAUDE.md/dots.toml`. No deletions yet, so both old mechanisms still work. Covers: the `dots.toml` NEW rows in the two managed-repo buckets.
    **Verify:** `./dots status` reports all links healthy for omarchy and claude, with the omarchy plugin glob expanding to the same 4 links `links.conf` listed.
13. **Migration — relocate neovim** — move `~/.config/nvim` to `~/Documents/Projects/nvim`, add its `dots.toml`, register it, link it. Covers: the neovim bucket. Carries R-1.
    **Verify:** `ls -l ~/.config/nvim` is a symlink into the new location; `nvim --headless "+checkhealth" +qa` reports no new errors; `git -C ~/Documents/Projects/nvim status` is clean.
14. **Cutover** — install the binary to `~/.local/bin/dots`; delete `omarchy/bin/dots`, `omarchy/links.conf`, `CLAUDE.md/install.sh`; update `bin/relink-dotfiles.hook` and both READMEs. Covers: every DELETE and MODIFY row.
    **Verify:** `command -v dots` resolves to `~/.local/bin/dots`; `dots status` from a directory outside all three repos reports all three; re-install the hook with `omarchy hook install post-update bin/relink-dotfiles.hook` and confirm `bash ~/.config/omarchy/hooks/post-update.d/relink-dotfiles.hook` prints nothing and exits 0.

**Parallelizable after step 5:** steps 6-11 are one command each over the step 2-4 contracts and can be taken in any order. Steps 12 and 13 are independent of each other once 6 and 7 are green. Step 14 requires all of them.

## Verification

Commands are the ones this plan's step 1 defines (`Makefile`) plus the Go toolchain already
installed (`go version go1.27.0`).

| Level | What it proves | How |
| --- | --- | --- |
| Build & static analysis | The module compiles and passes vet on every package | `make build && go vet ./...` |
| Formatting | Matches the house style of `forgectl` / `forgesync` | `gofmt -l .` prints nothing |
| Read-only against live config | Classification is correct on 3 real repos and ~30 real links, including the 2 foreign `/usr/share/omarchy/` links | `dots status -v` — expect omarchy and claude fully `Linked`, `diagnose-crash` and `omarchy` under `~/.claude/skills/` reported `Foreign` |
| Idempotency | TR-2 holds for the two commands the hook and bootstrap re-run | `dots link omarchy && dots link omarchy` — second run creates no backup directory under `$XDG_STATE_HOME/dots/backups/` |
| Hook contract | TR-10 holds, so `omarchy update` stays silent when nothing drifted | `out=$(dots doctor omarchy --quiet 2>&1); [ -z "$out" ]; echo $?` prints 0 |
| Failure path — drift | `doctor` repairs the exact damage `sed -i` causes | Replace a linked file with a regular copy, run `dots doctor omarchy`, confirm symlink restored and the diff appears in `git status` |
| Failure path — foreign link | TR-3/TR-4: prune never touches a link out of the repo | `dots link claude --dry-run` lists no action for `~/.claude/skills/omarchy` |
| Failure path — bad config | TR-11: one broken repo does not break the others | Append `[[link]]` with no `system` key to `CLAUDE.md/dots.toml`, run `dots status`, confirm claude errors with a line number while omarchy and nvim still report |
| Failure path — no TTY | TR-6: repo-sourced commands never auto-execute | `dots bootstrap omarchy < /dev/null` reports the post-link command as skipped, exit 0 |
| Post-cutover signal | The hook still behaves on a real update, not just a simulated one | After the next `omarchy update`, confirm no "Dotfiles relinked" notification fired on a clean tree, and that one *does* fire (with the content in `git status`) when a migration edits a tracked file |
| Manual / QA | The desktop still works after the neovim relocation | `nvim --headless "+checkhealth" +qa`; open a real file and confirm LSP and plugins load |

## Migration & rollout

- **Migration / backfill:** Three config files hand-written (step 12) and one directory relocated
  (step 13, ~1 git clone in size). `links.conf` conversion is 17 entries → 14 `[[link]]` blocks
  (the 4 plugin lines become one glob). Reversible: the old `bin/dots` and `links.conf` are not
  deleted until step 14, so steps 12-13 can be abandoned by deleting the new `dots.toml` files.
- **Flag:** `none`. The old and new tools coexist by construction between steps 12 and 14 — they
  read different config files and perform the same mutations idempotently.
- **Rollback:** In reverse order — (1) `git revert` the step-14 commit in the omarchy and CLAUDE.md
  repos, restoring `bin/dots`, `links.conf`, and `install.sh`; (2) `rm ~/.local/bin/dots`;
  (3) re-run `omarchy hook install post-update bin/relink-dotfiles.hook` to restore the old hook;
  (4) if neovim was already relocated, move `~/Documents/Projects/nvim` back to `~/.config/nvim`
  and delete the symlink. Users are left with the current bash tooling, fully functional; every
  mutation dots performed is additionally recoverable from `$XDG_STATE_HOME/dots/backups/`.
- **Compatibility window:** Steps 12-13 only. During it, `bin/dots` and `dots` both manage the
  omarchy repo; both are idempotent and agree on the desired end state, so interleaving is safe.
  `dots.toml` and `links.conf` must describe the same links until step 14 deletes the latter.

## Risks & open questions

| Item | Type | Impact | Owner | Blocks start? |
| --- | --- | --- | --- | --- |
| R-1 Relocating `~/.config/nvim` (D-12) may break plugin managers that cached absolute paths — `lazy.nvim` lockfiles, `mason` binary shims, LSP root detection resolving through a symlink | Risk | Broken editor until paths are rebuilt; recoverable by moving the directory back | @nox456 | No — isolated to step 13, which is last before cutover |
| R-2 Glob prune is the only default-delete path. A bug in repo-containment (TR-8) or target resolution (TR-4) deletes real symlinks | Risk | `~/.claude/skills/` emptied; recoverable by re-running `dots link claude` | @nox456 | No — constrained by TR-4 and verified in step 9 |
| R-3 `unlink` deletes the repo copy (D-7). If the TR-1 backup silently fails, the file is gone from disk and only in git history | Risk | Data loss for an unstaged file | @nox456 | No |
| R-4 `dots.toml` rewriting on `adopt`/`unlink` must preserve comments and ordering; `go-toml/v2` marshalling round-trips neither by default | Dependency | Config comments lost on every adopt; likely needs targeted line editing rather than marshal | @nox456 | No — affects steps 8-9, fallback is append/remove by line |
| R-5 Two writers can collide: `relink-dotfiles.hook` runs `dots doctor omarchy` during `omarchy update` while a manual `dots link omarchy` is in flight. No lock is specified | Risk | Interleaved backup/copy/symlink on the same path; worst case a link left as a plain file, repaired by re-running `dots doctor` | @nox456 | No — both operations are idempotent (TR-2) and the window is seconds; a `flock` on the repo root is the fix if it ever bites |
| Q-1 Should `dots status` report each repo's branch and ahead/behind state, or only the porcelain summary `bin/dots:165` produces today? | Question | Small scope change to `internal/gitstat` | @nox456 | No — defaults to today's behavior |
| Q-2 Where should the built binary live — `make install` symlinking `~/.local/bin/dots` to the repo build output (rebuild is instant, repo must stay present) or copying it (independent, must reinstall to update)? | Question | Affects step 14 only | @nox456 | No — defaults to symlink, matching how you install everything else |

## Out of scope

- **New test coverage** — this plan specifies no tests. Verification above uses only `go build`,
  `go vet`, `gofmt`, and observable checks against live config. What coverage the packages need is
  decided by the `write-tests` skill after implementation, and `internal/link/resolve.go` (pure,
  table-driven) is where it will matter most.
- **AUR packaging, semver tags, CI** — explicitly deferred by D-13; adding them later changes
  nothing in the tool.
- **Machine or host profiles** — different links per hostname was considered and declined. Every
  registered repo links identically on every machine.
- **Git write operations** — `dots` never commits or pushes (TR-12). Committing stays `git -C <repo>`.
- **Manifest install during `bootstrap`** — separate command by D-9. A fresh machine needs
  `dots manifest install omarchy` as an explicit second step.
- **Encrypted secrets** — no repo here tracks any, and adding it would change the backup model.
