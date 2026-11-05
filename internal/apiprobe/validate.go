package apiprobe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"
)

// specServer is the base URL routes are matched against. The probe's own
// base URL (real API or mock) does not matter for validation.
const specServer = "https://api.bunny.net"

// Validator checks recorded responses against an OpenAPI document.
type Validator struct {
	router routers.Router
}

// LoadValidator reads an OpenAPI 3 document from a file.
func LoadValidator(path string) (*Validator, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path comes from the command line
	if err != nil {
		return nil, fmt.Errorf("read spec: %w", err)
	}
	return NewValidator(data)
}

// NewValidator builds a validator from an OpenAPI 3 document.
//
// The document is not validated as a whole: bunny.net's spec has small
// errors in endpoints the probe does not use (for example a page default of
// 0 with minimum 1 on /pullzone), and those must not stop the DNS checks.
func NewValidator(spec []byte) (*Validator, error) {
	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromData(spec)
	if err != nil {
		return nil, fmt.Errorf("parse spec: %w", err)
	}
	doc.Servers = openapi3.Servers{{URL: specServer}}
	router, err := gorillamux.NewRouter(doc)
	if err != nil { // coverage-ignore: only fails for a document that LoadFromData accepted but has unroutable paths
		return nil, fmt.Errorf("build router: %w", err)
	}
	return &Validator{router: router}, nil
}

// Check sets ex.Findings to every difference between the recorded response
// and the documented one.
func (v *Validator) Check(ctx context.Context, ex *Exchange) {
	if ex.Error != "" {
		ex.Findings = append(ex.Findings, "request failed: "+ex.Error)
		return
	}
	target := specServer + ex.Path
	if ex.Query != "" {
		target += "?" + ex.Query
	}
	req, err := http.NewRequestWithContext(ctx, ex.Method, target, bytes.NewReader(ex.RequestBody))
	if err != nil { // coverage-ignore: method and path come from the probe itself
		ex.Findings = append(ex.Findings, "cannot build request: "+err.Error())
		return
	}
	route, params, err := v.router.FindRoute(req)
	if err != nil {
		ex.Findings = append(ex.Findings, fmt.Sprintf("%s %s is not documented", ex.Method, ex.Path))
		return
	}

	header := http.Header{}
	if ex.ContentType != "" {
		header.Set("Content-Type", ex.ContentType)
	}
	in := &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: params,
			Route:      route,
		},
		Status: ex.Status,
		Header: header,
		Options: &openapi3filter.Options{
			IncludeResponseStatus: true, // an undocumented status code is a finding
			MultiError:            true,
		},
	}
	in.SetBodyBytes(ex.rawBody)

	if err := openapi3filter.ValidateResponse(ctx, in); err != nil {
		ex.Findings = append(ex.Findings, flatten(err)...)
	}
}

// flatten turns a (multi-)error from kin-openapi into short, stable lines.
func flatten(err error) []string {
	var multi openapi3.MultiError
	if errors.As(err, &multi) {
		out := make([]string, 0, len(multi))
		for _, e := range multi {
			out = append(out, flatten(e)...)
		}
		return dedupe(out)
	}
	var respErr *openapi3filter.ResponseError
	if errors.As(err, &respErr) && respErr.Err != nil && !errors.Is(respErr.Err, err) {
		var inner openapi3.MultiError
		if errors.As(respErr.Err, &inner) {
			return flatten(inner)
		}
	}
	var schemaErr *openapi3.SchemaError
	if errors.As(err, &schemaErr) {
		field := "/" + strings.Join(schemaErr.JSONPointer(), "/")
		return []string{fmt.Sprintf("field %s: %s", field, schemaErr.Reason)}
	}
	return []string{firstLine(err.Error())}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return strings.TrimSpace(s[:i])
	}
	return strings.TrimSpace(s)
}

func dedupe(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, s := range in {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Strings(out)
	return out
}

// sensitiveKey matches JSON field names whose values must never be recorded.
var sensitiveKey = regexp.MustCompile(`(?i)(key|token|secret|password|email|userid|accountid)`)

// notSensitive are field names that match sensitiveKey but only hold
// bunny.net error codes, which the report needs.
var notSensitive = map[string]bool{"ErrorKey": true}

// Sanitize fills ex.ResponseBody from the raw body with sensitive values
// replaced. Non-JSON bodies are stored as a JSON string, truncated to 64 KiB.
func Sanitize(ex *Exchange) {
	if len(ex.rawBody) == 0 {
		return
	}
	var v any
	if err := json.Unmarshal(ex.rawBody, &v); err == nil {
		b, _ := json.Marshal(redact(v)) //nolint:errcheck // re-encoding decoded JSON cannot fail
		ex.ResponseBody = b
		return
	}
	text := string(ex.rawBody)
	if len(text) > 64<<10 {
		text = text[:64<<10] + "…(truncated)"
	}
	b, _ := json.Marshal(text) //nolint:errcheck // a string always encodes
	ex.ResponseBody = b
}

func redact(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, val := range t {
			if s, ok := val.(string); ok && s != "" && sensitiveKey.MatchString(k) && !notSensitive[k] {
				t[k] = "REDACTED"
				continue
			}
			t[k] = redact(val)
		}
		return t
	case []any:
		for i := range t {
			t[i] = redact(t[i])
		}
		return t
	default:
		return v
	}
}
