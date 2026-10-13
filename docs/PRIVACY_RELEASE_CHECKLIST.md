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
- Distinguish data sent directly to a reader-selected provider under the
  reader's own key from data received by project-operated infrastructure.
  Preserve that distinction when assessing an actual data-flow change.
- Confirm that the AI report action opens the device mail client and that no
  report is transmitted until the sender chooses to send it.
- Run `appstore/preflight.py` before an Apple submission. Update store privacy
  answers when the app's data handling changes, then read the saved answers back.
- Keep XML comments out of both bundled Apple manifests; record their rationale
  in this checklist.

### Direct bring-your-own-key AI requests

The release classification is **Data Not Collected**: the developer operates no
application server, app accounts, or analytics and receives none of the reader's
AI requests. Both Apple manifests have an empty collected-data array and false
tracking declarations. The mobile manifest's required-reason API entries remain
independent of that classification.

Find sends the entered query; passage study sends the selected passage and study
action, including an entered question for Ask a question. These optional requests
go directly to the selected provider under the reader's own key. The key is not
sent to the developer. Core reading works without AI.

This classification treats direct BYOK use as the reader's interaction with the
reader's own provider account. It is not an Apple ruling about this exact
relationship. The manifest does not assert that providers receive nothing,
retain nothing, or cannot associate requests with a provider account. Provider
retention and use depend on provider policies and account settings; local
key removal does not delete provider records. Keep those facts in the public
privacy policy and the in-app AI disclosure.

A prompt or presentation change does not itself change the data flow. Reassess
this classification if requests begin passing through project-operated
infrastructure, developer-managed AI credentials replace reader-supplied keys,
or another feature introduces developer collection. A manifest change alone
does not update a store's public privacy label.

References:

- [Apple collection and linkage definitions](https://developer.apple.com/app-store/app-privacy-details/)
- [Google Play Data safety definitions](https://support.google.com/googleplay/android-developer/answer/10787469?hl=en)

### Provider account terms

Readers create their provider accounts and keys directly with the provider;
provider terms govern eligibility, billing, retention, and account controls.
BibleText does not determine a key's service tier or override those terms.
Review the provider relationship when adding a provider or changing how requests
are authenticated or routed.
