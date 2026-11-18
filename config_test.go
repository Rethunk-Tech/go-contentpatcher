package contentpatcher

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"testing"
)

// A config.json array is stored as one comma-separated value; nested objects are not values.
func TestConfigJsonListsJoinAndObjectsAreSkipped(t *testing.T) {
	t.Parallel()
	got, ok := ConfigValue(json.RawMessage(`["Forest", true, 2]`))
	if !ok || got != "Forest, true, 2" {
		t.Fatalf("list = %q, ok=%v", got, ok)
	}
	if _, ok := ConfigValue(json.RawMessage(`["Forest", {"a":1}]`)); ok {
		t.Fatal("a nested object was accepted as a config value")
	}
	if _, ok := ConfigValue(json.RawMessage(`{"Theme":"Dark"}`)); ok {
		t.Fatal("an object was accepted as a config value")
	}

	dir := t.TempDir()
	writeAt(t, filepath.Join(dir, "config.json"), []byte(`{"Theme":"Dark","Skip":{"a":1},"also":["A","B"]}`))
	cfg := ReadPackConfig(dir)
	if cfg["theme"] != "Dark" || cfg["also"] != "A, B" || len(cfg) != 2 {
		t.Fatalf("config = %#v", cfg)
	}
	writeAt(t, filepath.Join(dir, "config.json"), []byte(`{not json`))
	if got := ReadPackConfig(dir); len(got) != 0 {
		t.Fatalf("invalid JSON = %#v, want empty", got)
	}
}

// AllowMultiple matches any one of the current comma-separated values; otherwise the whole string must match.
func TestAMultiSelectConfigMatchesAnyOfItsCurrentValues(t *testing.T) {
	t.Parallel()
	multi := Config{Values: []string{"Forest"}, AllowMultiple: true}
	if !SettingValueMatches("Standard, Forest", multi) {
		t.Fatal("Forest among current values should match")
	}
	if SettingValueMatches("Standard, Beach", multi) {
		t.Fatal("a multi-select matched a value it does not list")
	}
	single := Config{Values: []string{"forest"}}
	if !SettingValueMatches(" Forest ", single) {
		t.Fatal("case and surrounding space should not block a match")
	}
	if SettingValueMatches("Standard, Forest", single) {
		t.Fatal("a single-choice field matched a comma-separated list as a whole")
	}
}

// A suggested setting is appended once, ignoring case, and blanks are not listed.
func TestASuggestedSettingIsListedOnceIgnoringCase(t *testing.T) {
	t.Parallel()
	var values []string
	AddSettingValue(&values, " Forest ")
	AddSettingValue(&values, "forest")
	AddSettingValue(&values, "  ")
	AddSettingValue(&values, "Beach")
	if !slices.Equal(values, []string{"Forest", "Beach"}) {
		t.Fatalf("values = %#v", values)
	}
}

// A toggle is a single-choice field; a boolean toggle is only the on/off pair (or one of those words).
func TestAToggleIsSingleChoiceAndABooleanToggleIsOnOrOff(t *testing.T) {
	t.Parallel()
	named := Schema{AllowValues: []string{"Standard", "Forest"}}
	if !named.Toggle() || named.BooleanToggle() {
		t.Fatalf("named farms Toggle=%v BooleanToggle=%v", named.Toggle(), named.BooleanToggle())
	}
	onOff := Schema{AllowValues: []string{"true", "false"}}
	if !onOff.Toggle() || !onOff.BooleanToggle() {
		t.Fatalf("true/false Toggle=%v BooleanToggle=%v", onOff.Toggle(), onOff.BooleanToggle())
	}
	multi := Schema{AllowValues: []string{"true", "false"}, AllowMultiple: true}
	if multi.Toggle() || multi.BooleanToggle() {
		t.Fatal("a multi-select field was treated as a toggle")
	}
	defOn := Schema{DefaultValue: "enabled"}
	if !defOn.Toggle() || !defOn.BooleanToggle() {
		t.Fatalf("default enabled Toggle=%v BooleanToggle=%v", defOn.Toggle(), defOn.BooleanToggle())
	}
	seasonal := Schema{DefaultValue: "seasonal"}
	if seasonal.Toggle() || seasonal.BooleanToggle() {
		t.Fatal("a free-text default was treated as a toggle")
	}
	three := Schema{AllowValues: []string{"on", "off", "auto"}}
	if !three.Toggle() || three.BooleanToggle() {
		t.Fatalf("three named values Toggle=%v BooleanToggle=%v", three.Toggle(), three.BooleanToggle())
	}
}

// A patch whose config condition fails is off; an unknown field is ignored, and a missing config.json uses the schema default.
func TestAPatchIsOffWhenThePlayersSettingDoesNotMatch(t *testing.T) {
	t.Parallel()
	schema := map[string]Schema{"theme": {Key: "Theme", DefaultValue: "Light"}}
	wantDark := []Config{{Field: "Theme", Values: []string{"Dark"}}}
	if !ConfigHolds(wantDark, schema, map[string]string{"theme": "Dark"}) {
		t.Fatal("configured Dark should hold")
	}
	if ConfigHolds(wantDark, schema, nil) {
		t.Fatal("schema default Light matched a Dark-only condition")
	}
	if !ConfigHolds([]Config{{Field: "Missing", Values: []string{"x"}}}, schema, nil) {
		t.Fatal("a condition on a field the schema does not define was treated as a miss")
	}
}
