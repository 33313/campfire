// CSQL in the browser: the same grammar as internal/csql, for suggestions and
// for syntax errors as you type, at no request (SPEC.md §9.2). The server
// parses again and is the authority; it alone resolves names. Both parsers
// answer to internal/csql/testdata/corpus.json, message for message.
//
// Also here: the chips' half of the language. A query of the "basic" shape —
// per field one any-of set and one none-of set, or a range of days, all
// and-ed — is what the chips can show and build.

const VALUE_OPS = ["=", "!=", "in", "not in"];
const DAY_OPS = ["=", "!=", "<", "<=", ">", ">="];

/** Fields, in the order they are offered, and what each takes. */
export const FIELDS = ["assignee", "reporter", "label", "bucket", "priority", "state", "team",
  "created", "started", "completed", "due", "text"];
export const FIELD = {
  assignee: { ops: VALUE_OPS, nullable: true, named: true },
  reporter: { ops: VALUE_OPS, named: true },
  label: { ops: VALUE_OPS, nullable: true, named: true },
  bucket: { ops: VALUE_OPS, nullable: true, named: true },
  priority: { ops: [...VALUE_OPS, "<", "<=", ">", ">="] },
  state: { ops: VALUE_OPS },
  team: { ops: VALUE_OPS, named: true },
  created: { ops: DAY_OPS, day: true },
  started: { ops: DAY_OPS, nullable: true, day: true },
  completed: { ops: DAY_OPS, nullable: true, day: true },
  due: { ops: DAY_OPS, nullable: true, day: true },
  text: { ops: ["~", "!~"] },
};

export class CsqlError extends Error {
  /** `expect` is set when the query ended where more was needed: what that was. */
  constructor(msg, pos, end, expect = null) {
    super(msg);
    this.pos = pos;
    this.end = end;
    this.expect = expect;
  }
}

// Lexing -------------------------------------------------------------------

const wordChar = (c) => /[A-Za-z0-9_\-+.@]/.test(c) || c.charCodeAt(0) >= 0x80;
const quoteGo = (s) => JSON.stringify(s); // as Go's strconv.Quote, for what a query holds

function lex(src) {
  const out = [];
  let i = 0;
  while (i < src.length) {
    const c = src[i];
    if (c === " " || c === "\t" || c === "\n" || c === "\r") { i++; continue; }
    if (c === '"' || c === "'") {
      let text = "";
      let j = i + 1;
      for (; j < src.length && src[j] !== c; j++) {
        if (src[j] === "\\" && j + 1 < src.length) j++;
        text += src[j];
      }
      if (j >= src.length) throw new CsqlError("this quote is never closed", i, src.length);
      out.push({ kind: "s", text, pos: i, end: j + 1 });
      i = j + 1;
    } else if (c === "#") {
      let j = i + 1;
      while (j < src.length && src[j] >= "0" && src[j] <= "9") j++;
      if (j === i + 1) throw new CsqlError("# is followed by an id, such as #12", i, i + 1);
      out.push({ kind: "#", text: src.slice(i + 1, j), pos: i, end: j });
      i = j;
    } else if ("(),=~".includes(c)) {
      out.push({ kind: "p", text: c, pos: i, end: i + 1 });
      i++;
    } else if (c === "!" || c === "<" || c === ">") {
      let j = i + 1;
      if (j < src.length && (src[j] === "=" || (c === "!" && src[j] === "~"))) j++;
      if (c === "!" && j === i + 1) throw new CsqlError("! is only part of != and !~; use not", i, j);
      out.push({ kind: "p", text: src.slice(i, j), pos: i, end: j });
      i = j;
    } else if (wordChar(c)) {
      let j = i;
      while (j < src.length && wordChar(src[j])) j++;
      out.push({ kind: "w", text: src.slice(i, j), pos: i, end: j });
      i = j;
    } else {
      throw new CsqlError(`${quoteGo(c)} has no meaning here`, i, i + 1);
    }
  }
  out.push({ kind: "", text: "", pos: src.length, end: src.length });
  return out;
}

// Parsing ------------------------------------------------------------------

const describe = (t) => (t.kind ? quoteGo(t.text) : "the end");
const RELATIVE = /^[+-]\d{1,4}[dw]$/;

function isDate(s) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(s);
  if (!m) return false;
  const d = new Date(Date.UTC(+m[1], +m[2] - 1, +m[3]));
  return d.getUTCFullYear() === +m[1] && d.getUTCMonth() === +m[2] - 1 && d.getUTCDate() === +m[3];
}

function parser(toks) {
  let i = 0;
  const peek = () => toks[i];
  const next = () => toks[i++];
  const kw = (w) => peek().kind === "w" && peek().text.toLowerCase() === w;
  const punct = (s) => peek().kind === "p" && peek().text === s;
  // An error at the end of the query says what was wanted there, for
  // suggestions; anywhere else it is just wrong.
  const fail = (t, msg, expect) => new CsqlError(msg, t.pos, t.end, t.kind ? null : expect);

  function or() {
    const of = [and()];
    while (kw("or")) { next(); of.push(and()); }
    return of.length === 1 ? of[0] : { type: "or", of };
  }
  function and() {
    const of = [not()];
    while (kw("and")) { next(); of.push(not()); }
    return of.length === 1 ? of[0] : { type: "and", of };
  }
  function not() {
    if (kw("not")) { next(); return { type: "not", x: not() }; }
    if (punct("(")) {
      const open = next();
      const n = or();
      if (!punct(")")) {
        const t = peek();
        if (!t.kind) throw new CsqlError("this ( is never closed", open.pos, open.end, { want: "joiner" });
        throw fail(t, `expected ) before ${describe(t)}`);
      }
      next();
      return n;
    }
    return cond();
  }
  function cond() {
    const t = next();
    const name = t.text.toLowerCase();
    const f = FIELD[name];
    if (t.kind !== "w" || !f) {
      throw fail(t, `expected a field (${FIELDS.join(", ")}), not ${describe(t)}`, { want: "field" });
    }
    const c = { type: "cond", field: name, op: "", values: [], pos: t.pos };
    const op = next();
    c.op = op.kind === "w" ? op.text.toLowerCase() : op.text;
    if (c.op === "not" && op.kind === "w" && kw("in")) { next(); c.op = "not in"; }
    const allowed = f.ops.some((o) => o === c.op && (op.kind === "p") === (o !== "in" && o !== "not in"));
    if (!allowed) {
      throw fail(op, `expected an operator after ${name} (${f.ops.join(", ")}), not ${describe(op)}`,
        { want: "op", field: name });
    }
    if (c.op !== "in" && c.op !== "not in") {
      const v = value(name, false);
      if (v.none && c.op !== "=" && c.op !== "!=") throw new CsqlError("none only takes = or !=", v.pos, v.pos + 4);
      c.values = [v];
      return c;
    }
    if (!punct("(")) {
      const t = peek();
      throw fail(t, `expected ( after ${c.op}, as in (marta, sally)`, { want: "open", field: name });
    }
    next();
    for (;;) {
      c.values.push(value(name, true));
      if (punct(",")) { next(); continue; }
      if (punct(")")) { next(); return c; }
      const t = peek();
      throw fail(t, `expected , or ) in the list, not ${describe(t)}`, { want: "next", field: name });
    }
  }
  function value(field, inList) {
    const t = next();
    const v = { raw: t.text, quoted: false, pos: t.pos, ref: false, none: false };
    const f = FIELD[field];
    if (t.kind === "s") {
      v.quoted = true;
    } else if (t.kind === "#") {
      if (!f.named && field !== "state") throw new CsqlError(`${field} does not take an #id`, t.pos, t.end);
      v.raw = `#${t.text}`;
      v.ref = true;
      return v;
    } else if (t.kind === "w") {
      const low = t.text.toLowerCase();
      if (["and", "or", "not", "in"].includes(low)) {
        throw new CsqlError(`expected a value for ${field}, not the keyword ${low}`, t.pos, t.end);
      }
      if (low === "none") {
        if (!f.nullable) throw new CsqlError(`${field} is never none`, t.pos, t.end);
        v.raw = low;
        v.none = true;
        return v;
      }
      if (low === "me" || low === "today") v.raw = low;
    } else {
      throw fail(t, `expected a value for ${field}, not ${describe(t)}`, { want: "value", field, inList });
    }
    if (field === "priority") {
      const s = v.raw.toLowerCase().replace(/^p/, "");
      const n = /^\d+$/.test(s) ? Number(s) : NaN;
      if (!(n >= 1 && n <= 4)) throw new CsqlError("priority is 1 to 4, or P1 to P4", t.pos, t.end);
      v.raw = String(n);
    } else if (f.day) {
      v.raw = v.raw.toLowerCase();
      if (!isDate(v.raw) && v.raw !== "today" && !RELATIVE.test(v.raw)) {
        throw new CsqlError(`${field} takes a day: 2026-09-01, today, -7d or +2w`, t.pos, t.end);
      }
    }
    return v;
  }
  function parse() {
    if (!peek().kind) return null;
    const n = or();
    const t = peek();
    if (t.kind) {
      if (t.kind === "p" && t.text === ")") throw new CsqlError("this ) closes nothing", t.pos, t.end);
      throw new CsqlError(`expected and or or before ${describe(t)}`, t.pos, t.end);
    }
    return n;
  }
  return { parse };
}

/** The query as a tree, or a CsqlError saying where it is wrong. Empty is null. */
export function parse(src) {
  return parser(lex(src)).parse();
}

// Printing -----------------------------------------------------------------

/** A value as a query writes it: bare when it reads back as one word. */
export function quote(s) {
  const bare = s !== "" && [...s].every(wordChar)
    && !["and", "or", "not", "in", "none", "me", "today"].includes(s.toLowerCase());
  return bare ? s : `"${s.replace(/[\\"]/g, "\\$&")}"`;
}

/** One value as printed, which is also how the chips hold it. */
export function token(v) {
  if (v.ref || v.none || (!v.quoted && (v.raw === "me" || v.raw === "today"))) return v.raw;
  return quote(v.raw);
}

/** The query as written, normalised: what the corpus compares. */
export function format(n) {
  const walk = (n, outer) => {
    if (n.type === "or" || n.type === "and") {
      const level = n.type === "or" ? 1 : 2;
      const s = n.of.map((x) => walk(x, level)).join(` ${n.type} `);
      return outer > level ? `(${s})` : s;
    }
    if (n.type === "not") return `not ${walk(n.x, 3)}`;
    const vals = n.values.map(token);
    return n.op === "in" || n.op === "not in"
      ? `${n.field} ${n.op} (${vals.join(", ")})` : `${n.field} ${n.op} ${vals[0]}`;
  };
  return n ? walk(n, 0) : "";
}

// Suggesting ---------------------------------------------------------------

/**
 * What the query wants at the caret: `want` is field, op, value, open, next
 * or joiner, or null when the query is wrong before the caret. `from` is where
 * the word being typed starts, which an offer replaces, and `prefix` is that
 * word so far, unquoted.
 */
export function complete(src, caret) {
  const before = src.slice(0, caret);
  let toks;
  let typing = null;
  try {
    toks = lex(before);
  } catch (err) {
    // An open quote at the caret is a quoted value being typed.
    if (err.message !== "this quote is never closed") return { want: null };
    typing = { pos: err.pos, prefix: before.slice(err.pos + 1) };
    try { toks = lex(before.slice(0, err.pos)); } catch { return { want: null }; }
  }
  toks.pop(); // the end
  const last = toks.at(-1);
  if (!typing && last && last.end === caret && (last.kind === "w" || last.kind === "#")) {
    toks.pop();
    typing = { pos: last.pos, prefix: last.kind === "#" ? `#${last.text}` : last.text };
  }
  const depth = toks.reduce((d, t) => d + (t.text === "(" && t.kind === "p") - (t.text === ")" && t.kind === "p"), 0);
  const at = { from: typing ? typing.pos : caret, prefix: typing ? typing.prefix : "", depth };
  if (!toks.length) return { ...at, want: "field" };
  toks.push({ kind: "", text: "", pos: before.length, end: before.length });
  try {
    parser(toks).parse();
    return { ...at, want: "joiner" };
  } catch (err) {
    if (!(err instanceof CsqlError) || !err.expect) return { want: null };
    return { ...at, ...err.expect, want: err.expect.want };
  }
}

// The chips' shape ---------------------------------------------------------

const DAY_FIELDS = FIELDS.filter((f) => FIELD[f].day);

/**
 * The chips' state for a query, or null when it is not of their shape. A
 * value field holds the tokens it takes (`plus`) and refuses (`minus`); a
 * day field holds a range (`from`, `to`), a day before which (`before`), or
 * `none`.
 */
export function toBasic(n) {
  const basic = {};
  const conds = !n ? [] : n.type === "and" ? n.of : [n];
  for (const c of conds) {
    if (c.type !== "cond" || c.field === "text") return null;
    const f = FIELD[c.field];
    if (f.day) {
      const d = (basic[c.field] ||= {});
      const v = token(c.values[0]);
      if (c.values[0].none && c.op === "=" && !Object.keys(d).length) d.none = true;
      else if (c.op === "=" && !Object.keys(d).length) { d.from = v; d.to = v; }
      else if (c.op === ">=" && !d.from && !d.none && !d.before) d.from = v;
      else if (c.op === "<=" && !d.to && !d.none && !d.before) d.to = v;
      else if (c.op === "<" && !Object.keys(d).length) d.before = v;
      else return null;
      continue;
    }
    const chip = (basic[c.field] ||= { plus: [], minus: [] });
    const side = c.op === "=" || c.op === "in" ? "plus" : c.op === "!=" || c.op === "not in" ? "minus" : null;
    if (!side || chip[side].length) return null; // a second set would be all-of, which chips do not say
    chip[side] = c.values.map(token);
  }
  return basic;
}

/** The query the chips' state says. */
export function fromBasic(basic) {
  const parts = [];
  for (const field of FIELDS) {
    const b = basic[field];
    if (!b) continue;
    if (DAY_FIELDS.includes(field)) {
      if (b.none) parts.push(`${field} = none`);
      else if (b.before) parts.push(`${field} < ${b.before}`);
      else if (b.from && b.from === b.to) parts.push(`${field} = ${b.from}`);
      else {
        if (b.from) parts.push(`${field} >= ${b.from}`);
        if (b.to) parts.push(`${field} <= ${b.to}`);
      }
      continue;
    }
    for (const [side, one, many] of [["plus", "=", "in"], ["minus", "!=", "not in"]]) {
      const vals = b[side] || [];
      if (vals.length === 1) parts.push(`${field} ${one} ${vals[0]}`);
      else if (vals.length > 1) parts.push(`${field} ${many} (${vals.join(", ")})`);
    }
  }
  return parts.join(" and ");
}

/** A server's error span, in UTF-8 bytes, as indices into the string. */
export function spanOf(src, span) {
  if (!span) return null;
  const enc = new TextEncoder();
  const at = (b) => {
    let i = 0, n = 0;
    while (i < src.length && n < b) { const cp = src.codePointAt(i); n += enc.encode(String.fromCodePoint(cp)).length; i += cp > 0xffff ? 2 : 1; }
    return i;
  };
  return [at(span[0]), at(span[1])];
}
