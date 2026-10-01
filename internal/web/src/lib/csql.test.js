// The client's half of the CSQL corpus: internal/csql/csql_test.go runs the
// same file against the server's parser, so the two cannot drift apart.
// Run with `npm test`.

import { test } from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { parse, format, complete, toBasic, fromBasic, CsqlError } from "./csql.js";

const corpus = JSON.parse(readFileSync(new URL("../../../csql/testdata/corpus.json", import.meta.url)));

test("the corpus parses and fails as the server's parser does", () => {
  for (const c of corpus) {
    if (!c.err) {
      assert.equal(format(parse(c.q)), c.out, c.q);
      continue;
    }
    assert.throws(() => parse(c.q), (err) => {
      assert.ok(err instanceof CsqlError, c.q);
      assert.deepEqual([err.message, err.pos, err.end], [c.err, ...c.span], c.q);
      return true;
    });
  }
});

test("what the query wants at the caret", () => {
  const at = (q, caret = q.length) => {
    const { want, field, prefix } = complete(q, caret);
    return [want, field, prefix].filter((x) => x !== undefined && x !== "").join(" ");
  };
  assert.equal(at(""), "field");
  assert.equal(at("assi"), "field assi");
  assert.equal(at("assignee "), "op assignee");
  assert.equal(at("assignee no"), "op assignee no");
  assert.equal(at("assignee = "), "value assignee");
  assert.equal(at("assignee = ma"), "value assignee ma");
  assert.equal(at("assignee in ("), "value assignee");
  assert.equal(at("assignee in (marta, sa"), "value assignee sa");
  assert.equal(at("assignee in (marta "), "next assignee");
  assert.equal(at("bucket = \"Camp"), "value bucket Camp");
  assert.equal(at("assignee = marta "), "joiner");
  assert.equal(at("assignee = marta an"), "joiner an");
  assert.equal(at("(assignee = marta "), "joiner");
  assert.equal(at("assignee = marta and "), "field");
  assert.equal(at("not ("), "field");
  assert.equal(at("created >= "), "value created");
  // Wrong before the caret: nothing to offer, the error says it.
  assert.equal(at("asignee = "), "");
  assert.equal(at("priority = 9 and "), "");
  // The caret, not the end, is where it asks.
  assert.equal(at("assignee = marta and label = api", 11), "value assignee");
});

test("the chips show a query of their shape, and only that", () => {
  const round = (q) => fromBasic(toBasic(parse(q)));
  for (const q of [
    "",
    "assignee in (marta, none) and assignee != me",
    "label = api and label not in (roses, \"needs review\")",
    "bucket = DEV-6 and priority in (1, 2) and state != Done",
    "created >= -7d and started = none and completed = today and due < today",
    "created >= 2026-09-01 and created <= 2026-09-30",
  ]) assert.equal(round(q), q);
  for (const q of [
    "assignee = marta or label = api",
    "not label = api",
    "label = api and label = roses", // every one of them: not what a chip says
    "priority <= 2",
    "text ~ boiler",
    "due > today",
    "created = none",
  ].filter((q) => { try { parse(q); return true; } catch { return false; } })) {
    assert.equal(toBasic(parse(q)), null, q);
  }
});
