package bibletext

// Which model an AI request is working on, as the request itself says.
//
// Every AI waiting screen names the provider and model the reader is waiting
// on: the Study with AI panel (Explain, Context, Translation and Ask), the
// Search tab's Find, and the key test in Settings. The configuration read when
// a screen is built (activeModelFor) names only the model a request is
// expected to send. The resolver (ai_model_resolve.go) sends the reader's
// override, else a self-healed pick, else the shipped default, and when that
// model is gone it discovers a replacement and sends again. So the request
// says which model it is about to send to, every time it sends, through its
// context: a screen puts an observer on the context it starts the request
// with, and the resolver calls it before the first attempt and again before
// the retry.
//
// A report can reach its screen late. The observer hops to the Fyne goroutine
// (fyne.Do), and in the time that takes the reader can cancel the request, or
// take the faster-model offer and start another; a request abandoned between
// two attempts reports its retry all the same. So a screen names the model
// only while the report is from the request it is waiting on — the Study
// panel by its fetch generation, Find by its session token, the key test by
// its sequence number — and drops any other.

import (
	"context"
	"math"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
)

// aiModelInUse is a provider and the model a request sends to there. The zero
// value means the request has not said yet.
type aiModelInUse struct {
	provider string // provider id (ai_providers.go)
	model    string // the model id, exactly as sent
}

// label is the waiting screens' wording for m: the provider as the Settings
// assistant picker names it ("Claude (Anthropic)"), then the model as the
// Settings model picker lists it, which is by its id. "" before a report.
func (m aiModelInUse) label() string {
	if m.model == "" {
		return ""
	}
	name := m.provider
	if info, ok := providerByID(m.provider); ok {
		name = info.Name
	}
	if name == "" {
		return m.model
	}
	return name + " · " + m.model
}

type aiModelObserverKey struct{}

// withAIModelObserver returns ctx carrying observe. A request run under the
// returned context calls it, on the request's own goroutine, with each model
// it is about to send to.
func withAIModelObserver(ctx context.Context, observe func(aiModelInUse)) context.Context {
	return context.WithValue(ctx, aiModelObserverKey{}, observe)
}

// showAIModelOnUI is withAIModelObserver for a screen: show runs on the Fyne
// goroutine. It runs there for every report, however late, so show must check
// that the screen it paints is still waiting on the request that reported
// (see the file comment).
func showAIModelOnUI(ctx context.Context, show func(aiModelInUse)) context.Context {
	return withAIModelObserver(ctx, func(m aiModelInUse) {
		fyne.Do(func() { show(m) })
	})
}

// reportAIModel tells ctx's observer, if it has one, that the request is about
// to send to model at provider.
func reportAIModel(ctx context.Context, provider, model string) {
	if ctx == nil {
		return
	}
	if observe, ok := ctx.Value(aiModelObserverKey{}).(func(aiModelInUse)); ok && observe != nil {
		observe(aiModelInUse{provider: provider, model: model})
	}
}

// aiModelLineWidth is the measure of the line naming the model: the measure of
// the "Capable models can take a minute or more." hint beside it, so the two
// muted lines centre on one axis, and a long model id wraps inside it rather
// than widening the waiting column.
const aiModelLineWidth = 260

// aiModelLine is the waiting screens' quiet line naming the model at work, in
// the hint's own style (centeredCaption: caption size, muted, centred) and at
// its measure: 260pt wide, and one line as tall as the hint's box makes a line
// (captionHeightFor). Empty, it still holds that line, so the report that
// fills it moves nothing around it; a name too long for the measure wraps onto
// more lines.
type aiModelLine struct {
	text *widget.RichText
	box  *fyne.Container
	// relayout lays out again whatever holds the line, when a report changes
	// its height. The screen that draws the line sets it; nil means nothing.
	relayout func()
}

func newAIModelLine(m aiModelInUse) *aiModelLine {
	rt := centeredCaption(m.label())
	return &aiModelLine{
		text: rt,
		box:  container.New(modelLineLayout{width: aiModelLineWidth}, rt),
	}
}

// show puts m on the line.
func (l *aiModelLine) show(m aiModelInUse) {
	if l == nil || len(l.text.Segments) == 0 {
		return
	}
	seg, ok := l.text.Segments[0].(*widget.TextSegment)
	if !ok || seg.Text == m.label() {
		return
	}
	before := l.box.MinSize().Height
	seg.Text = m.label()
	l.text.Refresh()
	if l.box.MinSize().Height != before && l.relayout != nil {
		l.relayout()
		return
	}
	l.box.Refresh()
}

// String is the line's text, "" before a report.
func (l *aiModelLine) String() string {
	if l == nil {
		return ""
	}
	return l.text.String()
}

// modelLineLayout lays the line's text out at the line's measure and reports
// the height it wraps to there, measured as the hint's box is: one line is
// captionHeightFor(1), and each line a long name wraps onto adds a row of
// caption text. A word-wrapping RichText knows its wrapped height only once it
// has a width (wrappedParagraph), so the text is given the measure before it
// is asked. Its own height is not the one reported: it carries the RichText's
// inner padding above and below the text, which the hint's box leaves out, so
// taking it made even the empty line taller than a line of the hint and
// pushed Cancel further down than the words need. An empty line counts one.
type modelLineLayout struct{ width float32 }

func (l modelLineLayout) MinSize(objs []fyne.CanvasObject) fyne.Size {
	lines := 1
	for _, o := range objs {
		if o.Size().Width != l.width {
			o.Resize(fyne.NewSize(l.width, o.MinSize().Height))
		}
		if n := captionLines(o.MinSize().Height); n > lines {
			lines = n
		}
	}
	return fyne.NewSize(l.width, captionHeightFor(1)+float32(lines-1)*captionRow())
}

// captionRow is the height of one row of caption text, as a RichText stacks
// the rows of a wrapped paragraph: Fyne puts no line spacing between them.
func captionRow() float32 {
	return fyne.MeasureText("M", theme.CaptionTextSize(), fyne.TextStyle{}).Height
}

// captionLines is how many rows of caption text a word-wrapping RichText of
// height h (its MinSize, once it has its width) holds: h less the inner
// padding above and below, in rows.
func captionLines(h float32) int {
	row := captionRow()
	if row <= 0 {
		return 1
	}
	n := int(math.Round(float64((h - 2*theme.InnerPadding()) / row)))
	if n < 1 {
		return 1
	}
	return n
}

func (l modelLineLayout) Layout(objs []fyne.CanvasObject, size fyne.Size) {
	for _, o := range objs {
		o.Resize(fyne.NewSize(l.width, size.Height))
		o.Move(fyne.NewPos((size.Width-l.width)/2, 0))
	}
}
