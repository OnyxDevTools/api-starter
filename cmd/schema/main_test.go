// cmd/schema/main_test.go
package main

import (
	"encoding/json"
	"reflect"
	"testing"
)

func schema(t *testing.T, raw string) object {
	t.Helper()
	var v object
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func localSchema(t *testing.T) object {
	t.Helper()
	v, err := readSchema("../../api/onyx.schema.json")
	if err != nil {
		t.Fatal(err)
	}
	return v
}
func TestSchemaMergePreservesExistingTables(t *testing.T) {
	for _, key := range []string{"tables", "entities"} {
		remote := schema(t, `{"`+key+`":[{"name":"Existing","custom":"preserved"}],"metadata":{"revision":"keep"}}`)
		before := remote[key].([]any)[0]
		merged, err := merge(remote, localSchema(t))
		if err != nil {
			t.Fatal(err)
		}
		tables, err := tableList(merged)
		if err != nil {
			t.Fatal(err)
		}
		if len(tables) != 2 || !reflect.DeepEqual(tables[0], before) || merged["metadata"] == nil {
			t.Fatal("existing schema changed")
		}
		if key == "entities" && !reflect.DeepEqual(merged[key], merged["tables"]) {
			t.Fatal("aliases differ")
		}
		if _, err = merge(merged, localSchema(t)); err != nil {
			t.Fatal(err)
		}
	}
}
func TestRefuseIncompatibleOrIncompleteSchema(t *testing.T) {
	local := localSchema(t)
	for _, remote := range []object{schema(t, `{}`), schema(t, `{"tables":[],"entities":[{"name":"Other"}]}`), schema(t, `{"tables":[{"name":"Task","attributes":[]}]}`)} {
		if _, err := merge(remote, local); err == nil {
			t.Fatal("unsafe schema accepted")
		}
	}
}
func TestValidateLocalSchema(t *testing.T) {
	if err := validate(localSchema(t)); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{`{"tables":[]}`, `{"tables":[{"name":"Task"}]}`, `{"tables":[{}]}`} {
		if err := validate(schema(t, raw)); err == nil {
			t.Fatal("invalid schema accepted")
		}
	}
}
