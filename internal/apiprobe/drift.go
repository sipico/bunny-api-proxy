package apiprobe

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// dnsPathPrefix selects the part of bunny.net's API this project uses.
const dnsPathPrefix = "/dnszone"

var httpMethods = []string{"get", "put", "post", "delete", "patch"}

// SpecDrift lists what changed in the DNS part of the API between two
// OpenAPI documents: paths, operations, parameters, request bodies,
// response codes, and every schema those operations use (directly or
// through $ref). Descriptions and examples are ignored.
func SpecDrift(oldSpec, newSpec []byte) ([]string, error) {
	var o, n map[string]any
	if err := json.Unmarshal(oldSpec, &o); err != nil {
		return nil, fmt.Errorf("parse old spec: %w", err)
	}
	if err := json.Unmarshal(newSpec, &n); err != nil {
		return nil, fmt.Errorf("parse new spec: %w", err)
	}
	op, np := dnsPaths(o), dnsPaths(n)
	var out []string

	for _, p := range sortedKeys(np) {
		if _, ok := op[p]; !ok {
			out = append(out, fmt.Sprintf("new path %s (%s)", p, strings.Join(methodsOf(np[p]), ", ")))
		}
	}
	for _, p := range sortedKeys(op) {
		if _, ok := np[p]; !ok {
			out = append(out, fmt.Sprintf("removed path %s", p))
		}
	}
	for _, p := range sortedKeys(np) {
		oldPath, ok := op[p]
		if !ok {
			continue
		}
		newPath := np[p]
		for _, m := range httpMethods {
			oldOp, inOld := oldPath[m].(map[string]any)
			newOp, inNew := newPath[m].(map[string]any)
			label := strings.ToUpper(m) + " " + p
			switch {
			case inNew && !inOld:
				out = append(out, "new operation "+label)
			case inOld && !inNew:
				out = append(out, "removed operation "+label)
			case inOld && inNew:
				out = append(out, operationDrift(label, oldOp, newOp)...)
			}
		}
	}

	oldSchemas, newSchemas := schemasOf(o), schemasOf(n)
	used := schemaClosure(np, newSchemas)
	for name := range schemaClosure(op, oldSchemas) {
		used[name] = true
	}
	for _, name := range sortedKeys(used) {
		oldS, inOld := oldSchemas[name].(map[string]any)
		newS, inNew := newSchemas[name].(map[string]any)
		switch {
		case inNew && !inOld:
			out = append(out, "new schema "+name)
		case inOld && !inNew:
			out = append(out, "removed schema "+name)
		case inOld && inNew:
			out = append(out, schemaDrift(name, oldS, newS)...)
		}
	}
	return out, nil
}

func operationDrift(label string, o, n map[string]any) []string {
	var out []string
	if a, b := paramNames(o), paramNames(n); !reflect.DeepEqual(a, b) {
		out = append(out, fmt.Sprintf("%s: query/path parameters %v -> %v", label, a, b))
	}
	if a, b := sortedKeys(asMap(o["responses"])), sortedKeys(asMap(n["responses"])); !reflect.DeepEqual(a, b) {
		out = append(out, fmt.Sprintf("%s: response codes %v -> %v", label, a, b))
	}
	if a, b := sortedKeys(asMap(asMap(o["requestBody"])["content"])), sortedKeys(asMap(asMap(n["requestBody"])["content"])); !reflect.DeepEqual(a, b) {
		out = append(out, fmt.Sprintf("%s: request content types %v -> %v", label, a, b))
	}
	return out
}

func schemaDrift(name string, o, n map[string]any) []string {
	var out []string
	op, np := asMap(o["properties"]), asMap(n["properties"])
	for _, k := range sortedKeys(np) {
		if _, ok := op[k]; !ok {
			out = append(out, fmt.Sprintf("schema %s: new field %s", name, k))
		}
	}
	for _, k := range sortedKeys(op) {
		if _, ok := np[k]; !ok {
			out = append(out, fmt.Sprintf("schema %s: removed field %s", name, k))
		}
	}
	for _, k := range sortedKeys(np) {
		if ov, ok := op[k]; ok && !reflect.DeepEqual(shape(ov), shape(np[k])) {
			out = append(out, fmt.Sprintf("schema %s: field %s changed: %s -> %s", name, k, compact(shape(ov)), compact(shape(np[k]))))
		}
	}
	for _, key := range []string{"type", "enum", "x-enumNames", "required", "additionalProperties"} {
		if !reflect.DeepEqual(o[key], n[key]) {
			out = append(out, fmt.Sprintf("schema %s: %s %s -> %s", name, key, compact(o[key]), compact(n[key])))
		}
	}
	return out
}

// shape drops documentation-only keys so wording changes are not reported.
func shape(v any) any {
	switch t := v.(type) {
	case map[string]any:
		out := map[string]any{}
		for k, val := range t {
			if k == "description" || k == "example" || k == "examples" || k == "title" {
				continue
			}
			out[k] = shape(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i := range t {
			out[i] = shape(t[i])
		}
		return out
	default:
		return v
	}
}

func compact(v any) string {
	if v == nil {
		return "(none)"
	}
	b, _ := json.Marshal(v) //nolint:errcheck // values came from json.Unmarshal
	s := string(b)
	if len(s) > 200 {
		s = s[:200] + "…"
	}
	return s
}

func dnsPaths(spec map[string]any) map[string]map[string]any {
	out := map[string]map[string]any{}
	for p, v := range asMap(spec["paths"]) {
		if strings.HasPrefix(strings.ToLower(p), dnsPathPrefix) {
			out[p] = asMap(v)
		}
	}
	return out
}

func schemasOf(spec map[string]any) map[string]any {
	return asMap(asMap(spec["components"])["schemas"])
}

// schemaClosure returns every schema reachable from the given paths.
func schemaClosure(paths map[string]map[string]any, schemas map[string]any) map[string]bool {
	seen := map[string]bool{}
	var todo []string
	for _, p := range paths {
		todo = append(todo, refsIn(p)...)
	}
	for len(todo) > 0 {
		name := todo[len(todo)-1]
		todo = todo[:len(todo)-1]
		if seen[name] {
			continue
		}
		seen[name] = true
		todo = append(todo, refsIn(schemas[name])...)
	}
	return seen
}

func refsIn(v any) []string {
	var out []string
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok && k == "$ref" {
				out = append(out, s[strings.LastIndex(s, "/")+1:])
				continue
			}
			out = append(out, refsIn(val)...)
		}
	case []any:
		for _, val := range t {
			out = append(out, refsIn(val)...)
		}
	}
	return out
}

func paramNames(op map[string]any) []string {
	params, _ := op["parameters"].([]any) //nolint:errcheck // missing means no parameters
	out := make([]string, 0, len(params))
	for _, p := range params {
		m := asMap(p)
		out = append(out, fmt.Sprintf("%v:%v", m["in"], m["name"]))
	}
	sort.Strings(out)
	return out
}

func methodsOf(path map[string]any) []string {
	var out []string
	for _, m := range httpMethods {
		if _, ok := path[m]; ok {
			out = append(out, strings.ToUpper(m))
		}
	}
	return out
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any) //nolint:errcheck // a missing or non-object value is an empty map
	return m
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
