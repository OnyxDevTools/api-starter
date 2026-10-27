// cmd/schema/main.go
// Explicit schema tooling. The API server never imports or calls this command.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"time"
)

type object = map[string]any

func readSchema(path string) (object, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var schema object
	if err = json.Unmarshal(data, &schema); err != nil {
		return nil, err
	}
	return schema, nil
}
func tableList(schema object) ([]any, error) {
	value, ok := schema["tables"]
	if !ok {
		value = schema["entities"]
	}
	tables, ok := value.([]any)
	if !ok {
		return nil, errors.New("schema must contain a tables array")
	}
	if legacy, exists := schema["entities"]; exists && schema["tables"] != nil && !reflect.DeepEqual(legacy, tables) {
		return nil, errors.New("schema aliases disagree")
	}
	return tables, nil
}
func validate(schema object) error {
	tables, err := tableList(schema)
	if err != nil {
		return err
	}
	if len(tables) == 0 {
		return errors.New("local schema has no tables")
	}
	names := map[string]bool{}
	for _, value := range tables {
		table, ok := value.(object)
		if !ok {
			return errors.New("invalid table")
		}
		name, _ := table["name"].(string)
		if name == "" || names[name] {
			return errors.New("table names must be unique and nonblank")
		}
		names[name] = true
		attributes, ok := table["attributes"].([]any)
		if !ok || len(attributes) == 0 {
			return errors.New("table must define attributes")
		}
		fields := map[string]bool{}
		for _, v := range attributes {
			attribute, ok := v.(object)
			if !ok {
				return errors.New("invalid attribute")
			}
			field, _ := attribute["name"].(string)
			typ, _ := attribute["type"].(string)
			if field == "" || typ == "" || fields[field] {
				return errors.New("attribute names must be unique and types nonblank")
			}
			fields[field] = true
		}
		id, ok := table["identifier"].(object)
		if !ok {
			return errors.New("table must define an identifier")
		}
		idName, _ := id["name"].(string)
		if !fields[idName] {
			return errors.New("identifier must name an attribute")
		}
	}
	return nil
}

// merge only adds missing tables, preserving the full remote definitions.
// Existing starter tables must be compatible; mutations require manual review.
func merge(remote, local object) (object, error) {
	tables, err := tableList(remote)
	if err != nil {
		return nil, err
	}
	additions, err := tableList(local)
	if err != nil {
		return nil, err
	}
	existing := map[string]object{}
	for _, value := range tables {
		table, ok := value.(object)
		if !ok {
			return nil, errors.New("invalid remote table")
		}
		name, _ := table["name"].(string)
		existing[name] = table
	}
	for _, value := range additions {
		table := value.(object)
		name := table["name"].(string)
		if current, ok := existing[name]; ok {
			if err = compatible(current, table); err != nil {
				return nil, fmt.Errorf("existing %s is incompatible: %w", name, err)
			}
		} else {
			tables = append(tables, table)
		}
	}
	remote["tables"] = tables
	if _, ok := remote["entities"]; ok {
		remote["entities"] = tables
	}
	return remote, nil
}
func compatible(remote, local object) error {
	if !reflect.DeepEqual(remote["identifier"], local["identifier"]) {
		return errors.New("identifier differs; review manually")
	}
	raw, ok := remote["attributes"].([]any)
	if !ok {
		return errors.New("missing attributes")
	}
	fields := map[string]object{}
	for _, value := range raw {
		field, ok := value.(object)
		if !ok {
			return errors.New("invalid attribute")
		}
		name, _ := field["name"].(string)
		fields[name] = field
	}
	for _, value := range local["attributes"].([]any) {
		field := value.(object)
		name := field["name"].(string)
		current := fields[name]
		if current == nil || current["type"] != field["type"] || current["isNullable"] == true {
			return errors.New("required attribute differs; review manually")
		}
		delete(fields, name)
	}
	for _, field := range fields {
		if field["isNullable"] != true {
			return errors.New("extra required attribute; review manually")
		}
	}
	return nil
}
func cli(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, filepath.Join(".tools", "onyx"), args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("onyx CLI failed; check configuration and run the matching command directly for diagnostics")
	}
	return output, nil
}
func run(ctx context.Context, operation string) error {
	local, err := readSchema("api/onyx.schema.json")
	if err != nil {
		return err
	}
	if err = validate(local); err != nil {
		return err
	}
	switch operation {
	case "validate":
		fmt.Println("Local schema is valid.")
		return nil
	case "generate":
		// CLI 0.11.14 resolves config for file-only generation. These non-secret values
		// satisfy its resolver; this command does not contact a database.
		_, err = cli(ctx, "gen", "--go", "--schema", "api/onyx.schema.json", "--out", "internal/gen/onyx", "--package", "onyx", "--database-id", "offline-codegen", "--api-key", "offline-codegen", "--api-secret", "offline-codegen")
		if err != nil {
			return err
		}
		return exec.CommandContext(ctx, "gofmt", "-w", "internal/gen/onyx").Run()
	case "diff", "validate-remote", "publish":
	default:
		return errors.New("usage: go run ./cmd/schema validate|generate|diff|validate-remote|publish")
	}
	database := os.Getenv("ONYX_DATABASE_ID")
	if database == "" {
		return errors.New("set ONYX_DATABASE_ID explicitly to the intended development database")
	}
	flags := []string{"--database-id", database}
	if path := os.Getenv("ONYX_CONFIG_PATH"); path != "" {
		flags = append(flags, "--config", path)
	}
	dir, err := os.MkdirTemp("", "api-starter-schema-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	current := filepath.Join(dir, "remote.json")
	if _, err = cli(ctx, append([]string{"schema", "get", "--out", current}, flags...)...); err != nil {
		return err
	}
	remote, err := readSchema(current)
	if err != nil {
		return err
	}
	merged, err := merge(remote, local)
	if err != nil {
		return err
	}
	data, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	target := filepath.Join(dir, "merged.json")
	if err = os.WriteFile(target, data, 0600); err != nil {
		return err
	}
	command := func(action string) ([]byte, error) {
		return cli(ctx, append([]string{"schema", action, "--schema", target}, flags...)...)
	}
	if operation == "diff" {
		output, e := command("diff")
		if e != nil {
			return e
		}
		fmt.Print(string(output))
		return nil
	}
	if _, err = command("validate"); err != nil {
		return err
	}
	if operation == "publish" {
		if _, err = command("publish"); err != nil {
			return err
		}
	}
	fmt.Println("Schema operation completed.")
	return nil
}
func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: schema validate|generate|diff|validate-remote|publish")
		os.Exit(1)
	}
	if err := run(ctx, os.Args[1]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
