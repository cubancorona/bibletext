# Listing screenshots

Captured on GitHub runners, not by hand: `.github/workflows/linux-screenshots.yml`
and `.github/workflows/windows-screenshots.yml` build the release executable,
open it with links minted by the app's own link code, drive four scenes and
upload the frames. Dispatch either workflow, download its artifact, and
replace the files here.

- `linux/` — 1280×860, the window under Xvfb with software OpenGL. These are
  the URLs the AppStream MetaInfo points at, pinned to the commit named by
  `screenshots.ref` in `linux/listing.toml`; re-render with
  `go run ./cmd/linuxmeta render` after replacing them, and set the ref to the
  commit that carries the new files.
- `windows/` — 1600×960, the window's client area on the runner (above the
  Microsoft Store's 1366×768 floor). Uploaded by hand in Partner Center; see
  docs/WINDOWS_STORE_LISTING.md.

Both sets are the light appearance, the Berean Standard Bible, and the same
four scenes: a note received inside a shared link, a passage, search results,
and the settings sheet.
