export function matchScore({ identifier = "", title = "" }, q) {
  identifier = identifier.toLowerCase();
  title = title.toLowerCase();
  if (identifier === q) return 5;
  if (title.startsWith(q)) return 4;
  for (const boundary of title.matchAll(/[^\p{L}\p{N}\p{M}]/gu)) {
    if (title.startsWith(q, boundary.index + boundary[0].length)) return 3;
  }
  if (title.includes(q)) return 2;
  if (identifier.includes(q)) return 1;
  return 0;
}

export function ranked(options, text) {
  const q = text.trim().toLowerCase();
  if (!q) return options;
  return options.map((item) => ({ item, score: matchScore(item, q) }))
    .filter(({ score }) => score).sort((a, b) => b.score - a.score).map(({ item }) => item);
}
