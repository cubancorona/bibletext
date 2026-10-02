#!/usr/bin/env python3
"""THE SITE'S GLYPH GUARD: every character a page sets in a web font is drawn by
a web font the page declares, in a real cut.

    scripts/check-site-glyphs.py TREE

TREE is a built site (build/site). For every page, the stylesheets it links and
carries are read as BUILT — the hashed files, not the templates — and for every
text node the guard works out what the browser would: the element's font stack,
weight, style and text-transform through the cascade (with the browser's own
defaults for h1-h6, b, i and the rest), the faces CSS font matching selects for
that weight and style (unicode-range composites included), and then, character
by character, whether one of those faces carries it. A character no declared
face carries falls through to whatever the reader's system has — another
design, in the middle of a word — and that is what this refuses:

  - a character a page sets in a web font that no web font in its stack covers
    (Psalm 119's stanza letters in a system Hebrew, a small capital in a system
    italic);
  - a run set in a face the browser must FAKE: an italic where the family has
    no italic cut (a slanted regular), a bold where it has no bold.

Only text whose stack OPENS with a web font is checked. A page that asks for a
system face first (the hand-written landing pages) has chosen the reader's
system on purpose, and there is nothing of ours to cover it with: a generic, or
a system face SYSTEM_FACES names. A stack that opens with any other name is
refused, below.

TWO DELIBERATE EXCEPTIONS, both in the interface's type and not the reading
face's: the chapter arrows, which the chrome face does not carry and which fall
to the system sans by design (ALLOWED; cmd/websitegen/assets.go, .arrow), and
the chrome face's italic, which is served as a slanted regular (ALLOWED_FAKES).

FAIL CLOSED. The guard models the CSS this site writes, not all of CSS, and
whatever lies outside that model stops it with status 2 rather than letting it
guess, so a stylesheet change that moves past the model is a publish that
stops, not one that is waved through. Outside the model:

  - syntax the guard does not decode: a backslash escape anywhere in a
    stylesheet or a style attribute (a property or a custom property's name
    spelt with one), a rule nested inside a style rule (body{.text{...}},
    .text{@media ...{...}}), a comment wedged between two tokens, a stray
    semicolon before a rule, an @import, and any at-rule but the few it reads;
  - values it does not evaluate: a CSS-wide keyword other than inherit and
    unset on a font property (initial, revert, revert-layer), all, a weight
    range, a text-transform other than upper-, lower- and capitalize, a
    font-family list the browser would drop as invalid (a trailing comma), a
    stack that opens with a family that is neither a declared face, a
    generic nor a system face named below (a typo, "Junicod", included);
  - faces the browser makes up: font-synthesis, a small-caps, petite-caps,
    unicase or titling-caps variant, a sub- or superscript position;
  - generated text: content other than an empty string, a list marker other
    than none, disc, circle, square or decimal, text-emphasis marks, a
    text-overflow string or ellipsis, a hyphenate-character, <font face>;
  - a font rule behind :hover or inside @media, @supports, @layer or
    @container, or on a pseudo-element;
  - faces whose file is not what their @font-face says: a src other than one
    woff2 url(), a descriptor the guard does not read or one given twice, and
    a file whose own family, italic flag or weight class disagrees with the
    rule (the one deliberate alias is FACE_ALIASES);
  - stylesheets the browser may not apply: an alternate or titled sheet, one
    with a media query or a type, one inside <noscript> or <template>, and a
    <base> that would move every relative link.

Custom properties are the sharpest case. The guard reads a var() (any case of
the name var) from the custom properties set on plain :root, a rule whose whole
selector is :root, outside any @media, @supports, @layer or @container, and
from nowhere else: it does not cascade them per element. So a font value that
reads a custom property set ANYWHERE else as well — on another selector
(.text{--scripture:...}), on :root inside @media, in an @property rule, in a
page's style attribute — is refused, however the cascade would resolve it,
because the browser may draw that element in a face the guard never looked at.
So is one that reads a custom property set empty. A custom property no font
value reads may be set anywhere (the palette's dark colours are set on :root
inside @media). An at-rule the guard does not descend into (@scope,
@starting-style, @keyframes, @page) that sets a custom property, a font
property or generated text is refused too.

The cascade it does model follows CSS: the browser's own sheet, then the
author's normal declarations by specificity and order with a style attribute's
above them all, then the author's !important ones with a style attribute's
!important last; stylesheets are read in the order the page names them.

Prints code points, character names, the element's selector path and page
paths. NEVER page text: the tree may hold licensed Scripture.

Exit status: 0 covered, 1 a gap found, 2 the guard cannot judge the tree.
Requires fontTools and brotli (pip3 install --user fonttools brotli; on Ubuntu,
apt install python3-fonttools python3-brotli).
"""

import collections
import concurrent.futures
import html.parser
import os
import re
import sys
import unicodedata

try:
    from fontTools.ttLib import TTFont
except ImportError:  # pragma: no cover - environment
    sys.stderr.write("check-site-glyphs: needs fontTools and brotli "
                     "(pip3 install --user fonttools brotli)\n")
    sys.exit(2)


class CannotJudge(Exception):
    """A stylesheet or page the guard cannot evaluate: fail closed."""


# Deliberately left to the system: (code point, the family the stack opens
# with). The chapter arrows and the pager's arrows are set in the chrome face,
# which has no arrows, so they fall to the system sans behind it — which draws
# them with weight, where the scripture face drew hairlines
# (cmd/websitegen/assets.go, .arrow).
ALLOWED = {
    (0x2190, "atkinson hyperlegible"),
    (0x2192, "atkinson hyperlegible"),
}

# Faked cuts deliberately accepted: (the kind, the family). The chrome face is
# served in its regular and bold alone (web_fonts.go), and the one italic line
# of chrome, a notice page's note under the passage (notice.css, .vnote), is a
# slanted regular. That is the interface's type, not the reading face's, and
# this guard exists for the reading face.
ALLOWED_FAKES = {
    ("italic", "atkinson hyperlegible"),
}

# Characters that are not drawn: collapsible white space and the
# default-ignorable code points, which need no glyph.
SKIP = {0x20, 0x09, 0x0A, 0x0D, 0x0C, 0x00AD, 0x034F, 0x061C, 0x180E, 0xFEFF}
SKIP |= set(range(0x200B, 0x2010)) | set(range(0x202A, 0x202F))
SKIP |= set(range(0x2060, 0x2070)) | set(range(0xFE00, 0xFE10))

GENERIC = {"serif", "sans-serif", "monospace", "cursive", "fantasy", "system-ui",
           "ui-serif", "ui-sans-serif", "ui-monospace", "ui-rounded", "math",
           "emoji", "fangsong", "-apple-system", "blinkmacsystemfont"}

# The system faces a page may open its stack with, by name: the hand-written
# landing page is set in Georgia (docs/index.html). Any other family a stack
# opens with must be a face the page declares or a generic, or the guard
# cannot say what draws the text — a misspelt "Junicod" falls straight to
# whatever comes next — and refuses the tree.
SYSTEM_FACES = {"georgia"}

# A face declared under another family's name on purpose: (the family the
# rule declares, the family the file names itself). The Hebrew face joins the
# reading face's family by its own unicode-range, at the regular weight and
# the bold, from its one regular file (cmd/websitegen/assets.go), so it is
# exempt from the weight check and must carry a unicode-range.
FACE_ALIASES = {("junicode", "bibletext hebrew")}

FONT_PROPS = {"font-family", "font-weight", "font-style", "text-transform"}
CSS_WIDE = {"initial", "inherit", "unset", "revert", "revert-layer"}
TRANSFORMS = {"none", "uppercase", "lowercase", "capitalize"}
LIST_MARKERS = {"none", "disc", "circle", "square", "decimal", "inside", "outside"}
SYNTH_VARIANTS = {"small-caps", "all-small-caps", "petite-caps", "all-petite-caps", "unicase",
                  "titling-caps", "sub", "super"}
# The descriptors an @font-face may carry: any other (font-stretch,
# size-adjust, font-feature-settings ...) is outside the model.
FACE_DESCRIPTORS = {"font-family", "src", "font-style", "font-weight", "font-display", "unicode-range"}
# At-rules: those whose rules the guard reads as conditional, those whose
# bodies it only searches for what they must not set, and the statements it
# lets pass. Any other at-rule is refused.
DESCEND = {"@media", "@supports", "@layer", "@container"}
UNREAD = {"@scope", "@starting-style", "@keyframes", "@-webkit-keyframes", "@page"}
STATEMENTS = {"@charset", "@layer"}
# An identifier, as CSS writes one: the shape a class, an id, an attribute
# name and an unquoted family name must have, or the browser drops the rule.
IDENT = r"(?:--|-?[_a-zA-Z\u0080-\U0010FFFF])[-\w\u0080-\U0010FFFF]*"
IDENT_RE = re.compile(IDENT)
# Where an element sits makes the stylesheets inside it inert, or not
# reliably applied.
INERT = {"noscript", "template", "textarea", "title", "xmp", "iframe", "noembed", "noframes", "object"}
DYNAMIC = {"hover", "focus", "active", "target", "visited", "focus-visible",
           "focus-within", "checked", "link", "any-link", "enabled", "disabled",
           "placeholder-shown", "invalid", "valid", "default", "indeterminate"}
NOT_TEXT = {"script", "style", "template", "noscript", "head", "title", "svg",
            "math", "iframe", "object", "canvas", "video", "audio", "img"}
VOID = {"area", "base", "br", "col", "embed", "hr", "img", "input", "link", "meta",
        "param", "source", "track", "wbr"}
# An open element of these kinds is closed by the start of another of them.
IMPLIED_END = {"p": {"p", "div", "ul", "ol", "dl", "h1", "h2", "h3", "h4", "h5", "h6",
                     "section", "article", "aside", "nav", "footer", "header", "table", "pre"},
               "li": {"li"}, "dt": {"dt", "dd"}, "dd": {"dt", "dd"}, "option": {"option"}}

# The browser's own stylesheet, for the font properties.
UA_WEIGHT = {"h1": "bold", "h2": "bold", "h3": "bold", "h4": "bold", "h5": "bold",
             "h6": "bold", "th": "bold", "b": "bolder", "strong": "bolder", "optgroup": "bold"}
UA_ITALIC = {"i", "em", "cite", "var", "dfn", "address"}
UA_MONO = {"code", "kbd", "samp", "pre", "tt"}
UA_CONTROL = {"button", "input", "select", "textarea"}


# --- CSS -----------------------------------------------------------------------

# A comment between these and anything, or anything and these, separates
# nothing that was not separate already, so taking it out changes no token.
_SEPARATORS = set("{};:,>+~()[]!=")


def strip_comments(css, where="a stylesheet"):
    """The CSS with its comments taken out as the tokenizer takes them: never
    inside a string, so a "/*" in a string value does not hide the rules after
    it. A comment wedged between two tokens (Juni/**/code, .a/**/.b) separates
    them without being white space, which no text substitution can model, so
    it is refused."""
    out, i, n, last, quote = [], 0, len(css), 0, None
    while i < n:
        c = css[i]
        if quote:
            if c == "\\":
                i += 2
                continue
            if c == quote or c == "\n":
                quote = None
        elif c in "\"'":
            quote = c
        elif c == "/" and css.startswith("*", i + 1):
            end = css.find("*/", i + 2)
            end = n if end < 0 else end + 2
            before = css[i - 1] if i else " "
            after = css[end] if end < n else " "
            if not (before.isspace() or after.isspace() or before in _SEPARATORS or after in _SEPARATORS):
                raise CannotJudge(f"a comment between two tokens in {where}, which the guard cannot place")
            out.append(css[last:i])
            last = i = end
            continue
        i += 1
    out.append(css[last:])
    return "".join(out)


def unquoted(text):
    """text with every string's contents blanked, leaving only its syntax."""
    return re.sub(r'"[^"\n]*"|\'[^\'\n]*\'', '""', text)


def at_name(prelude):
    m = re.match(r"@[-\w]+", prelude.strip().lower())
    return m.group(0) if m else ""


def statements(text, closed):
    """Refuse what stands between rules unless it is a statement at-rule the
    guard may pass (@charset, @layer a, b). An @import is not followed, and a
    stray semicolon is refused: the browser reads it as the start of the next
    rule's selector and drops that rule, which the guard would have read.
    closed: text ends where a rule begins, so every part of it must have been
    a statement ended by its semicolon."""
    parts = text.split(";")
    for n, s in enumerate(parts):
        s = s.strip()
        if not s and not closed and n == len(parts) - 1:
            continue  # white space after the last statement
        name = at_name(s)
        if name == "@import":
            raise CannotJudge("an @import the guard does not follow")
        if name not in STATEMENTS:
            raise CannotJudge(f"{s[:40]!r} between rules, where the browser does not read what the guard would"
                              if s else "a stray semicolon, which makes the browser drop the rule after it")


def blocks(css, conditional=False):
    """Yield (prelude, body, conditional) for every rule, descending into the
    grouping at-rules."""
    i, n = 0, len(css)
    while i < n:
        j = css.find("{", i)
        if j < 0:
            statements(css[i:], False)
            return
        prelude = css[i:j]
        # A statement at-rule (@charset, @layer a, b) ends in a semicolon
        # before the next block opens; anything else there is refused.
        if ";" in prelude:
            head, _, prelude = prelude.rpartition(";")
            statements(head, True)
        depth, k, quote = 1, j + 1, None
        while k < n and depth:
            c = css[k]
            if quote:
                if c == "\\":
                    k += 1
                elif c == quote:
                    quote = None
            elif c in "\"'":
                quote = c
            elif c == "{":
                depth += 1
            elif c == "}":
                depth -= 1
            k += 1
        if depth:
            raise CannotJudge("a rule that is never closed")
        body = css[j + 1:k - 1]
        prelude = prelude.strip()
        name = at_name(prelude) if prelude.startswith("@") else None
        if name in DESCEND:
            yield from blocks(body, True)
            i = k
            continue
        if name is not None and name not in UNREAD and name not in ("@font-face", "@property"):
            raise CannotJudge(f"an at-rule the guard does not know: {name or prelude[:40]!r}")
        if name not in UNREAD and "{" in unquoted(body):
            # CSS nesting: the browser applies the inner rule, which the guard
            # would read as a declaration named ".text{--scripture".
            raise CannotJudge(f"a rule nested inside {prelude[:60]!r}, which the guard does not read")
        # A style rule, an @font-face, an @property, or an at-rule the caller
        # searches for what it sets (Sheets).
        yield prelude, body, conditional
        i = k


def declarations(body):
    out = []
    depth, start, quote = 0, 0, None
    for k, c in enumerate(body + ";"):
        if quote:
            if c == quote:
                quote = None
            continue
        if c in "\"'":
            quote = c
        elif c == "(":
            depth += 1
        elif c == ")":
            depth -= 1
        elif c == ";" and depth == 0:
            decl = body[start:k].strip()
            start = k + 1
            if ":" not in decl:
                continue
            prop, value = decl.split(":", 1)
            value = value.strip()
            important = bool(re.search(r"!\s*important\s*$", value, re.I))
            value = re.sub(r"!\s*important\s*$", "", value, flags=re.I).strip()
            # A custom property's name is case-sensitive (--Scripture is not
            # --scripture); every other property's is not.
            prop = prop.strip()
            out.append((prop if prop.startswith("--") else prop.lower(), value, important))
    return out


def parse_range(text):
    cps = set()
    for part in text.split(","):
        part = part.strip().upper()
        if not part.startswith("U+"):
            raise CannotJudge(f"a unicode-range entry the guard cannot read: {part!r}")
        part = part[2:]
        try:
            if "?" in part:
                lo, hi = int(part.replace("?", "0"), 16), int(part.replace("?", "F"), 16)
            elif "-" in part:
                a, b = part.split("-")
                lo, hi = int(a, 16), int(b, 16)
            else:
                lo = hi = int(part, 16)
        except ValueError:
            raise CannotJudge(f"a unicode-range entry the guard cannot read: U+{part!r}") from None
        cps.update(range(lo, hi + 1))
    return frozenset(cps)


SYSTEM_FONTS = {"caption", "icon", "menu", "message-box", "small-caption", "status-bar"}
SIZE_KEYWORDS = {"xx-small", "x-small", "small", "medium", "large", "x-large", "xx-large",
                 "xxx-large", "larger", "smaller"}
STRETCH = {"ultra-condensed", "extra-condensed", "condensed", "semi-condensed",
           "semi-expanded", "expanded", "extra-expanded", "ultra-expanded"}


def expand_font(value):
    """The font shorthand as the three longhands this guard reads: it sets the
    style, the weight and the family, and resets whichever it leaves out."""
    v = value.strip()
    low = v.lower()
    if low in SYSTEM_FONTS or low.startswith("-apple-system-"):
        return [("font-family", "system-ui"), ("font-weight", "400"), ("font-style", "normal")]
    if low in ("inherit", "unset"):
        return [("font-family", "inherit"), ("font-weight", "inherit"), ("font-style", "inherit")]
    style, weight = "normal", "400"
    tokens = re.sub(r"\s*/\s*", "/", v).split()
    for i, tok in enumerate(tokens):
        t = tok.lower()
        size = t.split("/")[0]
        if size in SIZE_KEYWORDS or re.fullmatch(r"[\d.]+(px|em|rem|%|pt|pc|in|cm|mm|ex|ch|vw|vh|vmin|vmax|q)", size):
            family = " ".join(tokens[i + 1:])
            if not family:
                break
            return [("font-family", family), ("font-weight", weight), ("font-style", style)]
        if t in ("italic", "oblique"):
            style = t
        elif t in ("bold", "bolder", "lighter") or re.fullmatch(r"\d{3}", t):
            weight = t
        elif t == "small-caps":
            raise CannotJudge(f"a small-caps variant the guard does not model: {value!r}")
        elif t != "normal" and t not in STRETCH:
            break
    raise CannotJudge(f"a font shorthand the guard cannot read: {value!r}")


def family_list(value):
    """The families of a font-family value, lowercased, as the browser reads
    them: each a quoted string or a run of identifiers, never empty. A value the
    browser would drop as invalid (a trailing comma, a keyword among the names)
    is refused, because the browser would keep an earlier declaration the guard
    never weighed. So is a generic in quotes, which names a font called
    "serif" rather than the generic."""
    entries, cur, quote = [], [], None
    for c in value.strip() + ",":
        if quote:
            cur.append(c)
            if c == quote:
                quote = None
        elif c in "\"'":
            quote = c
            cur.append(c)
        elif c == ",":
            entries.append("".join(cur).strip())
            cur = []
        else:
            cur.append(c)
    if quote:
        raise CannotJudge(f"a font-family with an unclosed string: {value!r}")
    names = []
    for e in entries:
        m = re.fullmatch(r'"([^"]*)"|\'([^\']*)\'', e)
        if m:
            name = (m.group(1) if m.group(1) is not None else m.group(2)).lower()
            if name in GENERIC or name in CSS_WIDE:
                raise CannotJudge(f"a generic or keyword in quotes in a font-family: {value!r}")
            names.append(name)
            continue
        idents = [i.lower() for i in e.split()]
        if not idents or not all(IDENT_RE.fullmatch(i) for i in idents) or \
                any(i in CSS_WIDE or i == "default" for i in idents):
            raise CannotJudge(f"a font-family the browser would drop as invalid: {value!r}")
        names.append(" ".join(idents))
    return tuple(names)


def keyword(value, prop):
    """A font property's value with inherit and unset read as inheriting, which
    for these inherited properties they are; the other CSS-wide keywords are
    refused."""
    v = value.strip().lower()
    if v in ("inherit", "unset"):
        return "inherit"
    if v in CSS_WIDE:
        raise CannotJudge(f"{prop}:{v}, a keyword the guard does not evaluate")
    return value


_SIMPLE = re.compile(r"(#" + IDENT + r"|\." + IDENT + r"|\[[^\]]*\]|::?[-\w]+(?:\([^)]*\))?)")


class Compound:
    __slots__ = ("tag", "ids", "classes", "attrs", "pseudos", "nots", "pseudo_element", "spec")

    def __init__(self, text):
        m = re.match(r"([a-zA-Z][-\w]*|\*)?", text)
        self.tag = (m.group(1) or "*").lower()
        rest = text[m.end():]
        self.ids, self.classes, self.attrs, self.pseudos, self.nots = [], [], [], [], []
        self.pseudo_element = None
        a = b = 0
        c = 0 if self.tag == "*" else 1
        pos = 0
        while pos < len(rest):
            m = _SIMPLE.match(rest, pos)
            if not m:
                raise CannotJudge(f"a selector the guard cannot read: {text!r}")
            s = m.group(1)
            pos = m.end()
            if s.startswith("#"):
                self.ids.append(s[1:]); a += 1
            elif s.startswith("."):
                self.classes.append(s[1:]); b += 1
            elif s.startswith("["):
                am = re.fullmatch(r"\[\s*(" + IDENT + r")\s*(?:([~|^$*]?=)\s*(\"[^\"]*\"|'[^']*'|" + IDENT +
                                  r")\s*)?\]", s)
                if not am:
                    raise CannotJudge(f"an attribute selector the guard cannot read: {s!r}")
                val = am.group(3)
                if val is not None and val[:1] in "\"'":
                    val = val[1:-1]
                self.attrs.append((am.group(1).lower(), am.group(2), val))
                b += 1
            elif s.startswith("::") or s.lower() in (":before", ":after", ":first-letter", ":first-line"):
                self.pseudo_element = s.lstrip(":").lower(); c += 1
            else:
                name = s[1:].lower()
                if name.startswith("not("):
                    inner = Compound(name[4:-1].strip())
                    self.nots.append(inner)
                    a += inner.spec[0]; b += inner.spec[1]; c += inner.spec[2]
                else:
                    self.pseudos.append(name); b += 1
        self.spec = (a, b, c)

    def key(self):
        if self.ids:
            return "#" + self.ids[0]
        if self.classes:
            return "." + self.classes[0]
        return self.tag

    def matches(self, el):
        if self.tag != "*" and el.tag != self.tag:
            return False
        if self.ids and el.attrs.get("id") not in self.ids:
            return False
        for c in self.classes:
            if c not in el.classes:
                return False
        for name, op, val in self.attrs:
            have = el.attrs.get(name)
            if have is None:
                return False
            if op == "=" and have != val:
                return False
            if op == "~=" and val not in have.split():
                return False
            if op == "^=" and not have.startswith(val):
                return False
            if op == "$=" and not have.endswith(val):
                return False
            if op == "*=" and val not in have:
                return False
            if op == "|=" and not (have == val or have.startswith(val + "-")):
                return False
        for p in self.pseudos:
            if p == "first-child":
                if el.index != 0:
                    return False
            elif p == "last-child":
                if el.parent is None or el.index != len(el.parent.elements) - 1:
                    return False
            elif p == "root":
                if el.parent is not None and el.parent.tag != "#document":
                    return False
            else:
                return False  # dynamic: never true of a page at rest
        for n in self.nots:
            if n.matches(el):
                return False
        return True


class Selector:
    __slots__ = ("parts", "spec", "dynamic", "pseudo_element", "unknown")

    def __init__(self, text):
        tokens = re.findall(r"\s*([>+~])\s*|\s+|([^\s>+~]+(?:\([^)]*\))?[^\s>+~]*)", text.strip())
        parts, comb = [], " "
        for c, compound in tokens:
            if c:
                comb = c
            elif compound:
                parts.append((comb, Compound(compound)))
                comb = " "
        if not parts:
            raise CannotJudge(f"an empty selector: {text!r}")
        self.parts = parts
        self.spec = tuple(sum(p[1].spec[i] for p in parts) for i in range(3))
        pseudos = {ps for _, p in parts for ps in p.pseudos}
        self.dynamic = bool(pseudos & DYNAMIC)
        self.unknown = bool(pseudos - DYNAMIC - {"first-child", "last-child", "root"})
        self.pseudo_element = parts[-1][1].pseudo_element

    def matches(self, el):
        return self._match(len(self.parts) - 1, el)

    def _match(self, i, el):
        comb, compound = self.parts[i]
        if not compound.matches(el):
            return False
        if i == 0:
            return True
        if comb == " ":
            p = el.parent
            while p is not None and p.tag != "#document":
                if self._match(i - 1, p):
                    return True
                p = p.parent
            return False
        if comb == ">":
            p = el.parent
            return p is not None and p.tag != "#document" and self._match(i - 1, p)
        siblings = el.parent.elements if el.parent is not None else []
        if comb == "+":
            return el.index > 0 and self._match(i - 1, siblings[el.index - 1])
        if comb == "~":
            return any(self._match(i - 1, s) for s in siblings[:el.index])
        return False


class Face:
    __slots__ = ("family", "style", "weight", "urange", "path")

    def __init__(self, family, style, weight, urange, path):
        self.family, self.style, self.weight, self.urange, self.path = family, style, weight, urange, path


# A custom property's name, as var() reads it.
VAR_NAME = r"--[-\w\u0080-\U0010FFFF]*"
VAR_RE = re.compile(r"var\(\s*(" + VAR_NAME + r")\s*(?:,([^)]*))?\)", re.I)

# What the body of an at-rule the guard does not read (@scope, @starting-style,
# @keyframes, @page) must not set: a custom property, a font property or
# generated text, any of which could change the face a page is drawn in or the
# characters it draws.
AT_RULE_SETS = re.compile(r"(?:^|[{;\s])((?:-[a-z]+-)?(?:--[-\w]+|font[-\w]*|text-transform|all|content|"
                          r"list-style[-\w]*|text-emphasis[-\w]*|text-overflow|hyphenate-character))\s*:", re.I)


def refuse_unmodelled(prop, value, where):
    """FAIL CLOSED on a declaration that resets the font properties, makes the
    browser fake a face, or draws characters that are not in the page's text,
    in a way the guard does not model. Checked for every rule and every style
    attribute, whether or not it matches an element."""
    base = prop if prop.startswith("--") else re.sub(r"^-[a-z]+-", "", prop)
    v = value.strip().lower()
    words = set(re.split(r"[\s,]+", v)) - {""}
    if base == "all":
        raise CannotJudge(f"all:{v} at {where}, which resets the font properties the guard reads")
    if base.startswith("font-synthesis"):
        raise CannotJudge(f"font-synthesis at {where}")
    if base in ("font-variant", "font-variant-caps", "font-variant-position") and (words & SYNTH_VARIANTS or "(" in v):
        raise CannotJudge(f"{prop}:{v} at {where}, a variant the browser may synthesise")
    if base == "content" and v not in ("none", "normal", '""', "''"):
        raise CannotJudge(f"generated text the guard does not draw: content at {where}")
    if base in ("list-style", "list-style-type") and not words <= LIST_MARKERS:
        raise CannotJudge(f"generated text the guard does not draw: {prop}:{v} at {where}")
    if base in ("text-emphasis", "text-emphasis-style") and v != "none":
        raise CannotJudge(f"generated text the guard does not draw: {prop} at {where}")
    if base == "text-overflow" and v != "clip":
        raise CannotJudge(f"generated text the guard does not draw: text-overflow at {where}")
    if base == "hyphenate-character" and v != "auto":
        raise CannotJudge(f"generated text the guard does not draw: hyphenate-character at {where}")
    if base == "text-security" and v != "none":
        raise CannotJudge(f"generated text the guard does not draw: {prop} at {where}")


class Sheets:
    """Everything the pages linking one set of stylesheets share: the faces, the
    custom properties on plain :root, where else any custom property is set, and
    the font rules indexed by the key of their last compound."""

    def __init__(self, sources):
        self.faces = collections.defaultdict(list)
        self.vars = {}        # name -> (important, value), from plain :root
        self.elsewhere = {}   # name -> where else it is set
        self.index = collections.defaultdict(list)
        order = 0
        for css, base, origin in sources:
            css = strip_comments(css, origin)
            if "\\" in css:
                raise CannotJudge(f"a backslash escape in {origin}; the guard does not decode escapes, so it cannot "
                                  f"say what a name or value written with one is")
            for prelude, body, conditional in blocks(css):
                name = at_name(prelude) if prelude.startswith("@") else None
                if name == "@font-face":
                    self._face(declarations(body), base, conditional)
                    continue
                if name == "@property":
                    prop = prelude.split(None, 1)[1].strip() if len(prelude.split()) > 1 else ""
                    self.elsewhere.setdefault(prop, f"an {prelude.strip()!r} rule")
                    continue
                if name is not None:
                    m = AT_RULE_SETS.search(body)
                    if m:
                        raise CannotJudge(f"{m.group(1)} set inside {name}, an at-rule the guard does not read")
                    continue
                decls = declarations(body)
                font_decls = []
                for p, v, imp in decls:
                    refuse_unmodelled(p, v, repr(prelude))
                    if p == "font":
                        font_decls += [(lp, lv, imp) for lp, lv in expand_font(v)]
                    elif p in FONT_PROPS:
                        font_decls.append((p, v, imp))
                selectors = [s.strip() for s in prelude.split(",") if s.strip()]
                # Plain :root is a rule whose whole selector is :root: one that
                # lists another selector beside it is dropped whole by the
                # browser if that one is invalid, which the guard cannot see.
                plain_root = prelude.strip() == ":root" and not conditional
                for sel_text in selectors:
                    where = sel_text if len(selectors) == 1 else f"{sel_text} in {prelude.strip()!r}"
                    where += " inside a conditional at-rule" if conditional else ""
                    for p, v, imp in decls:
                        if not p.startswith("--"):
                            continue
                        if not plain_root:
                            self.elsewhere.setdefault(p, where)
                        elif p not in self.vars or imp or not self.vars[p][0]:
                            # Every plain :root rule has the same specificity:
                            # the later wins, unless the earlier is !important.
                            self.vars[p] = (imp, v)
                    if not font_decls:
                        continue
                    sel = Selector(sel_text)
                    if conditional:
                        raise CannotJudge(f"a font rule inside @media/@container: {sel_text!r}")
                    if sel.dynamic or sel.unknown:
                        raise CannotJudge(f"a font rule behind a state the guard cannot see: {sel_text!r}")
                    if sel.pseudo_element:
                        raise CannotJudge(f"a font rule on generated content: {sel_text!r}")
                    order += 1
                    self.index[sel.parts[-1][1].key()].append((sel, order, font_decls))
        # Statically, for every font value in every rule, whether or not a page
        # has an element it matches.
        for rules in self.index.values():
            for _, _, decls in rules:
                for _, value, _ in decls:
                    self.refuse_set_elsewhere(value)
        # And every face, used or not: its file is in the tree and is what the
        # rule says it is.
        for faces in self.faces.values():
            for face in faces:
                check_face_file(face)

    def refuse_set_elsewhere(self, value, inline=None):
        """FAIL CLOSED on a font value that reads a custom property set anywhere
        but plain :root outside a conditional at-rule — directly, through
        another custom property's value or in a var() fallback — or one set
        empty. inline is the custom properties a page's style attributes set."""
        todo, seen = [value], set()
        while todo:
            for name, _ in VAR_RE.findall(todo.pop()):
                if name in seen:
                    continue
                seen.add(name)
                where = self.elsewhere.get(name) or (inline or {}).get(name)
                if where:
                    raise CannotJudge(f"a font value reads {name}, which is set at {where}; the guard reads custom "
                                      f"properties from plain :root alone, so it cannot say which face draws it")
                if name in self.vars:
                    if not self.vars[name][1].strip():
                        raise CannotJudge(f"a font value reads {name}, which is set empty")
                    todo.append(self.vars[name][1])

    def _face(self, decls, base, conditional):
        if conditional:
            raise CannotJudge("an @font-face inside @media/@supports")
        props = [p for p, _, _ in decls]
        if len(set(props)) != len(props) or any(imp for _, _, imp in decls):
            raise CannotJudge(f"an @font-face with a descriptor given twice or marked !important: {props}")
        d = {p: v for p, v, _ in decls}
        other = set(d) - FACE_DESCRIPTORS
        if other:
            raise CannotJudge(f"an @font-face descriptor the guard does not model: {sorted(other)}")
        family = family_list(d.get("font-family", ""))
        if len(family) != 1 or family[0] in GENERIC:
            raise CannotJudge(f"an @font-face family the guard cannot read: {d.get('font-family')!r}")
        # One woff2 file and nothing else. A list lets the browser skip an
        # entry (a format it does not take, a local() font the reader may
        # have) and load another, so the file the guard reads would not be
        # the one that draws.
        src = re.fullmatch(r"url\(\s*(\"|'|)([^\"'()\s]+)\1\s*\)(?:\s*format\(\s*(\"|'|)woff2\3\s*\))?",
                           d.get("src", "").strip(), re.I)
        if not src:
            raise CannotJudge(f"an @font-face src other than one woff2 url(): {d.get('src', '')!r}")
        weight = d.get("font-weight", "400").strip().lower()
        weight = {"normal": "400", "bold": "700"}.get(weight, weight)
        if not re.fullmatch(r"\d+", weight):
            raise CannotJudge(f"an @font-face weight the guard does not model: {weight!r}")
        style = d.get("font-style", "normal").strip().lower()
        if style not in ("normal", "italic", "oblique"):
            raise CannotJudge(f"an @font-face style the guard does not model: {style!r}")
        url = src.group(2)
        if re.match(r"[a-z]+:", url, re.I) or url.startswith("//"):
            raise CannotJudge(f"a face served from elsewhere: {url}")
        path = os.path.normpath(os.path.join(base, url))
        urange = parse_range(d["unicode-range"]) if "unicode-range" in d else None
        self.faces[family[0]].append(Face(family[0], style, int(weight), urange, path))

    def rules_for(self, el):
        keys = ["*", el.tag] + ["." + c for c in el.classes]
        if "id" in el.attrs:
            keys.append("#" + el.attrs["id"])
        for k in keys:
            for rule in self.index.get(k, ()):
                yield rule

    def resolve(self, value, inline=None):
        # The stylesheets' own font values were checked when they were read;
        # what a page's style attributes set is checked here, element by
        # element, against every value that reaches one.
        if inline:
            self.refuse_set_elsewhere(value, inline)
        for _ in range(8):
            m = VAR_RE.search(value)
            if not m:
                return value
            if m.group(1) in self.vars:
                repl = self.vars[m.group(1)][1]
            elif m.group(2) is not None:
                repl = m.group(2)
            else:
                raise CannotJudge(f"a font value naming an unset property: {m.group(1)}")
            if not repl.strip():
                raise CannotJudge(f"a font value reads {m.group(1)}, which comes out empty")
            value = value[:m.start()] + repl + value[m.end():]
        raise CannotJudge(f"a font value the guard cannot resolve: {value!r}")


# --- HTML -----------------------------------------------------------------------

class Element:
    __slots__ = ("tag", "attrs", "classes", "parent", "children", "elements", "index", "decls")

    def __init__(self, tag, attrs, parent):
        self.tag, self.attrs, self.parent = tag, attrs, parent
        self.classes = set((attrs.get("class") or "").split())
        self.children, self.elements, self.index, self.decls = [], [], 0, []
        if parent is not None:
            self.index = len(parent.elements)
            parent.elements.append(self)
            parent.children.append(self)


def style_declarations(text, where):
    """A style attribute's declarations, read as a stylesheet's are: comments
    out, and refused if it carries an escape or a rule."""
    css = strip_comments(text, where)
    if "\\" in css:
        raise CannotJudge(f"a backslash escape in {where}; the guard does not decode escapes")
    if "{" in unquoted(css):
        raise CannotJudge(f"a rule inside {where}")
    return declarations(css)


class Page(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.doc = Element("#document", {}, None)
        self.open = [self.doc]
        self.sheets = []  # ("link", href) or ("style", css), in the order the page has them
        self.inline_vars = {}
        self._style = None

    def applied(self, tag, a):
        """FAIL CLOSED on a stylesheet the browser may not apply: the guard
        reads every one it is given as applied, so one the browser skips could
        carry the good value that hides a bad one."""
        inert = [e.tag for e in self.open if e.tag in INERT]
        if inert:
            raise CannotJudge(f"a <{tag}> stylesheet inside <{inert[0]}>, which the browser may not apply")
        rel = a.get("rel", "").lower().split()
        media = a.get("media", "all").strip().lower()
        kind = a.get("type", "text/css").strip().lower()
        if "alternate" in rel or "title" in a or "disabled" in a or media != "all" or kind != "text/css":
            raise CannotJudge(f"a <{tag}> stylesheet the browser may not apply: alternate, titled, disabled, "
                              f"or with a media query or a type")

    def handle_starttag(self, tag, attrs):
        a = {}
        for k, v in attrs:
            # Of two attributes with one name, the browser keeps the first.
            a.setdefault(k.lower(), v if v is not None else "")
        if tag == "base":
            raise CannotJudge("a <base>, which moves every relative link the guard resolves")
        if tag == "font":
            raise CannotJudge("a <font> element, whose face attribute sets a family the guard does not read")
        top = self.open[-1]
        while top.tag in IMPLIED_END and tag in IMPLIED_END[top.tag]:
            self.open.pop()
            top = self.open[-1]
        el = Element(tag, a, top)
        if "style" in a:
            where = f"a style attribute on <{tag}>"
            el.decls = style_declarations(a["style"], where)
            for p, v, _ in el.decls:
                refuse_unmodelled(p, v, where)
                if p.startswith("--"):
                    self.inline_vars.setdefault(p, where)
        if tag == "link" and "stylesheet" in a.get("rel", "").lower().split():
            self.applied(tag, a)
            self.sheets.append(("link", a.get("href", "")))
        if tag == "style":
            self.applied(tag, a)
            self._style = []
        if tag not in VOID:
            self.open.append(el)

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in VOID and self.open[-1].tag == tag:
            self.open.pop()

    def handle_endtag(self, tag):
        if tag == "style" and self._style is not None:
            self.sheets.append(("style", "".join(self._style)))
            self._style = None
        for i in range(len(self.open) - 1, 0, -1):
            if self.open[i].tag == tag:
                del self.open[i:]
                return

    def handle_data(self, data):
        if self._style is not None:
            self._style.append(data)
            return
        self.open[-1].children.append(data)


# --- the cascade and font matching --------------------------------------------------

def bolder(w):
    return 400 if w < 350 else 700 if w < 550 else 900


def lighter(w):
    return 100 if w < 550 else 400 if w < 750 else 700


def weight_of(value, parent):
    v = value.strip().lower()
    if v == "inherit":
        return parent
    if v == "normal":
        return 400
    if v == "bold":
        return 700
    if v == "bolder":
        return bolder(parent)
    if v == "lighter":
        return lighter(parent)
    if re.fullmatch(r"\d+", v):
        return int(v)
    raise CannotJudge(f"a font-weight the guard does not model: {value!r}")


def computed(el, parent, sheets, inline=None):
    family, weight, style, transform = parent
    winners = {}
    # The browser's own sheet first; then the author's normal declarations by
    # specificity and order, with a style attribute's above every selector;
    # then the author's !important ones, a style attribute's last. A rank is
    # (importance, from a style attribute, specificity, order).
    ua = (-1, 0, (0, 0, 0), 0)
    if el.tag in UA_WEIGHT:
        winners["font-weight"] = (ua, UA_WEIGHT[el.tag])
    if el.tag in UA_ITALIC:
        winners["font-style"] = (ua, "italic")
    if el.tag in UA_MONO:
        winners["font-family"] = (ua, "monospace")
    if el.tag in UA_CONTROL:
        winners["font-family"] = (ua, "system-ui")
        winners["font-weight"] = (ua, "400")
        winners["font-style"] = (ua, "normal")
    for sel, order, decls in sheets.rules_for(el):
        if not sel.matches(el):
            continue
        for prop, value, important in decls:
            rank = (1 if important else 0, 0, sel.spec, order)
            if prop not in winners or rank >= winners[prop][0]:
                winners[prop] = (rank, value)
    for prop, value, important in el.decls:
        longhands = expand_font(value) if prop == "font" else [(prop, value)]
        for lp, lv in longhands:
            if lp in FONT_PROPS:
                sheets.refuse_set_elsewhere(lv, inline)
                rank = (1 if important else 0, 1, (0, 0, 0), 0)
                if lp not in winners or rank >= winners[lp][0]:
                    winners[lp] = (rank, lv)
    if "font-family" in winners:
        v = keyword(sheets.resolve(winners["font-family"][1], inline), "font-family")
        if v != "inherit":
            family = family_list(v)
            first = family[0]
            if first not in sheets.faces and first not in GENERIC and first not in SYSTEM_FACES:
                raise CannotJudge(f"a font stack that opens with {first!r}, which is not a face the page declares, "
                                  f"a generic or a system face the guard knows, so it cannot say what draws the text")
    if "font-weight" in winners:
        weight = weight_of(keyword(sheets.resolve(winners["font-weight"][1], inline), "font-weight"), weight)
    if "font-style" in winners:
        v = keyword(sheets.resolve(winners["font-style"][1], inline), "font-style").strip().lower()
        if v != "inherit":
            if v.startswith("oblique"):
                v = "oblique"
            if v not in ("normal", "italic", "oblique"):
                raise CannotJudge(f"a font-style the guard does not model: {v!r}")
            style = v
    if "text-transform" in winners:
        v = keyword(sheets.resolve(winners["text-transform"][1], inline), "text-transform").strip().lower()
        if v != "inherit":
            if v not in TRANSFORMS:
                raise CannotJudge(f"a text-transform the guard does not model: {v!r}")
            transform = v
    return family, weight, style, transform


def match(faces, style, weight):
    """CSS Fonts 4, 5.2: narrow by style, then by weight; the composite is every
    face with the winning descriptors. Returns (faces, synthetic italic,
    synthetic bold)."""
    styles = {f.style for f in faces}
    order = {"italic": ["italic", "oblique", "normal"], "oblique": ["oblique", "italic", "normal"],
             "normal": ["normal", "oblique", "italic"]}[style]
    want = next(s for s in order if s in styles)
    faces = [f for f in faces if f.style == want]
    weights = sorted({f.weight for f in faces})
    if weight in weights:
        w = weight
    elif 400 <= weight <= 500:
        cands = [x for x in weights if weight <= x <= 500] + sorted([x for x in weights if x < weight], reverse=True)
        cands += [x for x in weights if x > 500]
        w = cands[0]
    elif weight < 400:
        cands = sorted([x for x in weights if x < weight], reverse=True) + [x for x in weights if x > weight]
        w = cands[0]
    else:
        cands = [x for x in weights if x > weight] + sorted([x for x in weights if x < weight], reverse=True)
        w = cands[0]
    return ([f for f in faces if f.weight == w], style != "normal" and want == "normal",
            weight >= 600 and w < 600)


_FILES = {}


def font_file(path):
    """(character map, family, italic, weight class) of a face file, read once."""
    if path not in _FILES:
        if not os.path.isfile(path):
            raise CannotJudge(f"a stylesheet names {os.path.basename(path)}, which is not in the tree")
        try:
            font = TTFont(path, lazy=True)
            names = font["name"]
            family = (names.getDebugName(16) or names.getDebugName(1) or "").strip().lower()
            os2 = font["OS/2"]
            italic = bool(os2.fsSelection & 0x201 or font["head"].macStyle & 2 or font["post"].italicAngle)
            _FILES[path] = (frozenset(font.getBestCmap() or ()), family, italic, os2.usWeightClass)
            font.close()
        except Exception as e:
            raise CannotJudge(f"{os.path.basename(path)} is not a face the guard can read "
                              f"({type(e).__name__})") from None
    return _FILES[path]


def cmap(path):
    return font_file(path)[0]


def check_face_file(face):
    """FAIL CLOSED on a face whose file is not what its @font-face says. The
    browser believes the descriptors: an upright file declared as the italic is
    drawn upright where the page asks for italic, and another family's file
    declared under the reading face's name draws the reading text in that other
    design. The Hebrew face is the one alias (FACE_ALIASES)."""
    _, family, italic, weight = font_file(face.path)
    name = os.path.basename(face.path)
    alias = (face.family, family) in FACE_ALIASES
    if family != face.family and not alias:
        raise CannotJudge(f"an @font-face declares {name} as {face.family!r}, but the file's family is {family!r}")
    if alias and face.urange is None:
        raise CannotJudge(f"{name} joins {face.family!r} with no unicode-range, so it would draw every character")
    if italic != (face.style != "normal"):
        raise CannotJudge(f"an @font-face declares {name} as {face.style}, but the file is "
                          f"{'italic' if italic else 'upright'}")
    if not alias and (weight >= 600) != (face.weight >= 600):
        raise CannotJudge(f"an @font-face declares {name} at weight {face.weight}, but its weight class is {weight}")


def transformed(text, transform):
    if transform == "uppercase":
        return text.upper()
    if transform == "lowercase":
        return text.lower()
    if transform == "capitalize":
        # CSS raises the first letter of each word and leaves the others as
        # they are; the guard checks every letter in both forms rather than
        # model where a word begins.
        return text + text.upper()
    return text


def describe(el):
    path = []
    while el is not None and el.tag not in ("#document", "html", "body"):
        label = el.tag + "".join("." + c for c in sorted(el.classes))
        path.append(label)
        el = el.parent
    return " ".join(reversed(path[:3]))


_SHEETS = {}


def check_page(tree, path):
    """Returns (gaps, fakes, checked): gaps maps (code point, family, context) to
    a count, fakes maps (kind, family, context) to a count."""
    rel = os.path.relpath(path, tree)
    page = Page()
    with open(path, encoding="utf-8") as f:
        page.feed(f.read())
    page.close()
    # The stylesheets in the order the page names them, which is the order of
    # the cascade: a <style> before a <link> loses to it.
    sources, key = [], []
    for i, (kind, ref) in enumerate(page.sheets):
        if kind == "style":
            key.append(f"{rel}#style{i}")
            sources.append((ref, os.path.dirname(path), f"a <style> in {rel}"))
            continue
        if re.match(r"[a-z]+:", ref, re.I) or ref.startswith("//"):
            raise CannotJudge(f"{rel} links a stylesheet from elsewhere: {ref}")
        target = os.path.join(tree, ref.lstrip("/")) if ref.startswith("/") else \
            os.path.join(os.path.dirname(path), ref)
        target = os.path.normpath(target.split("?")[0].split("#")[0])
        if not os.path.isfile(target):
            raise CannotJudge(f"{rel} links {ref}, which is not in the tree")
        key.append(target)
        sources.append((target, os.path.dirname(target), os.path.relpath(target, tree)))
    key = tuple(key)
    if key not in _SHEETS:
        read = []
        for src, base, origin in sources:
            if origin.startswith("a <style>"):
                read.append((src, base, origin))
            else:
                with open(src, encoding="utf-8") as f:
                    read.append((f.read(), base, origin))
        _SHEETS[key] = Sheets(read)
    sheets = _SHEETS[key]

    gaps, fakes = collections.Counter(), collections.Counter()
    checked = 0
    root_style = (("serif",), 400, "normal", "none")
    stack = [(page.doc, root_style, False)]
    while stack:
        el, inherited, hidden = stack.pop()
        style = inherited if el.tag == "#document" else computed(el, inherited, sheets, page.inline_vars)
        hidden = hidden or el.tag in NOT_TEXT
        text = []
        for child in el.children:
            if isinstance(child, Element):
                stack.append((child, style, hidden))
            elif not hidden:
                text.append(child)
        if hidden or not text:
            continue
        family, weight, fstyle, transform = style
        chars = collections.Counter(transformed("".join(text), transform))
        for cp in [c for c in chars if ord(c) in SKIP]:
            del chars[cp]
        if not chars:
            continue
        if not family or family[0] not in sheets.faces:
            continue  # a system face first, by the page's own choice
        context = describe(el)
        first, synth_i, synth_b = match(sheets.faces[family[0]], fstyle, weight)
        if synth_i and ("italic", family[0]) not in ALLOWED_FAKES:
            fakes[("a slanted regular (no italic cut)", family[0], context)] += sum(chars.values())
        if synth_b and ("bold", family[0]) not in ALLOWED_FAKES:
            fakes[("a thickened regular (no bold cut)", family[0], context)] += sum(chars.values())
        composites = {}
        for ch, n in chars.items():
            cp = ord(ch)
            checked += n
            covered = False
            for fam in family:
                faces = sheets.faces.get(fam)
                if not faces:
                    break  # a system or generic family: the character has left our faces
                if fam not in composites:
                    composites[fam] = match(faces, fstyle, weight)[0]
                if any((f.urange is None or cp in f.urange) and cp in cmap(f.path) for f in composites[fam]):
                    covered = True
                    break
            if not covered and (cp, family[0]) not in ALLOWED:
                gaps[(cp, f"{family[0]} {weight} {fstyle}", context)] += n
    return gaps, fakes, checked


def check_chunk(args):
    tree, paths = args
    out = []
    for path in paths:
        try:
            out.append((os.path.relpath(path, tree),) + check_page(tree, path) + (None,))
        except CannotJudge as e:
            out.append((os.path.relpath(path, tree), None, None, 0, str(e)))
    return out


def main(argv):
    if len(argv) != 2 or not os.path.isdir(argv[1]):
        sys.stderr.write("usage: check-site-glyphs.py TREE\n")
        return 2
    tree = os.path.abspath(argv[1])
    pages = []
    for root, _, files in os.walk(tree):
        pages += [os.path.join(root, f) for f in files if f.endswith(".html")]
    pages.sort()
    if not pages:
        sys.stderr.write(f"check-site-glyphs: no pages under {argv[1]}\n")
        return 2
    chunks = [(tree, pages[i:i + 64]) for i in range(0, len(pages), 64)]
    gaps = collections.defaultdict(lambda: [0, []])
    fakes = collections.defaultdict(lambda: [0, []])
    checked, cannot = 0, []
    workers = min(8, os.cpu_count() or 1)
    with concurrent.futures.ProcessPoolExecutor(max_workers=workers) as pool:
        for result in pool.map(check_chunk, chunks):
            for rel, g, f, n, err in result:
                if err:
                    cannot.append((rel, err))
                    continue
                checked += n
                for k, c in g.items():
                    gaps[k][0] += c
                    gaps[k][1].append(rel)
                for k, c in f.items():
                    fakes[k][0] += c
                    fakes[k][1].append(rel)
    if cannot:
        for rel, err in cannot[:5]:
            print(f"cannot judge {rel}: {err}")
        if len(cannot) > 5:
            print(f"... and {len(cannot) - 5} more pages")
        return 2

    def where(rels):
        return rels[0] + (f" and {len(rels) - 1} more pages" if len(rels) > 1 else "")

    for (cp, face, context), (n, rels) in sorted(gaps.items())[:40]:
        name = unicodedata.name(chr(cp), "UNNAMED")
        print(f"U+{cp:04X} {name} x{n}: in {context}, set in {face}, which no declared face carries — "
              f"it falls to a system font. {where(rels)}")
    for (kind, family, context), (n, rels) in sorted(fakes.items())[:20]:
        print(f"{context}: {n} characters set in {family} as {kind}. {where(rels)}")
    if gaps or fakes:
        more = len(gaps) + len(fakes) - min(len(gaps), 40) - min(len(fakes), 20)
        if more > 0:
            print(f"... and {more} more")
        return 1
    print(f"    glyphs: {len(pages)} pages, {checked} characters set in web faces, every one drawn by a declared face")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
