# Privacy release checklist

Use this checklist whenever screenshots, store declarations, public contact
details, or data flows change. It records release requirements without storing
account identifiers or private fixture data.

## Public contact

- Change the public mailbox only in `config/product.json`.
- Run `scripts/check-support-contact.py`; it must pass before building an App
  Store, Play, GitHub, or Pages release.
- Regenerate the site pages with `scripts/publish-site.sh --dry-run` and confirm
  the privacy and support pages contain no unresolved contact marker.
- Rebuild every downloadable application that presents the mailbox. A source
  change cannot update an application or web page that is already deployed.
- Update external store contact fields in the same release window. Store
  checklists should refer to the configured mailbox rather than copy it.

## Neutral screenshots

- Replace legacy note-list screenshots and their phone, tablet, prepared, and
  design derivatives before the next store-metadata refresh.
- Use plainly synthetic labels and messages. Avoid family roles, realistic
  personal messages, email addresses, phone numbers, device identifiers, photo
  library filenames, and local filesystem paths.
- Inspect the final exported pixels, not only the source fixture. Run OCR over
  every size and derivative and review the recognized text manually.
- Keep the replacement set consistent across App Store Connect, Play Console,
  public documentation, and any release announcement that embeds an image.

## Privacy declarations

- Compare `cmd/mobile/PrivacyInfo.xcprivacy`, `docs/privacy.html`, Apple App
  Privacy answers, and Google Play Data safety answers against the final binary.
- Re-evaluate the declarations whenever a provider, analytics component,
  account feature, note transport, key-storage path, or report flow changes.
- Distinguish data sent directly to a selected third-party provider from data
  received by project-operated infrastructure. Record the applicable store
  definitions and exceptions; do not infer them from an earlier submission.
- Confirm that the AI report action opens the device mail client and that no
  report is transmitted until the sender chooses to send it.
- Run `appstore/preflight.py` before an Apple submission and review current
  Play Console declarations before uploading an Android release.
