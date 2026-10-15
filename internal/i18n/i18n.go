// Package i18n translates the messages Pasla shows to people.
//
// The English text is the message key (gettext style), so the source reads
// naturally and a missing translation simply falls back to English. Formats
// use only %s and %v so that arguments survive a trip between devices as
// plain strings.
package i18n

import (
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
)

// Languages the app ships with. English is the default.
const (
	EN = "en"
	TR = "tr"
)

var catalogs = map[string]map[string]string{TR: tr}

var defaultLang atomic.Value

func init() { defaultLang.Store(EN) }

// Norm maps any input ("tr-TR", "TR", "") to a supported language code.
func Norm(lang string) string {
	l := strings.ToLower(strings.TrimSpace(lang))
	if strings.HasPrefix(l, "tr") {
		return TR
	}
	return EN
}

// SetDefault sets the language used for log lines and the command line.
func SetDefault(lang string) { defaultLang.Store(Norm(lang)) }

// Default returns the process-wide language.
func Default() string { return defaultLang.Load().(string) }

// Msg is a translatable message. It is also an error whose Error() is the
// English text, which keeps logs and the device-to-device protocol stable.
type Msg struct {
	Format string
	Args   []any
}

// E builds a translatable message.
func E(format string, args ...any) *Msg { return &Msg{Format: format, Args: args} }

func (m *Msg) Error() string { return m.In(EN) }

// In renders the message in lang. Arguments that are errors are translated too.
func (m *Msg) In(lang string) string {
	f := m.Format
	if c, ok := catalogs[Norm(lang)]; ok {
		if t, ok := c[m.Format]; ok {
			f = t
		}
	}
	args := make([]any, len(m.Args))
	for i, a := range m.Args {
		if err, ok := a.(error); ok {
			args[i] = Text(lang, err)
		} else {
			args[i] = a
		}
	}
	if len(args) == 0 {
		return f
	}
	return fmt.Sprintf(f, args...)
}

// StringArgs flattens the arguments for the wire (errors become English).
func (m *Msg) StringArgs() []string {
	out := make([]string, len(m.Args))
	for i, a := range m.Args {
		out[i] = fmt.Sprint(a)
	}
	return out
}

// FromWire rebuilds a message received from another device.
func FromWire(format string, args []string) *Msg {
	a := make([]any, len(args))
	for i, s := range args {
		a[i] = s
	}
	return &Msg{Format: format, Args: a}
}

// Text renders any error in lang: translatable messages are translated,
// everything else (operating system errors) is passed through.
func Text(lang string, err error) string {
	if err == nil {
		return ""
	}
	var m *Msg
	if errors.As(err, &m) {
		return m.In(lang)
	}
	return err.Error()
}

// T translates a plain string with optional arguments.
func T(lang, format string, args ...any) string { return E(format, args...).In(lang) }

// Has reports whether a translation exists (used by the catalogue test).
func Has(lang, format string) bool {
	_, ok := catalogs[Norm(lang)][format]
	return ok
}
