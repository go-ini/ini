package ini

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type tokenKind uint8

const (
	tokEOF         tokenKind = iota
	tokSection               // [name] ; comment
	tokKey                   // name = value ; comment
	tokBooleanKey            // name
	tokNestedValue           // indented line under a key with an empty value
	tokRawBody               // body of an unparseable section
)

type token struct {
	kind         tokenKind
	name         string
	value        string
	comment      string
	line, column int // where the token starts, both from 1
}

// errorf reports err at the position of the token.
func (t token) errorf(err error) error {
	return &ParseError{Line: t.line, Column: t.column, Err: err}
}

// lexer turns INI input into tokens. Its rules, and the order in which they
// apply, are those of gopkg.in/ini.v1, so every input loads as it did there.
type lexer struct {
	src  string
	pos  int  // offset of the next unread byte
	eof  bool // the last line read had no '\n'
	opts *loadOptions
	line int // number of the last line read, from 1

	comment        []string // comment pieces for the next section or key
	lastValueEmpty bool     // the last key had an empty value, so indented lines are nested values
	inRaw          bool     // inside an unparseable section
	raw            strings.Builder
}

// next returns the next token, or tokEOF at the end of input.
func (l *lexer) next() (token, error) {
	for !l.eof {
		raw := l.readLine()
		lineNum := l.line
		column := func(s string) int { return utf8.RuneCountInString(raw[:len(raw)-len(s)]) + 1 }
		if l.opts.AllowNestedValues && l.lastValueEmpty && raw != "" && (raw[0] == ' ' || raw[0] == '\t') {
			value := strings.TrimSpace(raw)
			return token{kind: tokNestedValue, value: value, line: lineNum, column: column(strings.TrimLeftFunc(raw, unicode.IsSpace))}, nil
		}

		line := strings.TrimLeftFunc(raw, unicode.IsSpace)
		var tok token
		var err error
		switch {
		case line == "":
			continue
		case l.startsWithCommentSymbol(line):
			// Keep the line break so comment lines stay on separate lines.
			l.comment = append(l.comment, line)
			continue
		case line[0] == '[':
			if l.raw.Len() > 0 {
				l.unread(raw)
				return l.rawBody(), nil
			}
			tok, err = l.section(line)
		case l.inRaw:
			l.raw.WriteString(line)
			continue
		default:
			var ok bool
			if tok, ok, err = l.key(line); !ok && err == nil {
				continue
			}
		}
		if err != nil {
			return token{}, &ParseError{Line: lineNum, Column: column(line), Err: err}
		}
		tok.line, tok.column = lineNum, column(line)
		return tok, nil
	}
	if l.raw.Len() > 0 {
		return l.rawBody(), nil
	}
	return token{kind: tokEOF}, nil
}

// readLine returns the next line including its '\n', if any.
func (l *lexer) readLine() string {
	rest := l.src[l.pos:]
	i := strings.IndexByte(rest, '\n')
	if i < 0 {
		l.pos, l.eof = len(l.src), true
		l.line++
		return rest
	}
	l.pos += i + 1
	l.line++
	return rest[:i+1]
}

// unread puts back the line just read.
func (l *lexer) unread(line string) {
	l.pos -= len(line)
	l.eof = false
	l.line--
}

func (l *lexer) startsWithCommentSymbol(line string) bool {
	return startsWithAny(line, l.opts.CommentSymbols)
}

// startsWithAny reports whether the first character of s is one of chars.
func startsWithAny(s, chars string) bool {
	r, _ := utf8.DecodeRuneInString(s)
	return r != utf8.RuneError && strings.ContainsRune(chars, r)
}

func (l *lexer) takeComment() string {
	c := strings.TrimSpace(strings.Join(l.comment, ""))
	l.comment = l.comment[:0]
	return c
}

func (l *lexer) rawBody() token {
	t := token{kind: tokRawBody, value: l.raw.String()}
	l.raw.Reset()
	return t
}

// section reads a "[name] ; comment" line. The name ends at the last ']'.
func (l *lexer) section(line string) (token, error) {
	closeIdx := strings.LastIndexByte(line, ']')
	if closeIdx == -1 {
		return token{}, fmt.Errorf("unclosed section: %s", line)
	}
	name := line[1:closeIdx]
	if i := strings.IndexAny(line[closeIdx+1:], l.opts.CommentSymbols); i > -1 {
		l.comment = append(l.comment, line[closeIdx+1+i:])
	}

	l.lastValueEmpty = false
	l.inRaw = false
	for _, s := range l.opts.UnparseableSections {
		if s == name || ((l.opts.Insensitive || l.opts.InsensitiveSections) && strings.EqualFold(s, name)) {
			l.inRaw = true
			break
		}
	}
	return token{kind: tokSection, name: name, comment: l.takeComment()}, nil
}

// key reads a "name = value" line, or a boolean key when the line has no
// delimiter. ok is false for a line that SkipUnrecognizableLines skips.
func (l *lexer) key(line string) (_ token, ok bool, _ error) {
	name, offset, err := readKeyName(l.opts.KeyValueDelimiters, line)
	switch {
	case err == nil:
		value, err := l.readValue(line[offset:])
		if err != nil {
			return token{}, false, err
		}
		l.lastValueEmpty = value == ""
		return token{kind: tokKey, name: name, value: value, comment: l.takeComment()}, true, nil

	case errors.Is(err, ErrDelimiterNotFound) && l.opts.AllowBooleanKeys:
		// The whole line is the key name, read like a value.
		name, err := l.readValue(line)
		if err != nil {
			return token{}, false, err
		}
		return token{kind: tokBooleanKey, name: name, comment: l.takeComment()}, true, nil

	case (errors.Is(err, ErrDelimiterNotFound) || errors.Is(err, ErrEmptyKeyName)) && l.opts.SkipUnrecognizableLines:
		return token{}, false, nil
	}
	return token{}, false, err
}

// readValue reads a value that starts in "in", the rest of the current line,
// and any following lines it spans. The rules apply in a fixed order, which
// decides the result when they overlap: a value is either quoted, continued
// with a backslash, or plain. For a plain value the inline comment is cut
// before surrounding quotes are stripped, so `"a;b"` is `"a` with comment
// `;b"`.
func (l *lexer) readValue(in string) (string, error) {
	line := strings.TrimLeftFunc(in, unicode.IsSpace)
	if line == "" {
		if l.opts.AllowPythonMultilineValues && strings.HasSuffix(in, "\n") {
			return l.readPythonMultilines(""), nil
		}
		return "", nil
	}

	if q := l.valueQuote(line); q != "" {
		return l.readQuoted(line, q)
	}

	endsLine := line[len(line)-1] == '\n'
	line = strings.TrimSpace(line)
	if !l.opts.IgnoreContinuation && strings.HasSuffix(line, `\`) {
		return l.readContinuationLines(line[:len(line)-1]), nil
	}
	if !l.opts.IgnoreInlineComment {
		line = l.cutInlineComment(line)
	}

	switch {
	case !l.opts.PreserveSurroundingQuotes && (hasSurroundedQuote(line, '\'') || hasSurroundedQuote(line, '"')):
		return line[1 : len(line)-1], nil
	case l.opts.UnescapeValueCommentSymbols:
		// Last symbol first, so \; is unescaped before \#.
		symbols := []rune(l.opts.CommentSymbols)
		for i := len(symbols) - 1; i >= 0; i-- {
			c := string(symbols[i])
			line = strings.ReplaceAll(line, `\`+c, c)
		}
		return line, nil
	case l.opts.AllowPythonMultilineValues && endsLine:
		return l.readPythonMultilines(line), nil
	}
	return line, nil
}

// valueQuote returns the quote that opens line as a quoted value: `"""`, "`",
// or `"` with UnescapeValueDoubleQuotes. Single quotes and plain double
// quotes are handled later, as surrounding quotes.
func (l *lexer) valueQuote(line string) string {
	switch {
	case len(line) > 3 && line[:3] == `"""`:
		return `"""`
	case line[0] == '`':
		return "`"
	case l.opts.UnescapeValueDoubleQuotes && line[0] == '"':
		return `"`
	}
	return ""
}

// readQuoted reads a value that starts with quote q and ends at the last q on
// the same line or, failing that, the first later line containing q.
func (l *lexer) readQuoted(line, q string) (string, error) {
	rest := line[len(q):]
	if i := strings.LastIndex(rest, q); i > -1 {
		if q == `"` {
			return strings.ReplaceAll(rest[:i], `\"`, `"`), nil
		}
		return rest[:i], nil
	}

	var b strings.Builder
	b.WriteString(rest)
	for {
		next := l.readLine()
		i := strings.LastIndex(next, q)
		switch {
		case i == -1:
			b.WriteString(next)
			if l.eof {
				return "", fmt.Errorf("missing closing key quote from %q to %q", line, next)
			}
		case !l.opts.IgnoreContinuation && strings.HasSuffix(strings.TrimSpace(next[i+len(q):]), `\`):
			// A backslash after the closing quote continues the value.
			b.WriteString(next)
		default:
			b.WriteString(next[:i])
			if j := strings.IndexAny(next[i:], l.opts.CommentSymbols); j > -1 {
				l.comment = append(l.comment, strings.TrimSpace(next[i+j:]))
			}
			return b.String(), nil
		}
	}
}

// cutInlineComment moves a trailing "# ..." or "; ..." of a plain value to
// the comment. With SpaceBeforeInlineComment the symbol must follow a space,
// and symbols are tried in order, so " #" wins over an earlier " ;".
func (l *lexer) cutInlineComment(line string) string {
	i := -1
	if l.opts.SpaceBeforeInlineComment {
		for _, c := range l.opts.CommentSymbols {
			if i = strings.Index(line, " "+string(c)); i > -1 {
				break
			}
		}
	} else {
		i = strings.IndexAny(line, l.opts.CommentSymbols)
	}
	if i == -1 {
		return line
	}
	l.comment = append(l.comment, line[i:])
	return strings.TrimSpace(line[:i])
}

// readContinuationLines appends trimmed following lines while they end with
// a backslash, up to the first line that doesn't or a blank line. val is the
// first line without its backslash.
func (l *lexer) readContinuationLines(val string) string {
	var b strings.Builder
	b.WriteString(val)
	for {
		next := strings.TrimSpace(l.readLine())
		if next == "" {
			return b.String()
		}
		if !strings.HasSuffix(next, `\`) {
			b.WriteString(next)
			return b.String()
		}
		b.WriteString(next[:len(next)-1])
	}
}

// readPythonMultilines appends the following lines that start with a space,
// tab or form feed, each on a new line with its indentation kept.
func (l *lexer) readPythonMultilines(line string) string {
	var b strings.Builder
	b.WriteString(line)
	for l.pos < len(l.src) && strings.IndexByte(" \t\f", l.src[l.pos]) > -1 {
		next := strings.TrimSuffix(l.readLine(), "\n")
		b.WriteByte('\n')
		b.WriteString(next)
	}
	return b.String()
}

// skipBOM strips a UTF-8, UTF-16 LE or UTF-16 BE byte order mark.
// http://en.wikipedia.org/wiki/Byte_order_mark#Representations_of_byte_order_marks_by_encoding
func skipBOM(src string) string {
	switch {
	case len(src) >= 2 && (src[0] == 0xFE && src[1] == 0xFF || src[0] == 0xFF && src[1] == 0xFE):
		return src[2:]
	case len(src) >= 3 && src[0] == 0xEF && src[1] == 0xBB && src[2] == 0xBF:
		return src[3:]
	}
	return src
}

// parse reads all of reader and adds its sections and keys to f. The caller
// holds the lock, or f isn't shared yet.
func (f *File) parse(reader io.Reader) error {
	var b strings.Builder
	if _, err := io.Copy(&b, reader); err != nil {
		return err
	}
	l := &lexer{src: skipBOM(b.String()), opts: &f.options}

	// Ignore error because default section name is never empty string.
	name := f.options.DefaultSection
	if f.options.Insensitive || f.options.InsensitiveSections {
		name = strings.ToLower(name)
	}
	section, _ := f.addSection(name)

	var lastRegularKey *Key
	count := 1 // Counter for auto-increment key names ("-"), per section.
	for {
		tok, err := l.next()
		if err != nil {
			return err
		}

		switch tok.kind {
		case tokEOF:
			return nil

		case tokSection:
			if section, err = f.addSection(tok.name); err != nil {
				return tok.errorf(err)
			}
			section.comment = tok.comment
			count = 1

		case tokRawBody:
			section.isRawSection = true
			section.rawBody += tok.value

		case tokNestedValue:
			if err = lastRegularKey.addNestedValue(tok.value); err != nil {
				return tok.errorf(err)
			}

		case tokBooleanKey:
			key, err := section.addBooleanKey(tok.name)
			if err != nil {
				return tok.errorf(err)
			}
			key.comment = tok.comment

		case tokKey:
			name, isAutoIncr := tok.name, tok.name == "-"
			if isAutoIncr {
				name = "#" + strconv.Itoa(count)
				count++
			}
			key, err := section.addKey(name, tok.value)
			if err != nil {
				return tok.errorf(err)
			}
			key.isAutoIncrement = isAutoIncr
			key.comment = tok.comment
			lastRegularKey = key
		}
	}
}

// readKeyName returns the key name at the start of line and the offset of
// the value after its delimiter.
func readKeyName(delimiters string, line string) (string, int, error) {
	// Check if key name surrounded by quotes.
	var keyQuote string
	switch line[0] {
	case '"':
		if len(line) > 6 && line[0:3] == `"""` {
			keyQuote = `"""`
		} else {
			keyQuote = `"`
		}
	case '`':
		keyQuote = "`"
	}

	// Read the key name.
	var endIdx int
	if len(keyQuote) > 0 {
		startIdx := len(keyQuote)
		// The name ends at the first closing quote, so """"""name"""=value has
		// an empty name and fails, as it did in v1.
		pos := strings.Index(line[startIdx:], keyQuote)
		if pos == -1 {
			return "", -1, fmt.Errorf("missing closing key quote: %s", line)
		}
		pos += startIdx

		// Find key-value delimiter
		i := strings.IndexAny(line[pos+startIdx:], delimiters)
		if i < 0 {
			return "", -1, fmt.Errorf("%w: %s", ErrDelimiterNotFound, line)
		}
		endIdx = pos + i
		return strings.TrimSpace(line[startIdx:pos]), endIdx + startIdx + 1, nil
	}

	endIdx = strings.IndexAny(line, delimiters)
	if endIdx < 0 {
		return "", -1, fmt.Errorf("%w: %s", ErrDelimiterNotFound, line)
	}
	if endIdx == 0 {
		return "", -1, fmt.Errorf("%w: %s", ErrEmptyKeyName, line)
	}

	return strings.TrimSpace(line[0:endIdx]), endIdx + 1, nil
}

// hasSurroundedQuote reports whether in starts and ends with quote and has no
// other quote of the same kind.
func hasSurroundedQuote(in string, quote byte) bool {
	return len(in) >= 2 && in[0] == quote && in[len(in)-1] == quote &&
		strings.IndexByte(in[1:], quote) == len(in)-2
}
