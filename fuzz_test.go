package contentpatcher

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

// FuzzContentPack feeds a pack's content.json and one file it can name (as an Include, a Load or EditData source, a
// map or an image), with a decoy pack beside it. Scanning must not panic, read the decoy, or stamp a file outside
// the pack, and what it finds must survive the disk cache.
func FuzzContentPack(f *testing.F) {
	for _, s := range [][2]string{
		{
			`{"Format":"2.0.0","ConfigSchema":{"Mode":{"AllowValues":"A, B","Default":"A"}},"DynamicTokens":[{"Name":"T","Value":"{{Mode}}","When":{"HasMod":"X.Y"}}],` +
				`"Changes":[{"Action":"Include","FromFile":"data.json"},{"Action":"Load","Target":"Maps/A","FromFile":"data.tmx","When":{"Mode":"A"}},` +
				`{"Action":"EditMap","Target":"Maps/B","FromFile":"data.tmj","FromArea":{"X":0,"Y":0,"Width":2,"Height":2},"ToArea":{"X":1,"Y":1,"Width":2,"Height":2}},` +
				`{"Action":"EditImage","Target":"TileSheets/x","FromFile":"data.png","PatchMode":"Overlay"},{"Action":"EditData","Target":"Data/Objects","Entries":{"1":"{{T}}"},"Priority":"Late"}]}`,
			`{"Changes":[{"Action":"EditData","Target":"Data/Crops","Fields":{"1":{"2":"x"}}}]}`,
		},
		{`{"Changes":[{"Action":"Include","FromFile":"../decoy/content.json"},{"Action":"Include","FromFile":"/etc/passwd"},{"Action":"Load","Target":"A, B","FromFile":"{{Target}}.json"}]}`, `{}`},
		{`{"Changes":[{"Action":"EditMap","Target":"Maps/T","FromFile":"data.tbin"}]}`, "tBIN10\x00\x00\x00\x00"},
		{`{"Changes":[{"Action":"EditImage","Target":"TileSheets/x","FromFile":"data.png","PatchMode":"Overlay"}]}`, "\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x75\x30\x00\x00\x75\x30\x08\x02\x00\x00\x00\x00\x00\x00\x00"},
		{`{"Changes":[{"Action":"Load","Target":"Maps/T","FromFile":"data.tmx"}]}`, `<map><tileset><image source="../Maps/spring_town.png"/></tileset></map>`},
		{`// comment` + "\n" + `{"Changes":[{"Action":"Include","FromFile":"data.json, content.json"},],}`, `{"Changes":[{"Action":"Include","FromFile":"content.json"}]}`},
	} {
		f.Add([]byte(s[0]), []byte(s[1]))
	}
	f.Fuzz(func(t *testing.T, content, data []byte) {
		parent := t.TempDir()
		root, decoy := filepath.Join(parent, "pack"), filepath.Join(parent, "decoy")
		files := map[string][]byte{
			filepath.Join(root, "content.json"):  content,
			filepath.Join(decoy, "content.json"): []byte(`{"Changes":[{"Action":"Load","Target":"Decoy/Leak","FromFile":"x.json"}]}`),
		}
		for _, name := range []string{"data.json", "data.tmj", "data.tmx", "data.tbin", "data.png"} {
			files[filepath.Join(root, name)] = data
		}
		for p, b := range files {
			writeAt(t, p, b)
		}
		pack := Pack{Mentions: map[string]bool{}}
		pack.Schema = readConfigSchema(root)
		pack.Values = packConfigValues(pack.Schema, root)
		scanContentFile(root, "content.json", map[string]bool{}, When{}, &pack)
		for _, p := range pack.Patches {
			if strings.Contains(strings.ToLower(p.Target), "decoy/leak") {
				t.Fatalf("scanned a file outside the pack: %+v", p)
			}
		}
		for _, s := range pack.Files {
			if filepath.IsAbs(s.Path) || s.Path == ".." || strings.HasPrefix(filepath.ToSlash(s.Path), "../") {
				t.Fatalf("stamped %q outside the pack", s.Path)
			}
		}
		raw, err := encodeDiskPack(diskPackOf(pack))
		if err != nil {
			t.Fatalf("pack does not encode: %v", err)
		}
		if _, ok := decodeDiskPack(raw); !ok {
			t.Fatal("encoded pack does not decode")
		}
	})
}

// Seeded content.json fragments from the shapes this parser accepts.
var fuzzContentSeeds = [][]byte{
	[]byte(`{"Format":"2.8.0","Changes":[]}`),
	[]byte(`{"Format":"1.29.0","Changes":[{"Action":"Load","Target":"Maps/Town","FromFile":"assets/town.tmx"}]}`),
	[]byte(`{"Format":"2.0.0","ConfigSchema":{"Palette":{"AllowValues":"A, B","Default":"A"}},"Changes":[{"Action":"EditImage","Target":"Characters/Farmer","FromFile":"farmer.png","When":{"Palette":"B"}}]}`),
	[]byte(`{"DynamicTokens":[{"Name":"SeasonTile","Value":"spring"}],"Changes":[{"Action":"EditData","Target":"Data/Objects","Entries":{"128":{"Name":"Pufferfish"}}}]}`),
	[]byte(`{"Changes":[{"Action":"Include","FromFile":"patches.json","When":{"HasMod":"Some.Mod"}}]}`),
	[]byte(`{"Changes":[{"Action":"EditMap","Target":"Maps/Farm","FromFile":"farm.tmx","FromArea":{"X":0,"Y":0,"Width":16,"Height":16},"ToArea":{"X":0,"Y":0,"Width":16,"Height":16}}]}`),
	[]byte(`{"Changes":[{"Action":"Load","Target":"{{Season}}/{{Target}}","FromFile":"{{TargetWithoutPath}}.png","Priority":"Low"}]}`),
	[]byte(`{"Changes":[{"Action":"EditData","Target":"Data/AdditionalFarms","Entries":{"Custom":{"ID":"MyFarm","MapName":"CustomFarm"}}}]}`),
	[]byte(`{Format: "2.0.0", Changes: [{Action: "Load", Target: "Data/Objects", FromFile: "objects.json"}]}`),
	[]byte(`{"Changes":[{"Action":"EditImage","Target":"TileSheets/furniture","FromFile":"a.png, b.png","PatchMode":"Overlay"}]}`),
}

func FuzzReadPack(f *testing.F) {
	for _, seed := range fuzzContentSeeds {
		f.Add(seed, []byte(`{}`))
	}
	f.Add([]byte(`{"Format":"2.8.0","Changes":[]}`), []byte(`{"Palette":"B"}`))
	entries, _ := os.ReadDir("testdata")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		content, err := os.ReadFile(filepath.Join("testdata", entry.Name(), "content.json"))
		if err != nil {
			continue
		}
		config, _ := os.ReadFile(filepath.Join("testdata", entry.Name(), "config.json"))
		f.Add(content, config)
	}
	f.Fuzz(func(t *testing.T, content, config []byte) {
		if len(content) > 1<<16 || len(config) > 1<<16 {
			t.Skip()
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "content.json"), content, 0o600); err != nil {
			t.Skip()
		}
		if len(config) > 0 {
			_ = os.WriteFile(filepath.Join(dir, "config.json"), config, 0o600)
		}
		mod := Mod{
			Key:            "fuzz",
			Name:           "Fuzz",
			UniqueID:       SMAPI("Fuzz.Pack"),
			Folder:         dir,
			Enabled:        true,
			ContentPackFor: contentPatcherID,
		}
		pack := ReadPack(mod, true)
		if pack.Patches == nil {
			return
		}
		for _, patch := range pack.Patches {
			_ = patch.When.Holds(map[string]bool{})
			_ = DynamicWhenHolds(patch.When, pack.Tokens, map[string]bool{}, pack.Schema, pack.Values)
		}
	})
}

func FuzzScalarValue(f *testing.F) {
	for _, seed := range []string{`"hello"`, `true`, `false`, `0`, `1.5`, `null`, `[]`, `{}`, `"{{Token}}"`, `"\u0000"`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		got, ok := ScalarValue(raw)
		if !ok {
			return
		}
		var round json.RawMessage
		if json.Unmarshal(raw, &round) != nil {
			t.Fatalf("ScalarValue accepted %q but json.Unmarshal rejected it", raw)
		}
		if !utf8.ValidString(got) {
			t.Fatalf("ScalarValue returned invalid UTF-8 %q for %q", got, raw)
		}
	})
}

func FuzzSplitTargets(f *testing.F) {
	for _, seed := range []string{"", "Maps/Town", "a, b, c", "  x  ", ",", "a,,b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<16 {
			t.Skip()
		}
		parts := SplitTargets(s)
		for _, p := range parts {
			if p == "" || strings.TrimSpace(p) != p {
				t.Fatalf("SplitTargets(%q) returned empty or untrimmed %q", s, p)
			}
		}
		if len(parts) > 0 {
			joined := strings.Join(parts, ",")
			again := SplitTargets(joined)
			if len(again) != len(parts) {
				t.Fatalf("SplitTargets round-trip length %d -> %d for %q", len(parts), len(again), s)
			}
		}
	})
}

func FuzzNormalizeTarget(f *testing.F) {
	for _, seed := range []string{"", "Maps/Town", `Maps\Town`, "MAPS//Town", "  Data/Objects  "} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<16 {
			t.Skip()
		}
		got := NormalizeTarget(s)
		if strings.Contains(got, "\\") || strings.Contains(got, "//") {
			t.Fatalf("NormalizeTarget(%q) = %q still has slashes to fold", s, got)
		}
		if got != strings.ToLower(got) {
			t.Fatalf("NormalizeTarget(%q) = %q is not lower-case", s, got)
		}
	})
}

func FuzzContentReference(f *testing.F) {
	for _, seed := range []string{"", "assets/a.png", `assets\a.png`, "../escape", "a/./b"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, rel string) {
		if len(rel) > 1<<16 {
			t.Skip()
		}
		got := ContentReference("content.json", rel)
		if got == "" {
			return
		}
		if strings.Contains(got, "\\") {
			t.Fatalf("ContentReference(%q) = %q still has backslashes", rel, got)
		}
	})
}

func FuzzResolveLoadFile(f *testing.F) {
	f.Add("assets/{{Target}}.png", "Maps/Town")
	f.Add("{{TargetWithoutPath}}.png", "Maps/Town")
	f.Add("plain.json", "Data/Objects")
	f.Add("{{unknown}}", "Maps/Town")
	f.Add("a{{b}}c{{d}}e", "x")
	f.Fuzz(func(t *testing.T, file, target string) {
		if len(file) > 1<<12 || len(target) > 1<<12 {
			t.Skip()
		}
		got, ok := ResolveLoadFile(file, target, nil, nil, nil, nil)
		if !ok {
			return
		}
		if strings.Contains(got, "{{") {
			t.Fatalf("ResolveLoadFile(%q, %q) returned unresolved %q", file, target, got)
		}
	})
}

func FuzzSMAPI(f *testing.F) {
	for _, seed := range []string{"", "Pathoschild.ContentPatcher", "  Mod.Id  ", "A.B", "smapi:already"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, unique string) {
		if len(unique) > 1<<12 || !utf8.ValidString(unique) {
			t.Skip()
		}
		id := SMAPI(unique)
		if id.Format() != formatSMAPI {
			t.Fatalf("SMAPI(%q).Format() = %q", unique, id.Format())
		}
		folded := id.Fold()
		if folded != SMAPI(unique).Fold() {
			t.Fatalf("Fold not stable for %q", unique)
		}
		if !Equal(id, SMAPI(unique)) {
			t.Fatalf("Equal failed for SMAPI(%q)", unique)
		}
	})
}

func FuzzConfigValue(f *testing.F) {
	for _, seed := range []string{`"hello"`, `true`, `1`, `"a, b"`, `null`, `[]`} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		_, _ = ConfigValue(raw)
	})
}

func FuzzReadPackConfig(f *testing.F) {
	for _, seed := range [][]byte{
		[]byte(`{}`),
		[]byte(`{"Palette":"A"}`),
		[]byte(`{"On":true,"N":1}`),
	} {
		f.Add(seed)
	}
	entries, _ := os.ReadDir("testdata")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if config, err := os.ReadFile(filepath.Join("testdata", entry.Name(), "config.json")); err == nil {
			f.Add(config)
		}
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.json"), raw, 0o600); err != nil {
			t.Skip()
		}
		_ = ReadPackConfig(dir)
	})
}

func FuzzTmxImageSources(f *testing.F) {
	f.Add([]byte(`<map><tileset><image source="a.png"/></tileset></map>`))
	f.Add([]byte(`<?xml version="1.0"?><map><imagelayer><image source="../x.png"/></imagelayer></map>`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		_ = TmxImageSources(raw)
	})
}

func FuzzPrintableRuns(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte{0, 1, 2, 'a', 'b', 0})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		_ = PrintableRuns(raw)
	})
}

func FuzzMapScanFor(f *testing.F) {
	f.Add([]byte(`<map orientation="orthogonal" width="1" height="1" tilewidth="16" tileheight="16"><layer name="Back" width="1" height="1"><data encoding="csv">0</data></layer></map>`), ".tmx")
	f.Add([]byte(`{"width":1,"height":1}`), ".tmj")
	f.Fuzz(func(t *testing.T, raw []byte, ext string) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		switch ext {
		case ".tmx", ".tbin", ".tmj":
		default:
			ext = ".tmx"
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "map"+ext)
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Skip()
		}
		entries, err := os.ReadDir(dir)
		if err != nil || len(entries) == 0 {
			t.Skip()
		}
		_, _ = MapScanFor(path, entries[0], ext)
	})
}

func FuzzNormalizeAsset(f *testing.F) {
	for _, seed := range []string{"", "Maps/Town", `Maps\Town`, "Portraits/Abigail"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<16 {
			t.Skip()
		}
		_ = NormalizeAsset(s)
	})
}

func FuzzCellSetRoundTrip(f *testing.F) {
	for _, seed := range []string{"", "0,0", "1,2;3,4", ";1,2;3,4", "0,0;0,1;1,0;1,1", "32767,32767", "-1,-1;2,2"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, s string) {
		if len(s) > 1<<12 {
			t.Skip()
		}
		encoded := encodeCellSet(s)
		if encoded == nil {
			return
		}
		decoded := decodeCellSet(encoded)
		again := encodeCellSet(decoded)
		if !bytes.Equal(encoded, again) && decodeCellSet(again) != decoded {
			t.Fatalf("cell-set round-trip diverged: %q -> %q", s, decoded)
		}
	})
}
