package api

import (
	"os"
	"sort"
	"strings"
	"testing"
)

// SPEC.md §11: the OpenAPI document is the definition of the interface rather
// than documentation generated from it. That only holds if the two cannot
// drift, which is what this test is for.
//
// SPEC.md §14.1 leans on the same correspondence: every MCP tool must
// correspond to exactly one operation in the document, and a tool without an
// operation must not exist.
func TestEveryRouteHasAnOperationAndViceVersa(t *testing.T) {
	doc, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	documented := parseOperations(t, string(doc))

	s := New(nil, nil)
	routed := map[string]bool{}
	for _, p := range s.Patterns() {
		routed[p] = true
	}

	// The stream is deliberately absent from the document (SPEC.md §12), and
	// backup lives on its own routes registered by the snapshot server.
	notRouted := map[string]bool{
		"GET /api/snapshots":  true,
		"POST /api/snapshots": true,
		"POST /api/restore":   true,
	}

	for op := range documented {
		if !routed[op] && !notRouted[op] {
			t.Errorf("api/openapi.yaml describes %s, which no route serves", op)
		}
	}
	for op := range routed {
		if !documented[op] {
			t.Errorf("%s is served but not described in api/openapi.yaml", op)
		}
	}
	if t.Failed() {
		var have []string
		for op := range routed {
			have = append(have, op)
		}
		sort.Strings(have)
		t.Logf("routes: %s", strings.Join(have, "\n        "))
	}
}

// Every operation must say whether an agent can reach it, because the agent
// tool list is derived from that (SPEC.md §14.3).
func TestEveryOperationDeclaresAgentExposure(t *testing.T) {
	doc, err := os.ReadFile("../../api/openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var operation string
	seen := map[string]bool{}
	for _, line := range strings.Split(string(doc), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "operationId:") {
			operation = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "operationId:"))
			if seen[operation] {
				t.Errorf("operationId %q appears twice", operation)
			}
			seen[operation] = true
		}
		if strings.HasPrefix(strings.TrimSpace(line), "x-agent:") && operation != "" {
			delete(seen, operation)
			seen["ok:"+operation] = true
			operation = ""
		}
	}
	for k := range seen {
		if !strings.HasPrefix(k, "ok:") {
			t.Errorf("operation %q does not declare x-agent", k)
		}
	}
}

// parseOperations reads "METHOD /path" pairs out of the document's paths
// section. The file is ours and its shape is fixed, so this is a scanner
// rather than a YAML dependency.
func parseOperations(t *testing.T, doc string) map[string]bool {
	t.Helper()
	methods := map[string]bool{"get": true, "post": true, "put": true, "patch": true, "delete": true}
	out := map[string]bool{}
	inPaths := false
	path := ""
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if inPaths && len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			break // a new top-level key, so the paths section is over
		}
		if !inPaths {
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		switch indent(line) {
		case 2:
			path = strings.TrimSuffix(trimmed, ":")
		case 4:
			verb := strings.TrimSuffix(trimmed, ":")
			if methods[verb] && path != "" {
				out[strings.ToUpper(verb)+" "+path] = true
			}
		}
	}
	if len(out) == 0 {
		t.Fatal("no operations were parsed out of api/openapi.yaml")
	}
	return out
}

func indent(line string) int {
	return len(line) - len(strings.TrimLeft(line, " "))
}
