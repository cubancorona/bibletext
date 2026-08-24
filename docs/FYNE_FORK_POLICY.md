# Fyne fork maintenance policy

BibleText carries Fyne changes that cannot live in the application module:

- iOS draw-loop latency and idle-frame handling;
- discrete caret blinking;
- current emoji-font coverage; and
- mobile platform bridge changes not yet present upstream.

The public fork is `github.com/cubancorona/fyne`. Its maintenance branch is a
small rebased commit stack over an upstream release. Each logical change stays
in its own commit and is classified as either upstreamable or BibleText-only.

## Versioning

Fork releases use an immutable suffix such as `v2.8.0-bt.1`. Published tags are
never moved because Go module proxies checksum them. A changed fork build gets
the next suffix.

The application consumes a fork release through a remote `replace` directive:

```go
require fyne.io/fyne/v2 <upstream-version>
replace fyne.io/fyne/v2 => github.com/cubancorona/fyne/v2 <fork-tag>
```

The fork retains `module fyne.io/fyne/v2`; the replacement target supplies the
implementation. Consumption is by immutable tag, not a moving branch.

The current release scripts still apply the reviewed patch stack to a temporary
working tree. Any switch to direct fork consumption must be one coherent change
that removes the superseded patch path and proves platform parity.

## Rebase procedure

For an upstream Fyne release:

1. Rebase the fork stack onto the exact upstream tag.
2. Resolve each fork commit separately; do not squash the stack.
3. Run Fyne's own test suite and the fork-specific invariant tests.
4. Build BibleText against the candidate fork tag.
5. Run BibleText's full host suite, tagged builds, and mobile wrappers.
6. Validate iOS and Android behavior on their native surfaces.
7. Publish a new immutable `-bt.N` tag only after those checks pass.

At minimum, fork-specific coverage must pin caret repaint cadence, emoji glyph
coverage, draw-loop wake/sleep behavior, Android warm-link delivery, and any
native text-selection changes.

## Scope discipline

A change belongs in the fork only when it structurally cannot be implemented in
BibleText. Application layout, product behavior, data handling, and release
policy remain in this repository.

Every upstreamable change should be proposed to Fyne after it stabilizes. Once
an equivalent upstream fix ships and passes the same invariants, remove the fork
commit rather than carrying a duplicate implementation.

Binary assets required by the engine belong in the fork commit that references
them. They must not depend on untracked side-channel copies.

## Consumption acceptance criteria

A direct-consumption change is complete only when:

- ordinary `go build ./...`, `go test ./...`, and `go test -race ./...` exercise
  the forked engine;
- desktop, simulator, device, Android debug, and Store-release wrappers use the
  same reviewed engine source;
- all temporary `go.mod` replacement and restoration behavior is removed or
  updated coherently;
- the existing patch files and copy steps are deleted only after parity is
  demonstrated; and
- the fallback path is documented and tested before the change is published.

See [`FYNE_28_PORT.md`](FYNE_28_PORT.md) for the currently known 2.8 compatibility
constraints and [`../patches/README.md`](../patches/README.md) for the active
temporary patch pipeline.
