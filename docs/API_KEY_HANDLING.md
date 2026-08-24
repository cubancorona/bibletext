# API.Bible release-key handling

## Current policy

Store releases currently include the project's API.Bible key. Persisted copies
must exist only in its dedicated external source and the final release binary.
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

Android uses a temporary Go wrapper because Fyne supplies its own linker flags.
The wrapper requires Fyne's path-trimming flag and merges the release value only
into native shared-library builds.
Both platform pipelines inspect the produced native payload and fail unless the
expected transformed value is present; local iOS exports and Android packages
are checked again after packaging. The iOS intermediate `.app`, temporary Go
caches, and Android packaging workspace are removed on exit; only the intended
final release artifacts remain.

Normal source, tests, simulator builds, and development-device builds contain
no project API.Bible key. Reader-supplied keys continue to work through Settings
and take precedence over the compiled fallback. On upgrade, the app removes an
old raw copy written by earlier releases only when its saved fingerprint proves
that the copy was app-seeded; a reader-owned value is left untouched. Clearing
the project fallback remains persistent without storing the project credential.

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

## Separate release TODO

- Replace the personal support mailbox with a role address, then update the app,
  Pages, store metadata, review notes, and downloadable binaries together.
