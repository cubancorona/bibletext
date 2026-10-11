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

### Optional AI requests in 1.2.20

The manifest conservatively covers provider-retained data from direct,
authenticated requests, even though the developer receives none of it. It
declares Other User Content and Search History as linked, for App Functionality
and Other Purposes, and User ID as linked for App Functionality. None is used
for tracking. User ID covers retained account/key-associated usage records,
not an assertion that a raw API secret is retained. Other Purposes covers
unpaid Gemini's permitted product/model improvement use. The app does not
enforce a provider service tier or promise that requests are de-identified.

The corresponding Play choices are optional Other user-generated content,
In-app search history and User IDs, conservatively disclosed as both collected
and shared. Requests are encrypted in transit but not ephemeral. Core reading
works without AI. Provider records are governed by provider controls; local
deletion does not delete them.

This is a conservative disclosure judgment, not an Apple ruling about direct
bring-your-own-key services. Apple's partner definition describes integrated
external vendors but does not expressly settle this exact relationship. Review
the live forms and retained records before submission, save the corresponding
choices, and read them back. The manifest alone changes no store label.

References:

- [Apple collection and linkage definitions](https://developer.apple.com/app-store/app-privacy-details/)
- [Apple manifest purposes](https://developer.apple.com/documentation/bundleresources/app-privacy-configuration/nsprivacycollecteddatatypes/nsprivacycollecteddatatypepurposes)
- [Google Play Data safety definitions](https://support.google.com/googleplay/android-developer/answer/10787469?hl=en)
- [Gemini handling and authentication details](https://ai.google.dev/gemini-api/terms)
- [Grok per-key request records](https://docs.x.ai/developers/faq/security)
- [Anthropic API-key-associated usage](https://platform.claude.com/docs/en/api/beta/organization/usage_report/retrieve_messages)

### Provider eligibility before release

Privacy disclosure does not settle whether a provider permits the application's
use. Gemini's terms effective 23 March 2026 require adult API users, restrict
clients directed toward or likely accessed by minors, describe professional or
business rather than consumer use, and require Paid Services for clients made
available in the EEA, Switzerland or UK. Resolve the applicability of these
terms to this general-purpose reader before releasing Gemini support. An age
rating, a reader-supplied key or a privacy label alone does not establish
eligibility. This review remains open; no provider configuration change is
implied by the manifest update.
