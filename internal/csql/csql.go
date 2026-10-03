// Package csql is the Campfire Simple Query Language: the one way a filter is
// written down, by the filter bar, by a saved view or by an agent (SPEC.md
// §9.2).
//
//	assignee in (marta, me) and (bucket = DEV-6 or label = roses) and not priority = 4
//
// A query is conditions joined by and, or and not, with parentheses. A
// condition is a field, an operator and a value; the fields and what they take
// are in SPEC.md §9.2. != is exactly not =, none is the value of an empty
// field, and #12 names a thing by its id, which is how a saved view stores it.
//
// The package parses, prints and compiles to SQL. The client has a parser of
// its own for suggestions and instant errors (internal/web/src/lib/csql.js);
// testdata/corpus.json holds what both must say, message for message.
// Turning names into ids needs the database, so that is behind Resolver.
package csql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Field is what a field takes.
type Field struct {
	Ops      []string // in the order they are offered
	Nullable bool     // takes none
	Named    bool     // its values name things the resolver looks up
}

var (
	valueOps = []string{"=", "!=", "in", "not in"}
	dayOps   = []string{"=", "!=", "<", "<=", ">", ">="}
)

// Fields, in the order they are offered.
var Fields = []string{"assignee", "reporter", "label", "bucket", "priority", "state", "team",
	"kind", "created", "started", "completed", "due", "text"}

var fields = map[string]Field{
	"assignee":  {Ops: valueOps, Nullable: true, Named: true},
	"reporter":  {Ops: valueOps, Named: true},
	"label":     {Ops: valueOps, Nullable: true, Named: true},
	"bucket":    {Ops: valueOps, Nullable: true, Named: true},
	"priority":  {Ops: append(append([]string{}, valueOps...), "<", "<=", ">", ">=")},
	"state":     {Ops: valueOps},
	"team":      {Ops: valueOps, Named: true},
	"kind":      {Ops: valueOps},
	"created":   {Ops: dayOps},
	"started":   {Ops: dayOps, Nullable: true},
	"completed": {Ops: dayOps, Nullable: true},
	"due":       {Ops: dayOps, Nullable: true},
	"text":      {Ops: []string{"~", "!~"}},
}

func isDay(field string) bool {
	return field == "created" || field == "started" || field == "completed" || field == "due"
}

// Node is And, Or, Not or Cond.
type Node interface{ node() }

type And struct{ Of []Node }
type Or struct{ Of []Node }
type Not struct{ X Node }

// Cond is one field compared with values.
type Cond struct {
	Field  string
	Op     string // = != < <= > >= ~ !~ in "not in"
	Values []*Value
	Pos    int
}

func (And) node()   {}
func (Or) node()    {}
func (Not) node()   {}
func (*Cond) node() {}

// Value is one value as written, normalised, and what it resolved to.
type Value struct {
	Raw    string // keywords lower-cased, priorities as their number
	Quoted bool
	Pos    int
	ID     int64 // the referent once resolved; a priority's number; an #id as written
	Ref    bool  // written as #id
	Me     bool  // the reader, whoever that is: kept as me, never an id
	None   bool  // the empty value
	// A day, once resolved in the reader's zone: its date, and its first and
	// last-plus-one microsecond.
	Day      string
	From, To int64
}

// Error is a query that does not parse or names something that is not there.
// Pos and End are byte offsets into the query, for underlining.
type Error struct {
	Msg      string
	Pos, End int
}

func (e *Error) Error() string { return e.Msg }

func errAt(pos, end int, format string, args ...any) *Error {
	return &Error{Msg: fmt.Sprintf(format, args...), Pos: pos, End: end}
}

// Lexing --------------------------------------------------------------------

type tok struct {
	kind byte // w word, s string, # ref, p punctuation or operator, 0 end
	text string
	pos  int
	end  int
}

func wordByte(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' ||
		c == '_' || c == '-' || c == '+' || c == '.' || c == '@' || c >= 0x80
}

func lex(src string) ([]tok, error) {
	var out []tok
	for i := 0; i < len(src); {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r':
			i++
		case c == '"' || c == '\'':
			var b strings.Builder
			j := i + 1
			for ; j < len(src) && src[j] != c; j++ {
				if src[j] == '\\' && j+1 < len(src) {
					j++
				}
				b.WriteByte(src[j])
			}
			if j >= len(src) {
				return nil, errAt(i, len(src), "this quote is never closed")
			}
			out = append(out, tok{kind: 's', text: b.String(), pos: i, end: j + 1})
			i = j + 1
		case c == '#':
			j := i + 1
			for j < len(src) && src[j] >= '0' && src[j] <= '9' {
				j++
			}
			if j == i+1 {
				return nil, errAt(i, i+1, "# is followed by an id, such as #12")
			}
			out = append(out, tok{kind: '#', text: src[i+1 : j], pos: i, end: j})
			i = j
		case c == '(' || c == ')' || c == ',' || c == '=' || c == '~':
			out = append(out, tok{kind: 'p', text: string(c), pos: i, end: i + 1})
			i++
		case c == '!' || c == '<' || c == '>':
			j := i + 1
			if j < len(src) && (src[j] == '=' || c == '!' && src[j] == '~') {
				j++
			}
			if c == '!' && j == i+1 {
				return nil, errAt(i, j, "! is only part of != and !~; use not")
			}
			out = append(out, tok{kind: 'p', text: src[i:j], pos: i, end: j})
			i = j
		case wordByte(c):
			j := i
			for j < len(src) && wordByte(src[j]) {
				j++
			}
			out = append(out, tok{kind: 'w', text: src[i:j], pos: i, end: j})
			i = j
		default:
			return nil, errAt(i, i+1, "%s has no meaning here", strconv.Quote(string(c)))
		}
	}
	return append(out, tok{pos: len(src), end: len(src)}), nil
}

// Parsing -------------------------------------------------------------------

type parser struct {
	toks []tok
	i    int
}

func (p *parser) peek() tok { return p.toks[p.i] }
func (p *parser) next() tok { t := p.toks[p.i]; p.i++; return t }

func (p *parser) kw(word string) bool {
	t := p.peek()
	return t.kind == 'w' && strings.EqualFold(t.text, word)
}

func (p *parser) punct(s string) bool {
	t := p.peek()
	return t.kind == 'p' && t.text == s
}

func describe(t tok) string {
	if t.kind == 0 {
		return "the end"
	}
	return strconv.Quote(t.text)
}

// Parse reads a query, checking every value that needs no database. An empty
// query is nil: no condition at all.
func Parse(src string) (Node, error) {
	toks, err := lex(src)
	if err != nil {
		return nil, err
	}
	p := &parser{toks: toks}
	if p.peek().kind == 0 {
		return nil, nil
	}
	n, err := p.or()
	if err != nil {
		return nil, err
	}
	if t := p.peek(); t.kind != 0 {
		if t.kind == 'p' && t.text == ")" {
			return nil, errAt(t.pos, t.end, "this ) closes nothing")
		}
		return nil, errAt(t.pos, t.end, "expected and or or before %s", describe(t))
	}
	return n, nil
}

func (p *parser) or() (Node, error) {
	var of []Node
	for {
		n, err := p.and()
		if err != nil {
			return nil, err
		}
		of = append(of, n)
		if !p.kw("or") {
			break
		}
		p.next()
	}
	if len(of) == 1 {
		return of[0], nil
	}
	return Or{of}, nil
}

func (p *parser) and() (Node, error) {
	var of []Node
	for {
		n, err := p.not()
		if err != nil {
			return nil, err
		}
		of = append(of, n)
		if !p.kw("and") {
			break
		}
		p.next()
	}
	if len(of) == 1 {
		return of[0], nil
	}
	return And{of}, nil
}

func (p *parser) not() (Node, error) {
	if p.kw("not") {
		p.next()
		n, err := p.not()
		if err != nil {
			return nil, err
		}
		return Not{n}, nil
	}
	if p.punct("(") {
		open := p.next()
		n, err := p.or()
		if err != nil {
			return nil, err
		}
		if !p.punct(")") {
			t := p.peek()
			if t.kind == 0 {
				return nil, errAt(open.pos, open.end, "this ( is never closed")
			}
			return nil, errAt(t.pos, t.end, "expected ) before %s", describe(t))
		}
		p.next()
		return n, nil
	}
	return p.cond()
}

func (p *parser) cond() (Node, error) {
	t := p.next()
	name := strings.ToLower(t.text)
	f, ok := fields[name]
	if t.kind != 'w' || !ok {
		return nil, errAt(t.pos, t.end, "expected a field (%s), not %s", strings.Join(Fields, ", "), describe(t))
	}
	c := &Cond{Field: name, Pos: t.pos}
	op := p.next()
	c.Op = op.text
	if op.kind == 'w' {
		c.Op = strings.ToLower(op.text)
		if c.Op == "not" && p.kw("in") {
			p.next()
			c.Op = "not in"
		}
	}
	allowed := false
	for _, o := range f.Ops {
		allowed = allowed || o == c.Op && (op.kind == 'p') == (o != "in" && o != "not in")
	}
	if !allowed {
		return nil, errAt(op.pos, op.end, "expected an operator after %s (%s), not %s",
			name, strings.Join(f.Ops, ", "), describe(op))
	}
	if c.Op != "in" && c.Op != "not in" {
		v, err := p.value(name)
		if err != nil {
			return nil, err
		}
		if v.None && c.Op != "=" && c.Op != "!=" {
			return nil, errAt(v.Pos, v.Pos+4, "none only takes = or !=")
		}
		c.Values = []*Value{v}
		return c, nil
	}
	if !p.punct("(") {
		t := p.peek()
		return nil, errAt(t.pos, t.end, "expected ( after %s, as in (marta, sally)", c.Op)
	}
	p.next()
	for {
		v, err := p.value(name)
		if err != nil {
			return nil, err
		}
		c.Values = append(c.Values, v)
		if p.punct(",") {
			p.next()
			continue
		}
		if p.punct(")") {
			p.next()
			return c, nil
		}
		t := p.peek()
		return nil, errAt(t.pos, t.end, "expected , or ) in the list, not %s", describe(t))
	}
}

var relative = regexp.MustCompile(`^[+-]\d{1,4}[dw]$`)

func (p *parser) value(field string) (*Value, error) {
	t := p.next()
	v := &Value{Raw: t.text, Pos: t.pos}
	switch t.kind {
	case 's':
		v.Quoted = true
	case '#':
		if !fields[field].Named && field != "state" {
			return nil, errAt(t.pos, t.end, "%s does not take an #id", field)
		}
		v.ID, _ = strconv.ParseInt(t.text, 10, 64)
		v.Raw, v.Ref = "#"+t.text, true
		return v, nil
	case 'w':
		low := strings.ToLower(t.text)
		switch low {
		case "and", "or", "not", "in":
			return nil, errAt(t.pos, t.end, "expected a value for %s, not the keyword %s", field, low)
		case "none":
			if !fields[field].Nullable {
				return nil, errAt(t.pos, t.end, "%s is never none", field)
			}
			v.Raw, v.None = low, true
			return v, nil
		case "me", "today":
			v.Raw = low
		}
	default:
		return nil, errAt(t.pos, t.end, "expected a value for %s, not %s", field, describe(t))
	}
	switch {
	case field == "priority":
		n, err := strconv.Atoi(strings.TrimPrefix(strings.ToLower(v.Raw), "p"))
		if err != nil || n < 1 || n > 4 {
			return nil, errAt(t.pos, t.end, "priority is 1 to 4, or P1 to P4")
		}
		v.Raw, v.ID = strconv.Itoa(n), int64(n)
	case field == "kind":
		v.Raw = strings.ToLower(v.Raw)
		if v.Raw != "ticket" && v.Raw != "bucket" {
			return nil, errAt(t.pos, t.end, "kind is ticket or bucket")
		}
	case isDay(field):
		v.Raw = strings.ToLower(v.Raw)
		if _, err := time.Parse("2006-01-02", v.Raw); err != nil && v.Raw != "today" && !relative.MatchString(v.Raw) {
			return nil, errAt(t.pos, t.end, "%s takes a day: 2026-09-01, today, -7d or +2w", field)
		}
	}
	return v, nil
}

// Resolving -----------------------------------------------------------------

// Resolver turns a name into the id of what it names, or an error saying why
// it cannot, and says what today is for the reader.
type Resolver interface {
	Resolve(field, name string) (int64, error)
	Me() int64
	StateExists(name string) bool
	// Now is the reading moment in the reader's zone.
	Now() time.Time
}

// Resolve gives every value what it means for this reader, and returns the
// first value that names nothing as an Error pointing at it.
func Resolve(n Node, r Resolver) error {
	return walk(n, func(c *Cond) error {
		for _, v := range c.Values {
			end := v.Pos + len(v.Raw)
			if v.Quoted {
				end += 2
			}
			switch {
			case v.Ref || v.None || c.Field == "text" || c.Field == "priority" || c.Field == "kind":
			case isDay(c.Field):
				day := resolveDay(v.Raw, r.Now())
				v.Day = day.Format("2006-01-02")
				v.From, v.To = day.UnixMicro(), day.AddDate(0, 0, 1).UnixMicro()
			case c.Field == "state":
				if !r.StateExists(v.Raw) {
					return errAt(v.Pos, end, "no work state is called %q", v.Raw)
				}
			case v.Raw == "me" && !v.Quoted && (c.Field == "assignee" || c.Field == "reporter"):
				v.Me, v.ID = true, r.Me()
			default:
				id, err := r.Resolve(c.Field, v.Raw)
				if err != nil {
					return errAt(v.Pos, end, "%s", err.Error())
				}
				v.ID = id
			}
		}
		return nil
	})
}

// resolveDay is the start of the day a value names, in now's zone.
func resolveDay(raw string, now time.Time) time.Time {
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	switch {
	case raw == "today":
		return today
	case relative.MatchString(raw):
		n, _ := strconv.Atoi(raw[1 : len(raw)-1])
		if raw[0] == '-' {
			n = -n
		}
		if raw[len(raw)-1] == 'w' {
			n *= 7
		}
		return today.AddDate(0, 0, n)
	}
	d, _ := time.ParseInLocation("2006-01-02", raw, now.Location())
	return d
}

func walk(n Node, fn func(*Cond) error) error {
	switch n := n.(type) {
	case And:
		for _, x := range n.Of {
			if err := walk(x, fn); err != nil {
				return err
			}
		}
	case Or:
		for _, x := range n.Of {
			if err := walk(x, fn); err != nil {
				return err
			}
		}
	case Not:
		return walk(n.X, fn)
	case *Cond:
		return fn(n)
	}
	return nil
}

// Ref is one thing a query names by id.
type Ref struct {
	Field string
	ID    int64
}

// Refs lists what a resolved query names by id, which is what a saved view
// keeps beside its text so a referenced label is never reaped and a deleted
// referent can be named.
func Refs(n Node) []Ref {
	var out []Ref
	seen := map[Ref]bool{}
	walk(n, func(c *Cond) error {
		for _, v := range c.Values {
			if v.Me || v.None || v.ID == 0 || c.Field == "priority" || c.Field == "state" && !v.Ref {
				continue
			}
			r := Ref{c.Field, v.ID}
			if !seen[r] {
				seen[r] = true
				out = append(out, r)
			}
		}
		return nil
	})
	return out
}

// Printing ------------------------------------------------------------------

// Canonical prints a resolved query as a saved view stores it: referents as
// #ids, so a rename changes nothing; me, none and days as written.
func Canonical(n Node) string {
	return render(n, func(field string, v *Value) string {
		if fields[field].Named && v.ID != 0 && !v.Me && !v.None {
			return "#" + strconv.FormatInt(v.ID, 10)
		}
		return printed(v)
	})
}

// Format prints a query for people, naming each #id by name(field, id). A
// name it cannot give stays an #id. With a nil name it is the query as
// written, normalised: the form both parsers' corpus compares.
func Format(n Node, name func(field string, id int64) (string, bool)) string {
	return render(n, func(field string, v *Value) string {
		if v.Ref && name != nil {
			if s, ok := name(field, v.ID); ok {
				return Quote(s)
			}
		}
		return printed(v)
	})
}

func printed(v *Value) string {
	if v.Ref || v.None || !v.Quoted && (v.Raw == "me" || v.Raw == "today") {
		return v.Raw
	}
	return Quote(v.Raw)
}

// Reserved are the words a query gives a meaning of its own. No username,
// label, team or work state may be one, so a filter never has to tell
// assignee = none from a user called none (SPEC.md §9.2).
var Reserved = []string{"and", "or", "not", "in", "none", "me", "today"}

// IsReserved reports whether a name is one of Reserved, in any case.
func IsReserved(name string) bool {
	for _, w := range Reserved {
		if strings.EqualFold(strings.TrimSpace(name), w) {
			return true
		}
	}
	return false
}

// Quote leaves a value bare when it reads back as the same word.
func Quote(s string) string {
	bare := s != ""
	for i := 0; i < len(s); i++ {
		if !wordByte(s[i]) {
			bare = false
		}
	}
	if IsReserved(s) {
		bare = false
	}
	if bare {
		return s
	}
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func render(n Node, val func(string, *Value) string) string {
	var b strings.Builder
	var walk func(Node, int)
	// Precedence: or 1, and 2, not 3. A child binding looser than its parent
	// is bracketed.
	walk = func(n Node, outer int) {
		switch n := n.(type) {
		case Or:
			if outer > 1 {
				b.WriteString("(")
			}
			for i, x := range n.Of {
				if i > 0 {
					b.WriteString(" or ")
				}
				walk(x, 1)
			}
			if outer > 1 {
				b.WriteString(")")
			}
		case And:
			if outer > 2 {
				b.WriteString("(")
			}
			for i, x := range n.Of {
				if i > 0 {
					b.WriteString(" and ")
				}
				walk(x, 2)
			}
			if outer > 2 {
				b.WriteString(")")
			}
		case Not:
			b.WriteString("not ")
			walk(n.X, 3)
		case *Cond:
			b.WriteString(n.Field + " " + n.Op + " ")
			if n.Op != "in" && n.Op != "not in" {
				b.WriteString(val(n.Field, n.Values[0]))
				return
			}
			b.WriteString("(")
			for i, v := range n.Values {
				if i > 0 {
					b.WriteString(", ")
				}
				b.WriteString(val(n.Field, v))
			}
			b.WriteString(")")
		}
	}
	if n != nil {
		walk(n, 0)
	}
	return b.String()
}

// Compiling -----------------------------------------------------------------

// Compile turns a resolved query into a SQL condition over the item query's
// aliases: i for the item, s for its work state.
//
// A comparison with a NULL column is NULL, which a WHERE reads as false, but
// whose not is NULL too: not would not mean what it says. Conditions under a
// not are wrapped to be false instead; the rest are left bare, so SQLite can
// still use the column's index.
func Compile(n Node) (string, []any) {
	var args []any
	var walk func(Node, bool) string
	walk = func(n Node, negated bool) string {
		switch n := n.(type) {
		case And:
			return join(n.Of, " AND ", func(x Node) string { return walk(x, negated) })
		case Or:
			return join(n.Of, " OR ", func(x Node) string { return walk(x, negated) })
		case Not:
			return "NOT " + walk(n.X, true)
		case *Cond:
			return cond(n, &args, negated)
		}
		return "1"
	}
	if n == nil {
		return "", nil
	}
	return walk(n, false), args
}

func join(of []Node, sep string, walk func(Node) string) string {
	parts := make([]string, len(of))
	for i, x := range of {
		parts[i] = walk(x)
	}
	return "(" + strings.Join(parts, sep) + ")"
}

var columns = map[string]string{
	"assignee": "i.assignee_id", "reporter": "i.reporter_id", "bucket": "i.bucket_id",
	"team": "i.team_id", "priority": "i.priority",
	"created": "i.created_at", "started": "i.actual_start", "completed": "i.actual_end", "due": "i.planned_end",
}

func safe(s string) string { return "IFNULL(" + s + ", 0)" }

func cond(c *Cond, args *[]any, negated bool) string {
	col := columns[c.Field]
	arg := func(a ...any) { *args = append(*args, a...) }
	eq := func(v *Value) string {
		switch {
		case v.None && c.Field == "label":
			return "NOT EXISTS (SELECT 1 FROM item_labels il WHERE il.item_id = i.id)"
		case v.None:
			return col + " IS NULL"
		case c.Field == "label":
			arg(v.ID)
			return "EXISTS (SELECT 1 FROM item_labels il WHERE il.item_id = i.id AND il.label_id = ?)"
		case c.Field == "state" && v.Ref:
			arg(v.ID)
			return "i.state_id = ?"
		case c.Field == "state":
			arg(strings.ToLower(v.Raw)) // as store.Fold: the database never folds case
			return "s.name_folded = ?"
		case c.Field == "kind":
			arg(v.Raw)
			return "i.kind = ?"
		case c.Field == "due":
			arg(v.Day)
			return col + " = ?"
		case isDay(c.Field):
			arg(v.From, v.To)
			return "(" + col + " >= ? AND " + col + " < ?)"
		}
		arg(v.ID)
		return col + " = ?"
	}
	var s string
	switch c.Op {
	case "=":
		s = eq(c.Values[0])
	case "!=":
		return "NOT " + safe(eq(c.Values[0]))
	case "in", "not in":
		parts := make([]string, len(c.Values))
		for i, v := range c.Values {
			parts[i] = eq(v)
		}
		s = "(" + strings.Join(parts, " OR ") + ")"
		if c.Op == "not in" {
			return "NOT " + safe(s)
		}
	case "~", "!~":
		pattern := "%" + strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(c.Values[0].Raw) + "%"
		arg(pattern, pattern)
		s = "(i.title LIKE ? ESCAPE '\\' OR i.description LIKE ? ESCAPE '\\')"
		if c.Op == "!~" {
			return "NOT " + safe(s)
		}
	default:
		// < <= > >=: a priority by its number (lower is higher: priority <= 2
		// is P1 and P2), a due date as text, any other day by its bounds.
		v := c.Values[0]
		switch c.Field {
		case "priority":
			arg(v.ID)
			s = col + " " + c.Op + " ?"
		case "due":
			arg(v.Day)
			s = col + " " + c.Op + " ?"
		default:
			bound := map[string]int64{"<": v.From, "<=": v.To, ">": v.To, ">=": v.From}[c.Op]
			op := map[string]string{"<": "<", "<=": "<", ">": ">=", ">=": ">="}[c.Op]
			arg(bound)
			s = col + " " + op + " ?"
		}
	}
	if negated {
		return safe(s)
	}
	return s
}
