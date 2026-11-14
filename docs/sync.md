---
title: Sync
description: Keep one machine's Hyla configuration on all of them, through WebDAV, S3 or GitHub, sealed with a passphrase.
---

# Sync

Hyla's configuration — theme, fonts, keys, agents, Feishu, Tailcat — is one file. **Sync** keeps it the same on every machine you use, through a server they all can reach: a WebDAV folder, an S3-compatible bucket, or a GitHub repository.

The snapshot is sealed on the machine that writes it. The server stores ciphertext: it cannot read your settings, and neither can anyone who finds the file without your passphrase.

## What it syncs

The configuration file alone, the one `hyla config path` names. It holds what differs from the defaults, so a machine that syncs takes the other machines' settings as they are.

It does **not** sync what belongs to one machine: sessions and their terminals, projects, the sidebar's SSH and K8s machines, window sizes, or the session you had selected. Those are that machine's own.

## Where it goes

| Server | What you fill in |
| --- | --- |
| **WebDAV** — Jianguoyun, Nextcloud, ownCloud | the address (as `https://dav.jianguoyun.com/dav/`), your user and an app password |
| **S3-compatible** — MinIO, R2, B2, OSS, COS | endpoint, bucket, prefix, region, access key and secret |
| **GitHub** | `owner/name`, the path (as `hyla/config.enc`), the branch, and a token — a **private** repository, since a snapshot holds your secrets even though they are sealed |

Hyla makes a `hyla/` folder in a WebDAV address, as it uses `hyla/config.enc` in a bucket or a repository, so the same server can hold other things too.

## The passphrase

One passphrase, typed on each machine, seals and opens the snapshot. It is derived into a key with Argon2id, and the contents are encrypted with AES-256-GCM: a wrong passphrase, or a file changed on the server, opens nothing at all.

**Remember it.** There is no recovery: without the passphrase the file cannot be read on any machine, by Hyla or by anyone else.

The passphrase and the server's credentials stay on each machine, in `sync.json` beside the configuration (readable by your user alone). They are never part of what syncs, so the snapshot does not carry the keys to itself.

## When it syncs

- **At launch**: Hyla pulls, so a machine you have not opened for a while starts with the settings the others have.
- **After a change**: a setting you change is pushed half a minute later, once the run of changes settles. Turn **Sync automatically** off to sync only when you say so.
- **When you say**: **Settings › Sync › Sync now**, or `hyla sync` from a terminal.

A machine with no configuration file takes the server's rather than sending an empty one.

## When both sides changed

The machines you use do not change the same setting at the same time often, but they can. Then **this machine's configuration stays**: the server's is written beside it as `config.remote-<machine>-<time>`, and the push replaces the server's copy. Nothing you wrote is lost without a copy, and Hyla tells you where it put the other side's.

## From the CLI

```sh
hyla sync                    # sync now, as the app does at launch
hyla sync status             # where it syncs, and how far it got
hyla sync push               # send this machine's, whatever the server has
hyla sync pull               # take the server's, whatever this machine has
hyla sync set backend webdav # and webdav.url, webdav.user, webdav.password…
hyla sync set passphrase "…" # the same on every machine
hyla sync set auto on
```

`hyla sync set` takes `backend` (`webdav`, `s3`, `github` or `off`), `passphrase`, `auto`, `webdav.url|user|password`, `s3.endpoint|bucket|prefix|region|accessKey|secretKey` and `github.repo|path|branch|token`.
