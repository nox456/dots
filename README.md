# dots

**Keep your dotfiles in git. Keep dots out of them.**

`dots` is a small CLI that links configuration files from your git repos into place with
symlinks — and keeps them linked. What makes it different: **your repos contain no `dots` file at
all.** Every repo it manages is plain files; everything `dots` needs to know lives in one
directory on your machine, `~/.config/dots/`.

> 🚧 **Work in progress.** `dots` is being built in phases. Commands and flags described here are
> the target behavior; check the changelog before relying on any of them.

---

## Why dots

**No config inside your repos.** Most dotfiles managers want a manifest in the repo — a
`.dotfiles.yml`, an `install.sh`, a `links.conf`. That turns your config repo into something that
has to know how it is deployed, and every repo ends up with its own linker. With `dots`, a repo is
a passive payload: your neovim config stays a neovim config, your Claude skills stay a folder of
skills. How they get linked is written down in *one* place, per machine, outside all of them.

**One command for the whole machine.** Repos are registered by name, so `dots status` checks every
one of them from any directory — including repos you never `cd` into, like `~/.config/nvim`.

**Survives system updates.** Package updates and desktop migrations love to `sed -i` your config
files, which silently replaces your symlink with a regular file. `dots doctor` detects that, takes
the new content into your repo (so you see it in `git diff`), and restores the link. It prints
nothing when all is well, so it fits in a post-update hook.

**One-to-many links.** `skills/*` → `~/.claude/skills/` links every entry by name and cleans up
links whose source you deleted — no custom script required.

**Hard to lose data with.** Everything `dots` overwrites or deletes is backed up first; symlinks it
doesn't own are never touched; and it never runs `git commit`, `push` or `checkout`.

## How it works

```
~/.config/dots/                       ← the only place dots reads configuration
├── omarchy.toml   ─┐
├── claude.toml    ─┼─ one file per repo; the filename is the repo's name
└── nvim.toml      ─┘

~/Documents/Projects/omarchy/         ← plain git repos, no dots files inside
~/Documents/Projects/CLAUDE.md/
~/Documents/Projects/nvim/

~/.config/hypr/bindings.lua  →  ~/Documents/Projects/omarchy/config/hypr/bindings.lua
~/.claude/skills/<each>      →  ~/Documents/Projects/CLAUDE.md/skills/<each>
~/.config/nvim               →  ~/Documents/Projects/nvim
```

The real files live in the repo; the system path is a symlink to them. Edit either side and
you're editing the same file — commit whenever you like, with plain `git`.

The directory listing *is* the registry: adding a repo means adding a file, removing one means
deleting it. A broken file only disables the repo it describes; the others keep working.

## Install

Requires Go 1.27+ and git.

```sh
git clone https://github.com/nox456/dots.git
cd dots
make install      # builds and symlinks the binary to ~/.local/bin/dots
```

Make sure `~/.local/bin` is on your `PATH`.

## Quick start

Register a repo by writing its config file:

```toml
# ~/.config/dots/nvim.toml
path = "~/Documents/Projects/nvim"
url  = "https://github.com/you/neovim-config.git"

[[link]]
repo   = "."
system = "~/.config/nvim"
```

Then:

```sh
dots status nvim    # what's linked, what isn't, what's uncommitted — changes nothing
dots link nvim      # make it so
dots status         # every registered repo at once
```

Already have a config file somewhere and want it tracked?

```sh
dots adopt omarchy ~/.config/foot/foot.ini
```

This moves the file into the repo, records the link in `~/.config/dots/omarchy.toml`, and symlinks
it back. `dots unlink` reverses all of it.

## Configuration

One TOML file per repo in `$XDG_CONFIG_HOME/dots/` (usually `~/.config/dots/`). Only `*.toml`
files directly in that directory are read; subdirectories and other files are ignored.

```toml
# ~/.config/dots/omarchy.toml
path = "~/Documents/Projects/omarchy"          # where the repo is (or will be) cloned
url  = "https://github.com/you/omarchy.git"    # used by `dots bootstrap` when path is missing

# A single file
[[link]]
repo   = "config/hypr/bindings.lua"            # relative to the repo root
system = "~/.config/hypr/bindings.lua"

# A glob: every entry in plugins/ is linked into ~/.config/omarchy/plugins/ by name.
# Links whose source was deleted from the repo are pruned, and anything new you create
# in that directory is adopted into the repo on the next `dots link`.
[[link]]
repo   = "config/omarchy/plugins/*"
system = "~/.config/omarchy/plugins/"

[settings]
# Directories whose unexpected children `dots status` should point out. A child that is
# its own git repo (a cloned plugin, say) is only reported — never adopted.
scan = ["~/.config/omarchy/plugins/*/"]

[adopt]
# Where `dots adopt` puts a file when you don't say. First match wins.
map = [
  { system = "~/.config/*",   repo = "config/*" },
  { system = "~/Pictures/bg", repo = "wallpapers" },
]

[hooks]
# Offered after `dots bootstrap` links this repo. Always shown in full and
# only run after you confirm.
post_link = "hyprctl reload && omarchy restart shell"
```

| Key | Required | Meaning |
| --- | --- | --- |
| `path` | yes | Where the repo lives on this machine. `~` is expanded. |
| `url` | yes | Clone URL used by `bootstrap` when `path` is missing. |
| `[[link]].repo` | yes | Path inside the repo. A trailing `/*` makes it a glob. |
| `[[link]].system` | yes | Where the link goes. For a glob, the directory the matches are linked into. |
| `[settings].scan` | no | Globs of directories to watch for untracked additions. |
| `[adopt].map` | no | Default repo locations for `dots adopt`. |
| `[hooks].post_link` | no | A command offered after `bootstrap` links the repo. |

A `[[link]]` can never point outside its repo: a glob match that escapes the repo root is an
error, not a link.

## Commands

| Command | What it does |
| --- | --- |
| `dots status [repo]` | Report every link's state, untracked items under `scan` directories, and uncommitted changes. Read-only. |
| `dots link [repo]` | Create missing links, adopt files that exist only on the system, prune links whose source is gone. |
| `dots doctor [repo]` | Repair links that were replaced by a regular file; the new content goes into the repo. |
| `dots adopt <repo> <path> [repo-path]` | Move a system file into a repo, record it, and link it. |
| `dots unlink <repo> <path>` | Put the real file back, remove the record, delete the repo copy. |
| `dots bootstrap [repo]` | For each repo: clone if missing, link, offer `post_link`. |
| `dots repo list\|add\|remove` | Manage the files in `~/.config/dots/` without editing them by hand. |
| `dots version` | Print the version. |

Leave out `[repo]` to act on every registered repo. Global flags: `-v/--verbose` (also show
healthy links), `--quiet` (only show problems), `--dry-run` (show what would change, change
nothing), `--yes` (answer "yes" to `post_link` prompts — only works at a terminal).

### Link states

| State | Meaning | Who fixes it |
| --- | --- | --- |
| Linked | Symlink pointing at the repo copy. Healthy. | — |
| Drifted | A regular file sits where the link should be, and the repo has a copy. Typical after an update ran `sed -i`. | `dots doctor` |
| Unlinked | The repo has the file; nothing at the system path. | `dots link` / `dots doctor` |
| Unadopted | A file at the system path; nothing in the repo yet. | `dots link` |
| Orphan | A link into the repo whose source was deleted (globs only). Counts as unhealthy. | `dots link` prunes it |
| Foreign | A symlink pointing somewhere else entirely. Reported, never touched. | you |
| Missing | Neither side exists. | you |

## Setting up a new machine

`dots` deliberately doesn't fetch its own configuration — getting it there is two commands:

```sh
git clone https://github.com/you/dots-config.git ~/Documents/Projects/dots-config
ln -s ~/Documents/Projects/dots-config ~/.config/dots     # or copy the .toml files in

dots bootstrap
```

`bootstrap` clones every repo that's missing, links it, and offers each repo's `post_link` hook.
If `~/.config/dots/` doesn't exist it stops and tells you so.

### Letting dots track its own config

Register the config repo too, and `~/.config/dots` becomes a managed link — so every
`dots adopt` and `dots unlink` edit lands in a git working tree you can commit and push:

```toml
# ~/.config/dots/dots-config.toml
path = "~/Documents/Projects/dots-config"
url  = "https://github.com/you/dots-config.git"

[[link]]
repo   = "."
system = "~/.config/dots"
```

Pick one mode: if you register the config repo, don't also copy files into `~/.config/dots` —
`dots doctor` would treat the copied directory as drift and turn it back into a link.

## Safety

- **Backups first.** Anything `dots` is about to overwrite or delete is copied to
  `$XDG_STATE_HOME/dots/backups/<timestamp>/<repo>/` (usually `~/.local/state/…`). If the backup
  fails, the change doesn't happen.
- **Foreign links are sacred.** A symlink pointing outside the owning repo is reported and left
  alone — even inside a directory `dots` manages with a glob.
- **Pruning is narrow.** A path is only removed if it is a symlink, points inside the owning repo,
  and its target no longer exists. Never a regular file, never a directory.
- **Idempotent.** Running `link`, `doctor` or `bootstrap` twice changes nothing the second time.
- **No git writes.** `dots` reads `git status`; the only git command that writes is `git clone`
  during `bootstrap`. Committing is up to you.
- **Hooks need a human.** `post_link` is printed in full and runs only after you answer `y`, or
  when you pass `--yes` from a terminal. Without a terminal (CI, SSH scripts, cron) it's skipped —
  `--yes` included.
- **One bad file stays contained.** A config file that doesn't parse is reported with its file name
  and line, and only that repo is skipped.

## Scripting and hooks

`dots status` exits non-zero **only** when a link is unhealthy. Uncommitted changes are normal
and exit 0, so it's safe in scripts and prompts. Colours are disabled when output isn't a terminal.

`dots doctor --quiet` prints nothing and exits 0 when everything is fine, which makes it a good
post-update hook — notify only when there's output:

```sh
out=$(dots doctor --quiet 2>&1)
[ -n "$out" ] && notify-send "Dotfiles relinked" "$out"
```

## What dots doesn't do

- Commit, push, or pull — use `git` directly.
- Templates, per-host profiles, or encrypted secrets — every machine links the same way.
- Install packages or plugins.
- Manage its own configuration delivery — clone and link `~/.config/dots` yourself.

## Building from source

```sh
make build    # ./dots
make vet
make fmt
```
