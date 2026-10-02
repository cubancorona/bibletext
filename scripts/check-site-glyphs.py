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
system on purpose, and there is nothing of ours to cover it with.

TWO DELIBERATE EXCEPTIONS, both in the interface's type and not the reading
face's: the chapter arrows, which the chrome face does not carry and which fall
to the system sans by design (ALLOWED; cmd/websitegen/assets.go, .arrow), and
the chrome face's italic, which is served as a slanted regular (ALLOWED_FAKES).

FAIL CLOSED. The guard models the CSS this site writes, not all of CSS. A font
property it cannot evaluate statically — a weight range, a small-caps variant,
font-synthesis, a font rule behind :hover or inside @media, generated text, an
unresolvable var(), a shorthand it cannot parse — stops it with status 2 rather
than letting it guess, so a stylesheet change that moves past the model is a
publish that stops, not one that is waved through.

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

FONT_PROPS = {"font-family", "font-weight", "font-style", "text-transform"}
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

def strip_comments(css):
    return re.sub(r"/\*.*?\*/", "", css, flags=re.S)


def blocks(css, conditional=False):
    """Yield (prelude, body, conditional) for every rule, descending into the
    grouping at-rules."""
    i, n = 0, len(css)
    while i < n:
        j = css.find("{", i)
        if j < 0:
            return
        prelude = css[i:j]
        # A statement at-rule (@import, @charset) ends in a semicolon before
        # the next block opens; keep only what follows the last one.
        if ";" in prelude:
            prelude = prelude[prelude.rfind(";") + 1:]
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
        body = css[j + 1:k - 1]
        prelude = prelude.strip()
        low = prelude.lower()
        if low.startswith(("@media", "@container", "@supports", "@layer", "@document")):
            yield from blocks(body, True)
        elif low.startswith("@import"):
            raise CannotJudge("an @import the guard does not follow")
        elif not low.startswith("@") or low.startswith("@font-face"):
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
            out.append((prop.strip().lower(), value, important))
    return out


def parse_range(text):
    cps = set()
    for part in text.split(","):
        part = part.strip().upper()
        if not part.startswith("U+"):
            raise CannotJudge(f"a unicode-range entry the guard cannot read: {part!r}")
        part = part[2:]
        if "?" in part:
            lo, hi = int(part.replace("?", "0"), 16), int(part.replace("?", "F"), 16)
        elif "-" in part:
            a, b = part.split("-")
            lo, hi = int(a, 16), int(b, 16)
        else:
            lo = hi = int(part, 16)
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
    if low == "inherit":
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


def family_names(value):
    out = []
    for part in re.findall(r'"[^"]*"|\'[^\']*\'|[^,]+', value):
        name = part.strip().strip("\"'").strip()
        if name:
            out.append(name.lower())
    return out


_SIMPLE = re.compile(r"(#[-\w]+|\.[-\w]+|\[[^\]]*\]|::?[-\w]+(?:\([^)]*\))?)")


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
                am = re.match(r"\[\s*([-\w]+)\s*(?:([~|^$*]?=)\s*[\"']?([^\"'\]]*)[\"']?\s*)?\]", s)
                if not am:
                    raise CannotJudge(f"an attribute selector the guard cannot read: {s!r}")
                self.attrs.append((am.group(1).lower(), am.group(2), am.group(3)))
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


class Sheets:
    """Everything the pages linking one set of stylesheets share: the faces, the
    custom properties on :root, and the font rules indexed by the key of their
    last compound."""

    def __init__(self, sources):
        self.faces = collections.defaultdict(list)
        self.vars = {}
        self.index = collections.defaultdict(list)
        order = 0
        for css, base in sources:
            for prelude, body, conditional in blocks(strip_comments(css)):
                decls = declarations(body)
                if prelude.lower().startswith("@font-face"):
                    self._face(decls, base, conditional)
                    continue
                font_decls = []
                for p, v, imp in decls:
                    if p == "font":
                        font_decls += [(lp, lv, imp) for lp, lv in expand_font(v)]
                    elif p in FONT_PROPS:
                        font_decls.append((p, v, imp))
                    elif p.startswith("font-synthesis"):
                        raise CannotJudge(f"font-synthesis at {prelude!r}")
                    elif p in ("font-variant", "font-variant-caps") and "small-caps" in v.lower():
                        raise CannotJudge(f"a small-caps variant at {prelude!r}")
                if any(p == "content" and re.search(r"(\"[^\"]+\"|'[^']+')", v) for p, v, _ in decls):
                    raise CannotJudge(f"generated text the guard does not draw: {prelude!r}")
                for sel_text in [s for s in prelude.split(",") if s.strip()]:
                    if sel_text.strip() == ":root" and not conditional:
                        for p, v, _ in decls:
                            if p.startswith("--"):
                                self.vars[p] = v
                    if not font_decls:
                        continue
                    sel = Selector(sel_text)
                    if conditional:
                        raise CannotJudge(f"a font rule inside @media/@container: {sel_text.strip()!r}")
                    if sel.dynamic or sel.unknown:
                        raise CannotJudge(f"a font rule behind a state the guard cannot see: {sel_text.strip()!r}")
                    if sel.pseudo_element:
                        raise CannotJudge(f"a font rule on generated content: {sel_text.strip()!r}")
                    order += 1
                    self.index[sel.parts[-1][1].key()].append((sel, order, font_decls))

    def _face(self, decls, base, conditional):
        if conditional:
            raise CannotJudge("an @font-face inside @media/@supports")
        d = {p: v for p, v, _ in decls}
        family = family_names(d.get("font-family", ""))
        src = re.search(r"url\(\s*['\"]?([^'\")]+)['\"]?\s*\)", d.get("src", ""))
        if len(family) != 1 or not src:
            raise CannotJudge(f"an @font-face the guard cannot read: {d}")
        weight = d.get("font-weight", "400").strip().lower()
        weight = {"normal": "400", "bold": "700"}.get(weight, weight)
        if not re.fullmatch(r"\d+", weight):
            raise CannotJudge(f"an @font-face weight the guard does not model: {weight!r}")
        style = d.get("font-style", "normal").strip().lower()
        if style not in ("normal", "italic", "oblique"):
            raise CannotJudge(f"an @font-face style the guard does not model: {style!r}")
        url = src.group(1)
        if re.match(r"[a-z]+:", url):
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

    def resolve(self, value):
        for _ in range(8):
            m = re.search(r"var\(\s*(--[-\w]+)\s*(?:,([^)]*))?\)", value)
            if not m:
                return value
            if m.group(1) in self.vars:
                repl = self.vars[m.group(1)]
            elif m.group(2) is not None:
                repl = m.group(2)
            else:
                raise CannotJudge(f"a font value naming an unset property: {m.group(1)}")
            value = value[:m.start()] + repl + value[m.end():]
        raise CannotJudge(f"a font value the guard cannot resolve: {value!r}")


# --- HTML -----------------------------------------------------------------------

class Element:
    __slots__ = ("tag", "attrs", "classes", "parent", "children", "elements", "index")

    def __init__(self, tag, attrs, parent):
        self.tag, self.attrs, self.parent = tag, attrs, parent
        self.classes = set((attrs.get("class") or "").split())
        self.children, self.elements, self.index = [], [], 0
        if parent is not None:
            self.index = len(parent.elements)
            parent.elements.append(self)
            parent.children.append(self)


class Page(html.parser.HTMLParser):
    def __init__(self):
        super().__init__(convert_charrefs=True)
        self.doc = Element("#document", {}, None)
        self.open = [self.doc]
        self.links, self.styles = [], []
        self._style = None

    def handle_starttag(self, tag, attrs):
        a = {k.lower(): (v if v is not None else "") for k, v in attrs}
        top = self.open[-1]
        while top.tag in IMPLIED_END and tag in IMPLIED_END[top.tag]:
            self.open.pop()
            top = self.open[-1]
        el = Element(tag, a, top)
        if tag == "link" and "stylesheet" in a.get("rel", "").lower().split():
            self.links.append(a.get("href", ""))
        if tag == "style":
            self._style = []
        if tag not in VOID:
            self.open.append(el)

    def handle_startendtag(self, tag, attrs):
        self.handle_starttag(tag, attrs)
        if tag not in VOID and self.open[-1].tag == tag:
            self.open.pop()

    def handle_endtag(self, tag):
        if tag == "style" and self._style is not None:
            self.styles.append("".join(self._style))
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


def computed(el, parent, sheets):
    family, weight, style, transform = parent
    winners = {}
    # The browser's own sheet first, then the author's by importance,
    # specificity and order.
    if el.tag in UA_WEIGHT:
        winners["font-weight"] = ((-1, (0, 0, 0), 0), UA_WEIGHT[el.tag])
    if el.tag in UA_ITALIC:
        winners["font-style"] = ((-1, (0, 0, 0), 0), "italic")
    if el.tag in UA_MONO:
        winners["font-family"] = ((-1, (0, 0, 0), 0), "monospace")
    if el.tag in UA_CONTROL:
        winners["font-family"] = ((-1, (0, 0, 0), 0), "system-ui")
        winners["font-weight"] = ((-1, (0, 0, 0), 0), "400")
        winners["font-style"] = ((-1, (0, 0, 0), 0), "normal")
    for sel, order, decls in sheets.rules_for(el):
        if not sel.matches(el):
            continue
        for prop, value, important in decls:
            rank = (1 if important else 0, sel.spec, order)
            if prop not in winners or rank >= winners[prop][0]:
                winners[prop] = (rank, value)
    if "style" in el.attrs:
        for prop, value, important in declarations(el.attrs["style"]):
            longhands = expand_font(value) if prop == "font" else [(prop, value)]
            for lp, lv in longhands:
                if lp in FONT_PROPS:
                    winners[lp] = ((2 if important else 1, (9, 9, 9), 0), lv)
    if "font-family" in winners:
        v = sheets.resolve(winners["font-family"][1])
        if v.strip().lower() != "inherit":
            family = tuple(family_names(v))
    if "font-weight" in winners:
        weight = weight_of(sheets.resolve(winners["font-weight"][1]), weight)
    if "font-style" in winners:
        v = sheets.resolve(winners["font-style"][1]).strip().lower()
        if v != "inherit":
            if v.startswith("oblique"):
                v = "oblique"
            if v not in ("normal", "italic", "oblique"):
                raise CannotJudge(f"a font-style the guard does not model: {v!r}")
            style = v
    if "text-transform" in winners:
        v = sheets.resolve(winners["text-transform"][1]).strip().lower()
        if v != "inherit":
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


_CMAPS = {}


def cmap(path):
    if path not in _CMAPS:
        if not os.path.isfile(path):
            raise CannotJudge(f"a stylesheet names {os.path.basename(path)}, which is not in the tree")
        font = TTFont(path, lazy=True)
        _CMAPS[path] = frozenset(font.getBestCmap() or ())
        font.close()
    return _CMAPS[path]


def transformed(text, transform):
    if transform == "uppercase":
        return text.upper()
    if transform == "lowercase":
        return text.lower()
    if transform == "capitalize":
        return text.title()
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
    sources, key = [], []
    for href in page.links:
        if re.match(r"[a-z]+:", href) or href.startswith("//"):
            raise CannotJudge(f"{rel} links a stylesheet from elsewhere: {href}")
        target = os.path.join(tree, href.lstrip("/")) if href.startswith("/") else \
            os.path.join(os.path.dirname(path), href)
        target = os.path.normpath(target.split("?")[0].split("#")[0])
        if not os.path.isfile(target):
            raise CannotJudge(f"{rel} links {href}, which is not in the tree")
        key.append(target)
    for i, css in enumerate(page.styles):
        key.append(f"{rel}#style{i}")
    key = tuple(key)
    if key not in _SHEETS:
        for target in key:
            if "#style" in target:
                css = page.styles[int(target.rsplit("#style", 1)[1])]
                sources.append((css, os.path.dirname(path)))
            else:
                with open(target, encoding="utf-8") as f:
                    sources.append((f.read(), os.path.dirname(target)))
        _SHEETS[key] = Sheets(sources)
    sheets = _SHEETS[key]

    gaps, fakes = collections.Counter(), collections.Counter()
    checked = 0
    root_style = (("serif",), 400, "normal", "none")
    stack = [(page.doc, root_style, False)]
    while stack:
        el, inherited, hidden = stack.pop()
        style = inherited if el.tag == "#document" else computed(el, inherited, sheets)
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
