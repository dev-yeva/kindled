// Writing custom handlers guide:
// https://github.com/golang/example/blob/master/slog-handler-guide/README.md

package logger

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"

	"github.com/fatih/color"
)

var (
	timeColor = color.New(color.FgHiGreen).SprintFunc()
	msgColor  = color.New(color.FgHiBlue).SprintFunc()
	keyColor  = color.New(color.FgHiBlack).SprintFunc()
	valColor  = color.New(color.FgHiMagenta).SprintFunc()

	lineBreakReplacer = strings.NewReplacer(
		"\r\n", " ",
		"\n", " ",
		"\r", " ",
		"\u2028", " ",
		"\u2029", " ",
	)
)

type UserHandler struct {
	opts           *Options
	mu             *sync.Mutex
	out            io.Writer
	progressExists bool // true while the last thing written was a progress line
}

type Options struct {
	Level      slog.Leveler
	TimeFormat string
}

func NewUserHandler(out io.Writer, opts *Options) *UserHandler {
	h := &UserHandler{out: out, mu: &sync.Mutex{}, progressExists: false}
	if opts == nil {
		h.opts = &Options{Level: slog.LevelInfo}
	} else {
		h.opts = opts
	}
	return h
}

func (h *UserHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return level >= h.opts.Level.Level()
}

func (h *UserHandler) Handle(ctx context.Context, r slog.Record) error {
	// message starting with "\r" is a progress line that rewrites itself.
	// "\r" must come before timestamp.
	msg := r.Message
	isProgress := strings.HasPrefix(msg, "\r")
	msg = strings.TrimPrefix(msg, "\r")
	msg = strings.TrimSuffix(msg, "\n")

	buf := make([]byte, 0, 1024)
	buf = h.appendAttr(buf, slog.Time(slog.TimeKey, r.Time), false)
	buf = h.appendAttr(buf, slog.String(slog.MessageKey, msg), false)

	r.Attrs(func(a slog.Attr) bool {
		buf = h.appendAttr(buf, a, true)
		return true
	})
	buf = bytes.TrimRight(buf, " ")

	if !isProgress {
		buf = append(buf, '\n')
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	// Return to line start and erase leftovers of the previous progress line
	// (a shorter new line would otherwise keep the old tail).
	if isProgress || h.progressExists {
		buf = append([]byte("\r\x1b[K"), buf...)
	}
	h.progressExists = true

	_, err := h.out.Write(buf)
	return err
}

func (h *UserHandler) appendAttr(buf []byte, a slog.Attr, printKey bool) []byte {
	a.Value = a.Value.Resolve()

	switch a.Value.Kind() {
	case slog.KindTime:
		buf = fmt.Appendf(buf, "%s ", timeColor(a.Value.Time().Format(h.opts.TimeFormat)))
	case slog.KindGroup:
		attrs := a.Value.Group()
		for _, a := range attrs {
			buf = h.appendAttr(buf, a, true)
		}
	default:
		if a.Value.String() != "" {
			if printKey {
				cleanVal := lineBreakReplacer.Replace(a.Value.String())
				buf = fmt.Appendf(buf, "%s%s ", keyColor(a.Key+"="), valColor(cleanVal))
				//buf = fmt.Appendf(buf, "%s%s ", keyColor(a.Key+"="), valColor(strings.ReplaceAll(a.Value.String(), "\n", " ")))
			} else {
				buf = fmt.Appendf(buf, "%s ", msgColor(a.Value))
			}
		}
	}

	return buf
}

func (h *UserHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return h
}

func (h *UserHandler) WithGroup(name string) slog.Handler {
	return h
}
