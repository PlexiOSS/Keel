// Package genconfig provides functionality to auto-generate a sample YAML
// configuration file (`config.yaml.sample`) from a Go struct using struct tags.
package genconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
	"strings"
)

// SampleFileName defines the default name for the generated sample config file.
var SampleFileName = "config.yaml.sample"

// simpleYamlParser generates YAML from a struct based on `yaml`, `default`, `comment`, and `required` struct tags.
type simpleYamlParser struct {
	indent        int         // Current indentation level (used for formatting)
	originalValue any         // Root struct value
	currVal       any         // Current value being processed (useful for nested structs/maps)
	defaultOnly   bool        // Whether to use only the default values instead of actual field values
}

// GenConfig generates a sample config file from the given struct.
// The struct fields must be tagged with `yaml`, and optionally `default`, `comment`, and `required`.
func GenConfig(cfg any) {
	parser := simpleYamlParser{defaultOnly: true}

	// Remove existing sample config file if it exists
	if _, err := os.Stat(SampleFileName); err == nil {
		if err := os.Remove(SampleFileName); err != nil {
			panic(err)
		}
	}

	// Create a new sample config file
	file, err := os.Create(SampleFileName)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Write parsed YAML content to the file
	if _, err := file.WriteString(parser.Parse(cfg)); err != nil {
		panic(err)
	}
}

// Parse generates a YAML-formatted string from the given struct.
func (p *simpleYamlParser) Parse(v any) string {
	p.originalValue = v
	var sb strings.Builder

	t := reflect.TypeOf(v)
	for _, field := range reflect.VisibleFields(t) {
		sb.WriteString(p.processField(field))
	}

	return strings.TrimSpace(sb.String())
}

// processField processes a struct field and returns its YAML representation.
func (p *simpleYamlParser) processField(field reflect.StructField) string {
	kind := field.Type.Kind()
	tagName := field.Tag.Get("yaml")

	// Reject pointer types
	if kind == reflect.Ptr {
		panic(fmt.Sprintf("pointer type %s not supported", field.Name))
	}

	// Base indentation
	indent := strings.Repeat("  ", p.indent)
	result := ""

	switch kind {
	case reflect.Struct:
		result += fmt.Sprintf("%s%s:\n", indent, tagName)
		p.indent++
		curr := p.currVal
		p.currVal = p.getFieldValue(field.Name).Interface()
		for i := 0; i < field.Type.NumField(); i++ {
			result += p.processField(field.Type.Field(i))
		}
		p.currVal = curr
		p.indent--
		result += "\n"

	case reflect.Map:
		result += fmt.Sprintf("%s%s:\n", indent, tagName)
		mapVal := p.getFieldValue(field.Name)
		mapKeys := mapVal.MapKeys()

		for _, key := range mapKeys {
			val := mapVal.MapIndex(key).Interface()
			result += fmt.Sprintf("%s  %s:\n", indent, key.String())
			p.indent += 2
			curr := p.currVal
			p.currVal = val
			structType := reflect.TypeOf(val)
			for i := 0; i < structType.NumField(); i++ {
				result += p.processField(structType.Field(i))
			}
			p.currVal = curr
			p.indent -= 2
		}
		result += "\n"

	case reflect.Slice:
		comment := field.Tag.Get("comment")
		defaultTag := field.Tag.Get("default")
		fieldValue := p.getFieldValue(field.Name).Interface()
		var items []string

		// Convert value to []string
		var strValues []string
		data, _ := json.Marshal(fieldValue)
		_ = json.Unmarshal(data, &strValues)

		if len(strValues) == 0 || p.defaultOnly {
			items = strings.Split(defaultTag, ",")
		} else {
			for _, val := range strValues {
				if comment != "" {
					items = append(items, fmt.Sprintf("%s # %s", val, comment))
				} else {
					items = append(items, val)
				}
			}
		}

		result += fmt.Sprintf("%s%s:\n", indent, tagName)
		p.indent++
		for _, item := range items {
			result += fmt.Sprintf("%s- %s\n", strings.Repeat("  ", p.indent), strings.TrimSpace(item))
		}
		p.indent--

	default: // Primitives
		value := fmt.Sprintf("%v", p.getFieldValue(field.Name))
		if p.defaultOnly || strings.Contains(value, "<invalid reflect.Value>") {
			value = field.Tag.Get("default")
		}
		comment := field.Tag.Get("comment")
		required := field.Tag.Get("required")
		if required == "" {
			required = "true"
		}

		result += fmt.Sprintf("%s%s: %s", indent, tagName, value)

		if comment != "" {
			result += " # " + comment
		}
		if required == "false" {
			if comment != "" {
				result += " (optional)"
			} else {
				result += " # (optional)"
			}
		}
		result += "\n"
	}

	return result
}

// getFieldValue returns the reflect.Value for a given field name from the current struct being processed.
// Uses reflection safely with recover.
func (p *simpleYamlParser) getFieldValue(field string) reflect.Value {
	defer func() {
		_ = recover() // Ignore panics if a field doesn't exist
	}()
	if p.currVal != nil {
		return reflect.ValueOf(p.currVal).FieldByName(field)
	}
	return reflect.ValueOf(p.originalValue).FieldByName(field)
}
