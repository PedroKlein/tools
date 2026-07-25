package palette

import (
	"fmt"
	"io"
)

// v4 bat emitter — writes a bat tmTheme XML property list.
//
// tmTheme is Apple's XML plist. It does not tolerate # comments, so we
// bypass EmitStandard and emit the plist directly.

type batEmitter struct{}

func (batEmitter) App() string      { return "bat" }
func (batEmitter) Filename() string { return "bat.tmTheme" }

func (batEmitter) Emit(t *Theme, w io.Writer) error {
	s := t.Palette.Semantic
	fmt.Fprintln(w, `<?xml version="1.0" encoding="UTF-8"?>`)
	fmt.Fprintln(w, `<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">`)
	fmt.Fprintln(w, `<plist version="1.0">`)
	fmt.Fprintln(w, `<dict>`)
	fmt.Fprintf(w, "    <key>name</key><string>%s</string>\n", t.Name)
	fmt.Fprintln(w, `    <key>settings</key>`)
	fmt.Fprintln(w, `    <array>`)

	// Global settings.
	fmt.Fprintln(w, `        <dict>`)
	fmt.Fprintln(w, `            <key>settings</key>`)
	fmt.Fprintln(w, `            <dict>`)
	writeSetting(w, "background", s.Bg)
	writeSetting(w, "foreground", s.Fg)
	writeSetting(w, "caret", s.Cursor)
	writeSetting(w, "selection", s.SelectionBg)
	writeSetting(w, "lineHighlight", s.Border)
	fmt.Fprintln(w, `            </dict>`)
	fmt.Fprintln(w, `        </dict>`)

	// Syntax scopes.
	writeScope(w, "Comment", "comment", s.Syntax.Comment, "italic")
	writeScope(w, "Keyword", "keyword, storage.type, storage.modifier", s.Syntax.Keyword, "")
	writeScope(w, "String", "string, string.quoted", s.Syntax.String, "")
	writeScope(w, "Number", "constant.numeric, constant.language", s.Syntax.Number, "")
	writeScope(w, "Function", "entity.name.function, support.function", s.Syntax.Function, "")
	writeScope(w, "Type", "entity.name.type, support.type", s.Syntax.Type, "")
	writeScope(w, "Operator", "keyword.operator", s.Syntax.Operator, "")

	fmt.Fprintln(w, `    </array>`)
	fmt.Fprintln(w, `</dict>`)
	fmt.Fprintln(w, `</plist>`)
	return nil
}

func writeSetting(w io.Writer, key, value string) {
	fmt.Fprintf(w, "                <key>%s</key><string>%s</string>\n", key, value)
}

func writeScope(w io.Writer, name, scope, fg, fontStyle string) {
	fmt.Fprintln(w, `        <dict>`)
	fmt.Fprintf(w, "            <key>name</key><string>%s</string>\n", name)
	fmt.Fprintf(w, "            <key>scope</key><string>%s</string>\n", scope)
	fmt.Fprintln(w, `            <key>settings</key>`)
	fmt.Fprintln(w, `            <dict>`)
	fmt.Fprintf(w, "                <key>foreground</key><string>%s</string>\n", fg)
	if fontStyle != "" {
		fmt.Fprintf(w, "                <key>fontStyle</key><string>%s</string>\n", fontStyle)
	}
	fmt.Fprintln(w, `            </dict>`)
	fmt.Fprintln(w, `        </dict>`)
}
