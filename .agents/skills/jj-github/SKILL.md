---
name: jj-github
description: Guide for using Jujutsu (jj) version control with GitHub and GitLab. Use this skill whenever the user asks about jj with GitHub/GitLab, pushing changes with jj, creating pull requests using jj, handling bookmarks in jj, addressing review comments in jj, fetching/updating from remotes with jj, merge conflicts in jj, or any workflow involving jj and a remote Git host. Trigger on any mention of "jj git push", "jj bookmark", "jj git fetch", "jj with GitHub", "PR with jj", or similar jj+remote-host combinations.
---

# Using Jujutsu (jj) with GitHub and GitLab

Source: https://docs.jj-vcs.dev/latest/github/

> **Prerequisite**: This guide assumes basic familiarity with Git or Mercurial.

---

## Basic Workflow

### Option A: Generated Bookmark Name (simpler)

Jujutsu can auto-generate a bookmark name when you push:

```sh
# Start a new commit off the default bookmark
jj new main
# Make changes, commit with a message (starts a new empty working copy)
jj commit -m 'refactor(foo): restructure foo()'
jj commit -m 'feat(bar): add support for bar'
# Push parent of working copy (working copy itself is empty)
jj git push --change @-   # short: -c
```

### Option B: Named Bookmark

```sh
jj new main
jj commit -m 'refactor(foo): restructure foo()'
jj commit -m 'feat(bar): add support for bar'
# Create a bookmark on parent of working copy
jj bookmark create bar -r @-
# Track the bookmark on the remote
jj bookmark track bar
# Push it
jj git push
```

> ⚠️ Unlike Git, Jujutsu does **not** auto-move bookmarks when you make new commits — you must move them manually.

---

## Updating the Repository (equivalent of `git pull`)

No direct `git pull` equivalent yet. It's a two-step process:

```sh
# 1. Fetch everything from remote
jj git fetch

# 2. Rebase your branches onto main
jj rebase -o main
# If you have multiple branches, specify each:
jj rebase -b your-branch -o main
```

---

## Workspace Types

### Git Colocated Workspace (`jj git init`)

After `jj git init`, Git will be in detached HEAD state. Every `jj` command auto-syncs Jujutsu's and Git's views.

```sh
nvim docs/tutorial.md
# do more work...
jj commit -m "Update tutorial"
jj bookmark create doc-update -r @-
jj bookmark track doc-update
jj git push
```

### Pure Jujutsu Repository

Simpler — generate a bookmark on the fly:

```sh
jj commit
# Push change "mw", auto-creates bookmark "push-mwmpwkwknuz"
jj git push -c mw
```

---

## Addressing Review Comments

### Adding New Commits (GitHub-style)

```sh
# Create a new commit on top of your feature bookmark
jj new your-feature
# Make changes, review
jj diff
# Commit the fix
jj commit -m 'address pr comments'
# Move the bookmark to point to the new commit
jj bookmark move your-feature --to @-
# Push
jj git push
```

### Rewriting Commits (clean history style, e.g. LLVM/jj projects)

```sh
# New commit on top of the second-to-last commit in your-feature
jj new your-feature-   # trailing hyphen = parent in revset syntax
# Make changes, review
jj diff
# Squash the fix into the parent commit
jj squash
# Force-push (jj handles this automatically)
jj git push --bookmark your-feature
```

---

## Working with Other People's Bookmarks

By default, `jj git fetch` does **not** auto-import remote bookmarks locally.

```sh
# Check out someone else's bookmark
jj new <bookmark>@<remote>

# OR: auto-track all remote bookmarks (set in config)
# remotes.<name>.auto-track-bookmarks = "*"
# Then you can simply use:
jj new <bookmark>
```

---

## Using GitHub CLI (`gh`)

In non-colocated repos, `gh` can't find the `.git` directory. Fix:

```sh
GIT_DIR=$(jj git root) gh issue list
```

**Permanent fix with direnv** — add to `.envrc` in repo root:

```sh
export GIT_DIR=$(jj git root)
```

Then run `direnv allow`. After that, `gh` commands work normally.

---

## Useful Revsets

```sh
# All local branches not yet pushed to any remote
jj log -r 'bookmarks() & ~(main | remote_bookmarks())'

# Your commits not yet pushed
jj log -r 'mine() & bookmarks() & ~remote_bookmarks()'

# Remote bookmarks you authored or committed to
jj log -r 'remote_bookmarks() & (mine() | committer(your@email.com))'

# All ancestors of working copy not yet on any remote
jj log -r 'remote_bookmarks()..@'
```

---

## Merge Conflicts

Jujutsu records conflicts in commits rather than halting. See the [tutorial](https://docs.jj-vcs.dev/latest/tutorial/#conflicts) for details.

---

## Using Several Remotes (fork workflow)

```sh
# Clone from upstream
jj git clone --remote upstream https://github.com/upstream-org/repo
cd repo
# Add your fork as origin
jj git remote add origin git@github.com:your-org/your-repo-fork
```

Configure default fetch/push targets in `jj config edit --user`:

```toml
[git]
fetch = "upstream"
push = "origin"
```

Or fetch from both:

```toml
[git]
fetch = ["upstream", "origin"]
push = "origin"
```

---

## Git Push Options (`-o / --option`)

Forward platform-specific push options to the server:

```sh
jj git push -o <push_option>
jj git push -o foo -o bar=val   # multiple options
```

### GitLab Examples

```sh
# Skip CI
jj git push -o ci.skip

# Pass CI variables
jj git push -o 'ci.variable=MAX_RETRIES=10' -o 'ci.variable=MAX_TIME=600'

# Create a merge request on push
jj git push --allow-new \
  -o merge_request.create \
  -o merge_request.target=main \
  -o 'merge_request.title=Add feature X' \
  -o 'merge_request.description=Implements X with tests' \
  -o merge_request.draft

# Auto-merge when pipeline passes
jj git push \
  -o merge_request.merge_when_pipeline_succeeds \
  -o merge_request.remove_source_branch

# Manage labels and assignees
jj git push \
  -o 'merge_request.label=label1' \
  -o 'merge_request.unlabel=old-label' \
  -o 'merge_request.assign=user1' \
  -o 'merge_request.unassign=user2'
```

> Support is server-dependent. GitLab supports push options; other platforms may not. Check your host's documentation.
