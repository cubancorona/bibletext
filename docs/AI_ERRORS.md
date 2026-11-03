# AI request errors

Read this before changing AI HTTP error handling, retries, billing links or
timeouts. Study actions, Ask, Compare, both Find layouts and Settings' Test
key use the same classification. No prompt, provider key storage or
third-party consent rule changes as part of this handling.

## Classification and recovery

`ai_errors.go` classifies a response before generic model recovery or retry.
`aiResponseError` retains its underlying `apiHTTPError`, so existing status
checks still work. The request captures its `providerInfo`; a Settings change
while it is running must not rename the eventual error or send its billing
link to another provider.

| Failure | Required evidence | Reader guidance and actions |
|---|---|---|
| Exhausted account credit or spending limit | A recognized structured credit/spend code or a narrowly recognized provider clause. Gemini's documented 402 `RESOURCE_EXHAUSTED` is a depleted-prepay case. A clause alone does not count when a Google `QuotaFailure` or `RetryInfo` detail names a quota or a retry delay. | Name the provider; explain that the account behind the key needs attention with its vendor; offer the provider's billing link and Try again. |
| Other billing issue | A documented billing-error type without evidence of an exhausted balance or cap. | Describe a billing issue, without asserting that credit has run out; offer the same billing link and Try again. |
| Request timeout | The request's deadline, or the transport's wait for the response headers, met after the request was written whole: no response headers in time, or the request's deadline before the answer was whole. Or HTTP 504. | Name the provider, say the request was stopped, offer Try again and, when available, the existing faster-model action. Sent once. |
| HTTP 408 | A completed HTTP 408 response, without evidence that the request's own deadline expired. | Keep the existing ordinary 4xx policy: one attempt, then "The AI service returned an error. Please try again shortly." and existing generic recovery actions. No timeout-only faster-model action. |
| Connection never made, or lost | A refused connection, a DNS failure, a dial, DNS or TLS handshake timeout, or the request's deadline met before the request was written whole; or a connection that fails once made, reset, or timed out by TCP when its peer stops answering (ETIMEDOUT), before the response or inside it. | Keep 1.2.24's handling: up to three attempts while the request's budget lasts, then "Couldn't reach the AI service. Check your connection and try again." |
| Ordinary rate limit or quota | A transient rate-limit code, a Google `QuotaFailure` or `RetryInfo` detail without a structured account code, or ambiguous 429 / `RESOURCE_EXHAUSTED` without account evidence. | Keep the existing retry guidance. An HTTP status alone does not prove exhausted funds. |
| Authentication or permission | Existing key/permission response without account evidence. | Keep the existing key or permission guidance. A generic 403 is not billing evidence. |
| Output allowance exhausted | `errBudgetExhausted`: the model used its output allowance without a visible answer. | Keep the existing output/reasoning remedy. This is not a financial spending limit. |
| Cancellation | The reader cancels, closes, or supersedes the request. | Stop silently. A late failure cannot replace a newer request or reopen a closed panel. |

Provider error shapes vary. Numeric Google `error.code` is not an OpenAI
string code; Google's typed `ErrorInfo` details and Claude's nested
`error.details.error_code` are decoded separately. A specific transient
rate-limit code takes precedence over an ambiguous broad quota type. Gemini
opens every 429 `RESOURCE_EXHAUSTED` with the sentence OpenAI uses for an
exhausted account, so its `QuotaFailure` and `RetryInfo` details, which name
a per-minute or per-day quota and the delay before a retry, outrank that
clause unless a structured account code or `ErrorInfo` reason is present.
Unknown or malformed bodies continue through ordinary safe error handling.
No account failure is retried automatically. Only an explicit Try again or
faster-model action sends another request.

A timeout is the provider's only when the request was written whole and
one of the request's own timers then ran out: its deadline, or the
transport's wait for the response headers. `doAIRequest` watches each
attempt's write through the request's `httptrace` hooks (`aiWriteTrace`),
which the transport calls on HTTP/1.1 and HTTP/2 before it reports how the
connection failed: `GetConn`, each time the transport goes to get a
connection, starts the attempt unwritten, and `WroteRequest` without an
error marks it written. The transport sends a request again by itself, on
another connection, when the first did not process it (nothing written on a
pooled HTTP/1.1 connection, or an HTTP/2 stream a GOAWAY left
unprocessed), so a write reported on the first must not stand for a second
connection that is never made. `doAIRequest` marks such a timer met after
the write as the provider's (`aiProviderTimeout`); `isAITimeout` accepts
that mark and HTTP 504, and nothing else. HTTP 408 keeps its existing ordinary
4xx handling: one attempt and the generic error line, without timeout-only
faster-model recovery. Its terminal error retains no body-derived model or
completion-cap category, so it cannot enter those recovery paths. This
classification does not change automatic retries.
The error itself cannot
tell a deadline's two phases apart: one that expires during the TLS
handshake and one that expires after the write both reach the caller as
`context deadline exceeded`, and a dial timeout satisfies
`errors.Is(err, context.DeadlineExceeded)` too.

A timeout the connection itself reports, a `net.OpError`, is not one of the
request's timers (`aiTransportTimeout`). TCP that gives up on a peer which
stopped answering, its keep-alive probes or retransmissions unanswered,
reports ETIMEDOUT, a `net.Error` whose `Timeout` is true; that is the
connection lost, as a reset is, before the response or inside it. Over
HTTP/2 the transport's wait for the response headers ends in a `net.Error`
timeout that matches no deadline and is no connection's, so it stays the
provider's. Inside the answer only the request's deadline counts: an
answer cut off as it passes is the provider's, whatever the read reports
as the connection closes, an unexpected EOF included, and one cut off with
the deadline still ahead was lost with its connection, whatever error the
read gives. A connection never made or lost is usually the reader's own
network, so telling the reader that the provider took too long, or offering
a faster model, would misdirect them; it keeps the line and the retries
1.2.24 has.

What the code knows is the write, not delivery. On HTTP/1.1 the transport
reports the request written once its last bytes are in the transport's
buffer, before they are flushed, and nothing after the write tells a
network that went silent from a provider that does not answer: such a
request also reads as the provider's timeout when its deadline passes,
unless TCP gives up on the connection first.

## Wording

Every line here is shown word for word, {Provider} being the provider's
short name (ChatGPT, Claude, Gemini, Grok) and {Vendor} its vendor, and is
held byte for byte, straight apostrophes and curly quotation marks
included, by `TestTheAIErrorWordingWordForWord` (`ai_error_wording_test.go`).

Decided 15 and 16 October 2026: the account-limit message, the quoted
reason under it, the "{Provider} billing" link, Try again, the "Switch to
a faster model" action, and the timeout line. The account-limit message is:

> {Provider} didn't accept the request: the account behind your key has
> reached its spending limit or run out of credit. You can change that with
> {Vendor}; nothing in BibleText needs changing.

Under it, in small muted text, the provider's reason
([Provider reason text](#provider-reason-text)):

> {Vendor} said: “{recognized clause}”

Decided 2 November 2026: only the recognized opening clause is quoted,
never the rest of the reason, and Try again stands above the billing link.

Decided 2 November 2026: a billing error that is not an exhausted balance
or cap says:

> {Provider} didn't accept the request because of a billing issue with the
> account behind your key. You can check that with {Vendor}; nothing in
> BibleText needs changing.

A timeout says, where the screen offers the faster model:

> {Provider} took too long to answer, so the request was stopped. Try again,
> or switch to a faster model.

and where it does not, the provider being on its faster model already or no
longer the active one, or no provider having been captured:

> {Provider} took too long to answer, so the request was stopped. Try again.

Decided 2 November 2026: the timeout line names the faster model exactly
where the screen shows the faster-model action: on the study panel, for
every action including Ask and Compare, on both Find layouts and under
Settings' Test key. Each screen reads the offer once for both
(`aiErrorRecovery`; Test key's verdict on every paint).

Decided 2 November 2026: a request the reader cancelled, should its failure
reach a screen, reads "The request was cancelled."; with no provider
captured, {Provider} is "The AI service" and {Vendor} "the provider", and
no billing link is offered; and the vendors are named OpenAI, Anthropic,
Google and SpaceXAI.

The faster-model action is offered only for the captured provider while it
is still active. Its callback checks again before changing a model or
retrying. It must not change another provider's model after Settings has
changed. Decided 2 November 2026: Settings' Test key offers it on a timeout
too, beside its existing explicit test/retry control.

## Provider reason text

Raw provider error bodies can echo the key, selected passage, question,
account identifiers, URLs or arbitrary markup. They are never displayed as
a quoted reason. `aiAccountLimitReason` accepts only a fixed, known opening
clause and returns the actual matching substring, with the rest discarded.
That one-line clause appears in small muted text as:

> {Vendor} said: “{recognized clause}”

A structured account code without a safe clause still gets useful account
guidance and actions, with no quotation. Unknown messages are omitted rather
than relying on redaction to find every possible secret. Reason text is
plain text, not Markdown or HTML, and it is not an outbound-link source.
Billing destinations are fixed HTTPS URLs mapped to known providers.

## Presentation

`ai_error_ui.go` supplies the shared detail, billing link and actions. Study
errors normally keep their recovery actions below the message scroll. When
that fixed content cannot leave one readable message line inside the safe
sheet bounds, the title and Close remain fixed while passage, question,
message, recovery rows, disclaimer and disabled Report/Copy scroll together.
Find keeps its complete message and controls in a scrollable viewport. Settings keeps its existing Test key workflow and
adds the relevant recovery detail. Existing icon, button and toolbar visual
dimensions are reused.

Decided 2 November 2026: the study panel's error state keeps the panel's
own spacing on every screen. The header's rows stay the theme's padding,
7 points, apart, and each action has a row of its own, Try again first and
then the billing link or the faster model; no gap shrinks and no row is
shared to make room on a short phone. The column, the message and the
quoted reason under it, is kept whole by the rule every state of the panel
follows (`fitBody` and `wholeThrough` in `ai_panel.go`): the panel covers
the desktop header where it must, held to the most a sheet may be
(`sheetMostHeight`). Where even that is too short the column scrolls under
the pinned actions. Decided 3 November 2026: if that fixed layout cannot
leave a readable message line, use the combined overflow layout described
above. Normal-height composition returns when the window grows or an answer
replaces the error. Header gaps, separate recovery rows and all text/control
sizes stay unchanged. `aiRecoveryCutOn` holds the normal-layout cut cases;
`TestAIErrorExceptionalHeightScrollsContextAndRecovery` holds the exceptional
safe bounds and scroll reachability. An expanded quote retains its own scroll
viewport and the reader's choice.

Copy and Report remain disabled for an error, and the previous answer is
cleared. An error is not cached as an answer. A theme rebuild must not
silently repeat a paid request to recreate an error screen.

## Verification

Use synthetic HTTP responses and deterministic request contexts, never
real account limits or paid requests. Cover:

- Every provider's credit/spend cases, ordinary 429 and generic permission
  errors; structured-only, clause-only, malformed and unknown bodies.
- OpenAI's specific rate-limit code versus broad quota type; Claude's
  organization/workspace cap, nested spend code and generic billing type;
  Gemini's depleted prepay versus a free-tier quota whose message opens with
  the account clause; Grok's explicit account signal versus generic 403/429.
- On real transports in the process: no response headers in time, the
  request's deadline after the write and the deadline inside the body,
  each over HTTP/1.1 and HTTP/2, and HTTP 504, each sent once; HTTP 408 over
  HTTP/1.1 and HTTP/2 sent once with the generic line and no timeout-only
  faster-model action; a
  connection never made (refused, a host not found, a DNS, dial or TLS
  handshake timeout, the request's deadline before or while the request is
  written, a pooled connection that sends nothing and a redial that times
  out) or lost once the server has the whole request (timed out by TCP
  before any response or inside the body, or reset before any response,
  over HTTP/1.1 and HTTP/2) tried as 1.2.24 tries it, with its line. With
  synthetic answers, an answer cut off as the deadline passes, by an
  unexpected EOF, sent once, and one cut off with the deadline ahead, by
  any error, a timeout included, tried as 1.2.24 tries it. Cancellation
  while reading or waiting to retry; one attempt for a recognized account
  limit.
- Arbitrary secrets, echoed input, HTML, URLs, long messages and unsafe
  suffixes never reaching the displayed reason.
- Provider changes before completion and before a recovery tap; inactive
  provider key tests; no model mutation or retry for the wrong provider.
- Study, Ask, Compare, compact and sidebar Find, and Settings Test key;
  retry/faster/billing actions, stale completion, disabled Copy/Report and
  short portrait/landscape layouts.

Run the focused AI error and panel tests before the complete default,
`next` and race suites. Runtime layout checks use a memory-only key store,
seed Scripture, denied outbound requests and isolated preferences. Repeat
representative long account and timeout screens on native desktop, mimic,
iPhone/iPad simulators and Android emulator; a canvas rendering alone does
not prove a native reading overlay or a real device.

## Provider references

Checked 26 October 2026. Provider schemas and console URLs can change;
recheck official documentation when a new shape is added rather than
broadening a status-only guess.

- [OpenAI API errors](https://developers.openai.com/api/docs/guides/error-codes#api-errors)
  distinguishes request rate limits from account credit and spending limits.
- [Claude API errors](https://platform.claude.com/docs/en/api/errors) and
  [reaching a spend cap](https://platform.claude.com/docs/en/api/rate-limits#reaching-your-spend-cap)
  describe billing errors and spend-limit details.
- [Gemini API errors](https://ai.google.dev/gemini-api/docs/generate-content/api-errors)
  and [billing](https://ai.google.dev/gemini-api/docs/billing) distinguish
  depleted prepaid credit from request quota.
- [xAI debugging](https://docs.x.ai/developers/debugging) and
  [billing](https://docs.x.ai/console/billing) provide the account recovery
  context; an ordinary forbidden or rate-limit status is insufficient.

Not every recognised code is on these pages (checked 2 November 2026):
OpenAI's `billing_hard_limit_reached` and the xAI codes and clauses are absent
from them. They stay recognised because each is a specific account signal in
a structured field or a fixed opening clause; a status alone never classifies.
