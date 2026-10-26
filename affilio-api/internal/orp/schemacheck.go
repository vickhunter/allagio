package orp

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

// This checker exists so tests can validate documents against the canonical
// schemas in docs/protocol/schema/0.3. Request handling does not call it.
// GET /orp/schemas serves an embedded copy of those files.

var (
	catalogMu sync.Mutex
	catalogs  = map[string]*catalog{}
)

var schemaAnnotations = map[string]bool{
	"$schema": true, "$id": true, "$defs": true, "$comment": true, "$ref": true,
	"title": true, "description": true, "default": true, "examples": true, "deprecated": true,
}

var schemaKeywords = map[string]bool{
	"allOf": true, "anyOf": true, "oneOf": true, "if": true, "then": true, "else": true,
	"type": true, "const": true, "enum": true, "format": true, "pattern": true,
	"minLength": true, "maxLength": true, "minimum": true, "maximum": true,
	"minItems": true, "maxItems": true, "uniqueItems": true, "items": true,
	"required": true, "properties": true, "additionalProperties": true,
	"propertyNames": true, "minProperties": true, "maxProperties": true,
}

type schemaDoc struct {
	path string
	root any
}

type catalog struct {
	byPath   map[string]*schemaDoc
	byID     map[string]*schemaDoc
	patterns map[string]*regexp.Regexp
}

type invalid struct {
	path string
	msg  string
}

func (e *invalid) Error() string {
	if e.path == "" {
		return e.msg
	}
	return e.path + ": " + e.msg
}

// ValidateAgainst checks instance against a schema file under schema/0.3.
func ValidateAgainst(schemaFile string, instance any) error {
	abs, err := filepath.Abs(schemaFile)
	if err != nil {
		return err
	}
	root, err := schemaRoot(abs)
	if err != nil {
		return err
	}
	cat, err := loadCatalog(root)
	if err != nil {
		return err
	}
	doc, ok := cat.byPath[abs]
	if !ok {
		return fmt.Errorf("schema not loaded: %s", abs)
	}
	return cat.check(abs, doc.root, instance, "")
}

func schemaRoot(file string) (string, error) {
	dir := filepath.Dir(file)
	for {
		if _, err := os.Stat(filepath.Join(dir, "defs.schema.json")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("defs.schema.json not found")
		}
		dir = parent
	}
}

func loadCatalog(root string) (*catalog, error) {
	catalogMu.Lock()
	defer catalogMu.Unlock()
	if cat, ok := catalogs[root]; ok {
		return cat, nil
	}
	cat := &catalog{
		byPath:   map[string]*schemaDoc{},
		byID:     map[string]*schemaDoc{},
		patterns: map[string]*regexp.Regexp{},
	}
	if err := walkSchemaDir(cat, root); err != nil {
		return nil, err
	}
	// 0.4 schemas $ref the 0.3 defs by absolute $id. Index that sibling too.
	if filepath.Base(root) == "0.4" {
		sibling := filepath.Join(filepath.Dir(root), "0.3")
		if info, err := os.Stat(sibling); err == nil && info.IsDir() {
			if err := walkSchemaDir(cat, sibling); err != nil {
				return nil, err
			}
		}
	}
	catalogs[root] = cat
	return cat, nil
}

func walkSchemaDir(cat *catalog, root string) error {
	return filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".json") {
			return err
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var node any
		if err := json.Unmarshal(raw, &node); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		abs, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		doc := &schemaDoc{path: abs, root: node}
		cat.byPath[abs] = doc
		if obj, ok := node.(map[string]any); ok {
			if id, ok := obj["$id"].(string); ok && id != "" {
				cat.byID[id] = doc
			}
		}
		return nil
	})
}

func (cat *catalog) check(file string, schema any, instance any, path string) error {
	switch s := schema.(type) {
	case bool:
		if s {
			return nil
		}
		return &invalid{path: path, msg: "schema is false"}
	case map[string]any:
		return cat.checkMap(file, s, instance, path)
	default:
		return fmt.Errorf("schema at %s is %T", path, schema)
	}
}

func (cat *catalog) checkMap(file string, schema map[string]any, instance any, path string) error {
	for key := range schema {
		if schemaAnnotations[key] || schemaKeywords[key] {
			continue
		}
		return fmt.Errorf("unknown keyword %q in %s", key, file)
	}
	if ref, ok := schema["$ref"].(string); ok {
		targetFile, target, err := cat.resolve(file, ref)
		if err != nil {
			return err
		}
		if err := cat.check(targetFile, target, instance, path); err != nil {
			return err
		}
	}
	if err := cat.checkType(schema, instance, path); err != nil {
		return err
	}
	if value, ok := schema["const"]; ok && !sameValue(instance, value) {
		return &invalid{path: path, msg: "const mismatch"}
	}
	if raw, ok := schema["enum"]; ok {
		items, _ := raw.([]any)
		matched := false
		for _, item := range items {
			if sameValue(instance, item) {
				matched = true
				break
			}
		}
		if !matched {
			return &invalid{path: path, msg: "not in enum"}
		}
	}
	if err := cat.checkString(schema, instance, path); err != nil {
		return err
	}
	if err := cat.checkNumber(schema, instance, path); err != nil {
		return err
	}
	if err := cat.checkArray(file, schema, instance, path); err != nil {
		return err
	}
	if err := cat.checkObject(file, schema, instance, path); err != nil {
		return err
	}
	if raw, ok := schema["allOf"].([]any); ok {
		for _, sub := range raw {
			if err := cat.check(file, sub, instance, path); err != nil {
				return err
			}
		}
	}
	if raw, ok := schema["anyOf"].([]any); ok {
		var first error
		matched := false
		for _, sub := range raw {
			err := cat.check(file, sub, instance, path)
			if err == nil {
				matched = true
				break
			}
			if !isInvalid(err) {
				return err
			}
			if first == nil {
				first = err
			}
		}
		if !matched {
			return &invalid{path: path, msg: "no anyOf matched (" + first.Error() + ")"}
		}
	}
	if raw, ok := schema["oneOf"].([]any); ok {
		matched := 0
		var first error
		for _, sub := range raw {
			err := cat.check(file, sub, instance, path)
			if err == nil {
				matched++
				continue
			}
			if !isInvalid(err) {
				return err
			}
			if first == nil {
				first = err
			}
		}
		if matched != 1 {
			detail := ""
			if first != nil {
				detail = " (" + first.Error() + ")"
			}
			return &invalid{path: path, msg: fmt.Sprintf("oneOf matched %d%s", matched, detail)}
		}
	}
	if ifSchema, ok := schema["if"]; ok {
		err := cat.check(file, ifSchema, instance, path)
		if err == nil {
			if thenSchema, ok := schema["then"]; ok {
				if err := cat.check(file, thenSchema, instance, path); err != nil {
					return err
				}
			}
		} else if !isInvalid(err) {
			return err
		} else if elseSchema, ok := schema["else"]; ok {
			if err := cat.check(file, elseSchema, instance, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cat *catalog) checkType(schema map[string]any, instance any, path string) error {
	raw, ok := schema["type"]
	if !ok {
		return nil
	}
	types := []string{}
	switch v := raw.(type) {
	case string:
		types = []string{v}
	case []any:
		for _, item := range v {
			text, ok := item.(string)
			if !ok {
				return fmt.Errorf("type entry %T", item)
			}
			types = append(types, text)
		}
	default:
		return fmt.Errorf("type is %T", raw)
	}
	for _, typ := range types {
		if typeOK(typ, instance) {
			return nil
		}
	}
	return &invalid{path: path, msg: "type is not " + strings.Join(types, "|")}
}

func typeOK(typ string, instance any) bool {
	switch typ {
	case "object":
		_, ok := instance.(map[string]any)
		return ok
	case "array":
		_, ok := instance.([]any)
		return ok
	case "string":
		_, ok := instance.(string)
		return ok
	case "boolean":
		_, ok := instance.(bool)
		return ok
	case "null":
		return instance == nil
	case "number":
		_, ok := asFloat(instance)
		return ok
	case "integer":
		return isInteger(instance)
	default:
		return false
	}
}

func isInteger(value any) bool {
	n, ok := asFloat(value)
	if !ok || math.IsNaN(n) || math.IsInf(n, 0) {
		return false
	}
	return n == math.Trunc(n)
}

func asFloat(value any) (float64, bool) {
	switch n := value.(type) {
	case float64:
		return n, true
	case int:
		return float64(n), true
	case int64:
		return float64(n), true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

func (cat *catalog) checkString(schema map[string]any, instance any, path string) error {
	_, stringRule := schema["minLength"]
	_, stringRule2 := schema["maxLength"]
	_, stringRule3 := schema["pattern"]
	_, stringRule4 := schema["format"]
	if !stringRule && !stringRule2 && !stringRule3 && !stringRule4 {
		return nil
	}
	text, ok := instance.(string)
	if !ok {
		return &invalid{path: path, msg: "expected a string"}
	}
	count := utf8.RuneCountInString(text)
	if raw, ok := schema["minLength"]; ok {
		n, _ := asFloat(raw)
		if count < int(n) {
			return &invalid{path: path, msg: "shorter than minLength"}
		}
	}
	if raw, ok := schema["maxLength"]; ok {
		n, _ := asFloat(raw)
		if count > int(n) {
			return &invalid{path: path, msg: "longer than maxLength"}
		}
	}
	if expr, ok := schema["pattern"].(string); ok {
		match, err := cat.matcher(expr)
		if err != nil {
			return err
		}
		if !match(text) {
			return &invalid{path: path, msg: "pattern mismatch"}
		}
	}
	if format, ok := schema["format"].(string); ok {
		switch format {
		case "date-time":
			if _, err := time.Parse(time.RFC3339Nano, text); err != nil {
				return &invalid{path: path, msg: "not a date-time"}
			}
		case "uri":
			parsed, err := url.Parse(text)
			if err != nil || parsed.Scheme == "" || parsed.Host == "" {
				return &invalid{path: path, msg: "not a uri"}
			}
		default:
			return fmt.Errorf("unknown format %s", format)
		}
	}
	return nil
}

func (cat *catalog) matcher(expr string) (func(string) bool, error) {
	if strings.Contains(expr, "(?!") || strings.Contains(expr, "(?=") {
		if strings.HasPrefix(expr, "^P") {
			return isoDuration, nil
		}
		return nil, fmt.Errorf("unsupported lookahead in %s", expr)
	}
	if re, ok := cat.patterns[expr]; ok {
		return re.MatchString, nil
	}
	re, err := regexp.Compile(expr)
	if err != nil {
		return nil, err
	}
	cat.patterns[expr] = re
	return re.MatchString, nil
}

func (cat *catalog) checkNumber(schema map[string]any, instance any, path string) error {
	_, hasMin := schema["minimum"]
	_, hasMax := schema["maximum"]
	if !hasMin && !hasMax {
		return nil
	}
	n, ok := asFloat(instance)
	if !ok {
		return &invalid{path: path, msg: "expected a number"}
	}
	if raw, ok := schema["minimum"]; ok {
		bound, _ := asFloat(raw)
		if n < bound {
			return &invalid{path: path, msg: "below minimum"}
		}
	}
	if raw, ok := schema["maximum"]; ok {
		bound, _ := asFloat(raw)
		if n > bound {
			return &invalid{path: path, msg: "above maximum"}
		}
	}
	return nil
}

func (cat *catalog) checkArray(file string, schema map[string]any, instance any, path string) error {
	_, hasMin := schema["minItems"]
	_, hasMax := schema["maxItems"]
	_, hasUnique := schema["uniqueItems"]
	_, hasItems := schema["items"]
	if !hasMin && !hasMax && !hasUnique && !hasItems {
		return nil
	}
	items, ok := instance.([]any)
	if !ok {
		return &invalid{path: path, msg: "expected an array"}
	}
	if raw, ok := schema["minItems"]; ok {
		n, _ := asFloat(raw)
		if len(items) < int(n) {
			return &invalid{path: path, msg: "fewer than minItems"}
		}
	}
	if raw, ok := schema["maxItems"]; ok {
		n, _ := asFloat(raw)
		if len(items) > int(n) {
			return &invalid{path: path, msg: "more than maxItems"}
		}
	}
	if unique, _ := schema["uniqueItems"].(bool); unique {
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if sameValue(items[i], items[j]) {
					return &invalid{path: path, msg: "items are not unique"}
				}
			}
		}
	}
	if itemSchema, ok := schema["items"]; ok {
		for i, item := range items {
			if err := cat.check(file, itemSchema, item, path+"/"+strconv.Itoa(i)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cat *catalog) checkObject(file string, schema map[string]any, instance any, path string) error {
	_, hasReq := schema["required"]
	_, hasProps := schema["properties"]
	_, hasAdd := schema["additionalProperties"]
	_, hasNames := schema["propertyNames"]
	_, hasMin := schema["minProperties"]
	_, hasMax := schema["maxProperties"]
	if !hasReq && !hasProps && !hasAdd && !hasNames && !hasMin && !hasMax {
		return nil
	}
	obj, ok := instance.(map[string]any)
	if !ok {
		return &invalid{path: path, msg: "expected an object"}
	}
	if raw, ok := schema["minProperties"]; ok {
		n, _ := asFloat(raw)
		if len(obj) < int(n) {
			return &invalid{path: path, msg: "fewer than minProperties"}
		}
	}
	if raw, ok := schema["maxProperties"]; ok {
		n, _ := asFloat(raw)
		if len(obj) > int(n) {
			return &invalid{path: path, msg: "more than maxProperties"}
		}
	}
	props, _ := schema["properties"].(map[string]any)
	if raw, ok := schema["required"].([]any); ok {
		for _, item := range raw {
			name, _ := item.(string)
			if _, present := obj[name]; !present {
				return &invalid{path: joinPointer(path, name), msg: "required"}
			}
		}
	}
	for name, sub := range props {
		value, present := obj[name]
		if !present {
			continue
		}
		if err := cat.check(file, sub, value, joinPointer(path, name)); err != nil {
			return err
		}
	}
	if add, ok := schema["additionalProperties"]; ok {
		for name, value := range obj {
			if _, known := props[name]; known {
				continue
			}
			switch rule := add.(type) {
			case bool:
				if !rule {
					return &invalid{path: joinPointer(path, name), msg: "additional property"}
				}
			default:
				if err := cat.check(file, rule, value, joinPointer(path, name)); err != nil {
					return err
				}
			}
		}
	}
	if names, ok := schema["propertyNames"]; ok {
		for name := range obj {
			if err := cat.check(file, names, name, joinPointer(path, name)); err != nil {
				return err
			}
		}
	}
	return nil
}

func (cat *catalog) resolve(baseFile, ref string) (string, any, error) {
	filePart, frag, _ := strings.Cut(ref, "#")
	target := baseFile
	if filePart != "" {
		if strings.Contains(filePart, "://") {
			doc, ok := cat.byID[filePart]
			if !ok {
				return "", nil, fmt.Errorf("unknown schema id %s", filePart)
			}
			target = doc.path
		} else {
			joined := filepath.Join(filepath.Dir(baseFile), filepath.FromSlash(filePart))
			abs, err := filepath.Abs(joined)
			if err != nil {
				return "", nil, err
			}
			target = abs
		}
	}
	doc, ok := cat.byPath[target]
	if !ok {
		return "", nil, fmt.Errorf("schema file not loaded: %s", target)
	}
	node, err := walkPointer(doc.root, frag)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w", ref, err)
	}
	return target, node, nil
}

func walkPointer(node any, frag string) (any, error) {
	if frag == "" {
		return node, nil
	}
	if !strings.HasPrefix(frag, "/") {
		return nil, fmt.Errorf("bad pointer %q", frag)
	}
	for _, raw := range strings.Split(frag, "/")[1:] {
		token := strings.ReplaceAll(raw, "~1", "/")
		token = strings.ReplaceAll(token, "~0", "~")
		switch cur := node.(type) {
		case map[string]any:
			next, ok := cur[token]
			if !ok {
				return nil, fmt.Errorf("missing %s", token)
			}
			node = next
		case []any:
			index, err := strconv.Atoi(token)
			if err != nil || index < 0 || index >= len(cur) {
				return nil, fmt.Errorf("bad index %s", token)
			}
			node = cur[index]
		default:
			return nil, fmt.Errorf("cannot walk into %T", node)
		}
	}
	return node, nil
}

func joinPointer(path, name string) string {
	return path + "/" + strings.NewReplacer("~", "~0", "/", "~1").Replace(name)
}

func sameValue(a, b any) bool {
	af, aok := asFloat(a)
	bf, bok := asFloat(b)
	if aok || bok {
		return aok && bok && af == bf
	}
	rawA, errA := json.Marshal(a)
	rawB, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(rawA) == string(rawB)
}

func isInvalid(err error) bool {
	var target *invalid
	return errors.As(err, &target)
}

// isoDuration accepts the ORP duration pattern. Go's RE2 cannot compile the
// lookaheads in that pattern.
func isoDuration(s string) bool {
	if len(s) < 2 || s[0] != 'P' {
		return false
	}
	body := s[1:]
	date, timePart, hasT := strings.Cut(body, "T")
	if !consumeOrdered(date, "YMWD") {
		return false
	}
	if !hasT {
		return date != ""
	}
	if timePart == "" || timePart[0] < '0' || timePart[0] > '9' {
		return false
	}
	return consumeOrdered(timePart, "HMS")
}

func consumeOrdered(s, units string) bool {
	i := 0
	for _, unit := range units {
		if i >= len(s) || s[i] < '0' || s[i] > '9' {
			continue
		}
		j := i
		for j < len(s) && s[j] >= '0' && s[j] <= '9' {
			j++
		}
		if j < len(s) && rune(s[j]) == unit {
			i = j + 1
		}
	}
	return i == len(s)
}
