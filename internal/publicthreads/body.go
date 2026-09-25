package publicthreads

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	latheruntime "github.com/lathe-cli/lathe/pkg/runtime"
)

// buildCreateBody assembles the create-thread request body the same way the
// generated runtime does for a body with no envelope template: --set/--set-str
// take precedence and merge into one document, otherwise a --file body is used
// verbatim. Every create request must be a JSON object with a non-blank string
// userId. The merge mirrors Lathe's dotted-path semantics (object fields, array
// indices, and type inference for --set; forced strings for --set-str).
func buildCreateBody(file string, sets, stringSets []string) ([]byte, error) {
	return buildCreateBodyForVersion(file, sets, stringSets, "v1")
}

func buildCreateBodyForVersion(file string, sets, stringSets []string, version string) ([]byte, error) {
	if len(sets) > 0 || len(stringSets) > 0 {
		out := map[string]any{}
		for _, kv := range sets {
			path, value, err := parseSet(kv, "--set")
			if err != nil {
				return nil, err
			}
			if err := setNestedPath(out, path, inferValue(value)); err != nil {
				return nil, err
			}
		}
		for _, kv := range stringSets {
			path, value, err := parseSet(kv, "--set-str")
			if err != nil {
				return nil, err
			}
			if err := setNestedPath(out, path, value); err != nil {
				return nil, err
			}
		}
		body, err := json.Marshal(out)
		if err != nil {
			return nil, err
		}
		return body, validateCreateBodyForVersion(body, version)
	}
	if file != "" {
		body, err := latheruntime.ReadBody(file)
		if err != nil {
			return nil, err
		}
		return body, validateCreateBodyForVersion(body, version)
	}
	if version == "v2" {
		return []byte("{}"), nil
	}
	return nil, fmt.Errorf("create thread body is required and must include userId")
}

func validateCreateBody(body []byte) error {
	return validateCreateBodyForVersion(body, "v1")
}

func validateCreateBodyForVersion(body []byte, version string) error {
	if len(strings.TrimSpace(string(body))) == 0 {
		return fmt.Errorf("create thread body is required and must include userId")
	}

	var document map[string]any
	if err := json.Unmarshal(body, &document); err != nil {
		return fmt.Errorf("create thread body must be a JSON object: %w", err)
	}
	if document == nil {
		return fmt.Errorf("create thread body must be a JSON object")
	}
	userID, ok := document["userId"]
	if !ok && version == "v2" {
		return nil
	}
	if !ok {
		return fmt.Errorf("create thread body must include userId")
	}
	value, ok := userID.(string)
	if !ok {
		return fmt.Errorf("create thread body userId must be a string")
	}
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("create thread body userId must not be blank")
	}
	return nil
}

// The Project route requires an explicit choice, so no preset field can
// silently override inline configuration (or vice versa).
func validateProjectCreateBody(body []byte) error {
	if err := validateCreateBodyForVersion(body, "v2"); err != nil {
		return err
	}
	var document map[string]any
	_ = json.Unmarshal(body, &document)
	configuration, ok := document["configuration"].(map[string]any)
	if !ok {
		return fmt.Errorf("Project Session body must include configuration with type inline or agent")
	}
	var required []string
	switch configuration["type"] {
	case "inline":
		required = []string{"harness", "provider", "model", "instructions"}
	case "agent":
		required = []string{"agent_id"}
	default:
		return fmt.Errorf("configuration.type must be inline or agent")
	}
	allowed := map[string]bool{"type": true}
	for _, field := range required {
		value, ok := configuration[field].(string)
		if !ok || strings.TrimSpace(value) == "" {
			return fmt.Errorf("configuration.%s must be a non-blank string", field)
		}
		allowed[field] = true
	}
	for field := range configuration {
		if !allowed[field] {
			return fmt.Errorf("configuration.%s is not allowed for type %s; inline and Agent preset fields cannot be mixed", field, configuration["type"])
		}
	}
	return nil
}

func rejectLegacyConfiguration(body []byte) error {
	var document map[string]any
	_ = json.Unmarshal(body, &document)
	if _, exists := document["configuration"]; exists {
		return fmt.Errorf("configuration requires --project-id; for a preset use configuration.type=agent with configuration.agent_id")
	}
	return nil
}

func parseSet(kv, flag string) (string, string, error) {
	eq := strings.Index(kv, "=")
	if eq < 0 {
		return "", "", fmt.Errorf("invalid %s %q (expected key=value)", flag, kv)
	}
	path := kv[:eq]
	if path == "" {
		return "", "", fmt.Errorf("invalid %s %q (empty key)", flag, kv)
	}
	return path, kv[eq+1:], nil
}

type pathSegment struct {
	key string
	idx int // -1 = object field, >=0 = array index within key
}

func parsePath(path string) []pathSegment {
	parts := strings.Split(path, ".")
	segs := make([]pathSegment, 0, len(parts))
	for _, p := range parts {
		if open := strings.Index(p, "["); open >= 0 && strings.HasSuffix(p, "]") {
			key := p[:open]
			if idx, err := strconv.Atoi(p[open+1 : len(p)-1]); err == nil {
				segs = append(segs, pathSegment{key: key, idx: idx})
				continue
			}
		}
		segs = append(segs, pathSegment{key: p, idx: -1})
	}
	return segs
}

func setNestedPath(m map[string]any, path string, v any) error {
	return setNestedSegs(m, parsePath(path), v)
}

func setNestedSegs(m map[string]any, segs []pathSegment, v any) error {
	if len(segs) == 0 {
		return nil
	}
	seg := segs[0]
	rest := segs[1:]

	if seg.idx < 0 {
		if len(rest) == 0 {
			m[seg.key] = v
			return nil
		}
		switch next := m[seg.key].(type) {
		case map[string]any:
			return setNestedSegs(next, rest, v)
		case nil:
			child := map[string]any{}
			m[seg.key] = child
			return setNestedSegs(child, rest, v)
		default:
			return fmt.Errorf("conflicting --set: %s is not an object", seg.key)
		}
	}

	var arr []any
	switch existing := m[seg.key].(type) {
	case []any:
		arr = existing
	case nil:
		arr = []any{}
	default:
		return fmt.Errorf("conflicting --set: %s is not an array", seg.key)
	}
	for len(arr) <= seg.idx {
		arr = append(arr, nil)
	}
	if len(rest) == 0 {
		arr[seg.idx] = v
	} else {
		var child map[string]any
		switch existing := arr[seg.idx].(type) {
		case map[string]any:
			child = existing
		case nil:
			child = map[string]any{}
		default:
			return fmt.Errorf("conflicting --set: %s[%d] is not an object", seg.key, seg.idx)
		}
		if err := setNestedSegs(child, rest, v); err != nil {
			return err
		}
		arr[seg.idx] = child
	}
	m[seg.key] = arr
	return nil
}

func inferValue(s string) any {
	switch s {
	case "true":
		return true
	case "false":
		return false
	case "null":
		return nil
	}
	if i, err := strconv.ParseInt(s, 10, 64); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return s
}
