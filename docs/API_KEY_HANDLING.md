# API.Bible release-key handling

## Current policy

Published releases currently include the project's API.Bible key. Persisted
copies are limited to its dedicated external source, the encoded GitHub Actions
secret required by the cross-platform desktop builders, and final release
binaries.
The runtime necessarily decodes a temporary in-memory value when making a
request, but current releases never copy the project key into Preferences or a
platform credential store. It must never appear in tracked source, generated
files under the repository, general-purpose environment files, command output,
logs, fixtures, or reports.

The release scripts:

1. read only `BIBLE_API_KEY` from the invoking process or the dedicated macOS
   login-Keychain item `uk.co.bibletext.apibible-release` / account `release`;
2. never read `.env.local`, which may hold unrelated personal provider keys;
3. remove all known personal AI-provider variables before any subprocess and
   remove the raw Bible key from the environment before transforming it;
4. pass the key over stdin to an in-memory transformation;
5. inject the transformed value into the final executable with Go's linker;
6. fail the release if the dedicated credential is unavailable;
7. reject shell tracing and never print the key, a suffix, or a reversible
   value; and
8. use a per-release Go build cache and temporary directory that are removed
   with the release workspace.

GitHub's desktop jobs cannot access the local Keychain. They receive only the
reversible encoded payload through the encrypted repository secret
`BIBLETEXT_BUNDLED_KEY_ENC`, remove it from the environment before any build
subprocess, and reconstruct the linker assignment in memory. The raw project
key is never supplied to GitHub Actions. Rotate this secret whenever the
dedicated API.Bible credential changes. Every packaged desktop executable is
checked for the expected linker payload as well as `-trimpath` before upload.

Android uses a temporary Go wrapper because Fyne supplies its own linker flags.
The wrapper requires Fyne's path-trimming flag and merges the release value only
into native shared-library builds. It is used for both the local development APK
and release packages. The supported iOS simulator/device scripts likewise load
the dedicated external value and replace Fyne's initial executable with a keyed,
verified relink; they never retain a keyless fallback.

Every supported mobile packaging path inspects the produced native payload and
fails unless the expected transformed value is present. Local iOS exports and
Android packages are checked again after packaging. The iOS intermediate
`.app`, temporary Go caches, and Android packaging workspace are removed on
exit; only the intended final artifacts remain.

Normal source and test binaries contain no project API.Bible key. App artifacts
produced by the supported platform scripts do. Reader-supplied keys continue to
work through Settings and take precedence over the compiled fallback. On
upgrade, the app removes an old raw copy written by earlier releases only when
its saved fingerprint proves that the copy was app-seeded; a reader-owned value
is left untouched. Clearing the project fallback remains persistent without
storing the project credential.

## Security boundary

An application key embedded in a public client is recoverable. The link-time
transformation prevents accidental plaintext disclosure but is not encryption
and does not make the release binary a secret store. Repository cleanup cannot
retract binaries already distributed through an app store or release page.

API.Bible's current guidance says application keys should remain private and
should not be embedded in client-side code. Provider coordination is therefore
still required even while keyed releases continue:

- <https://care.api.bible/article/414-managing-apps>
- <https://api.bible/terms-and-conditions>

Questions for API.Bible should cover the existing deployed key, the permitted
mobile-client architecture for the licensed translation, rotation without
breaking installed versions, rate limits, and any approved longer-term design.

## Release verification

Before publishing a release:

- prove the exact local credential does not occur in tracked files or Git
  objects;
- prove the release script fails when the dedicated credential is absent;
- build with `-trimpath` and inspect binary strings for local paths, emails, and
  plaintext credentials;
- verify the keyed build can retrieve the licensed translation; and
- retain no extracted or intermediate credential-bearing artifact after the
  signed release is produced.

For a non-publishing platform validation, direct the final artifacts to a
temporary directory with `BIBLETEXT_IOS_OUT_DIR` or
`BIBLETEXT_ANDROID_DIST_DIR`, inspect them, then remove that directory.

## Public support contact

The public support mailbox has one tracked source:
`config/support-email.txt`. Application code reads it through `SupportEmail`,
and the site publisher renders separate display-text and mailto-recipient
markers into the privacy and support page templates. The shared conservative
grammar is tracked in `config/support-email-pattern.txt`; it excludes URI
delimiters and malformed dot or domain forms while allowing ordinary plus
tags.
Store and release entry points run `scripts/check-support-contact.py`, which
rejects an invalid configuration, a literal copy elsewhere in the project, a
missing template marker, or an unguarded publishing path.

A future move to a role mailbox therefore changes the configuration file only.
After that change, rebuild the applications, republish the site, and update any
external store contact field before release; do not paste either address into
source, documentation, store checklists, or release notes.
