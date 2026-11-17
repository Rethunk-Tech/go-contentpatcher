package contentpatcher

import (
	"bytes"
	"encoding/json"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/Rethunk-Tech/go-tilemask"
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
		Reset()
		t.Cleanup(Reset)
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
		for _, s := range pack.Files {
			if filepath.IsAbs(s.Path) || s.Path == ".." || strings.HasPrefix(filepath.ToSlash(s.Path), "../") {
				t.Fatalf("stamped %q outside the pack", s.Path)
			}
		}
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
		got, ok := ConfigValue(raw)
		if !ok {
			return
		}
		if !utf8.ValidString(got) {
			t.Fatalf("ConfigValue returned invalid UTF-8 %q for %q", got, raw)
		}
		if scalar, scalarOK := ScalarValue(raw); scalarOK && scalar != got {
			t.Fatalf("ConfigValue(%q) = %q, ScalarValue = %q", raw, got, scalar)
		}
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
		got := ReadPackConfig(dir)
		for k, v := range got {
			if k != strings.ToLower(k) {
				t.Fatalf("ReadPackConfig key %q is not lower-case", k)
			}
			if !utf8.ValidString(v) {
				t.Fatalf("ReadPackConfig[%q] invalid UTF-8 %q", k, v)
			}
		}
	})
}

func FuzzTmxImageSources(f *testing.F) {
	f.Add([]byte(`<map><tileset><image source="a.png"/></tileset></map>`))
	f.Add([]byte(`<?xml version="1.0"?><map><imagelayer><image source="../x.png"/></imagelayer></map>`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		for _, source := range TmxImageSources(raw) {
			if !bytes.Contains(raw, []byte(source)) {
				t.Fatalf("TmxImageSources invented %q from %q", source, raw)
			}
		}
	})
}

func FuzzPrintableRuns(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte{0, 1, 2, 'a', 'b', 0})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		for _, run := range PrintableRuns(raw) {
			if len(run) < 3 || run != strings.ToLower(run) {
				t.Fatalf("PrintableRuns returned %q", run)
			}
			for i := 0; i < len(run); i++ {
				if run[i] < ' ' || run[i] > '~' {
					t.Fatalf("PrintableRuns returned non-printable %q", run)
				}
			}
		}
	})
}

func FuzzMapScanFor(f *testing.F) {
	f.Add([]byte(`<map orientation="orthogonal" width="1" height="1" tilewidth="16" tileheight="16"><layer name="Back" width="1" height="1"><data encoding="csv">0</data></layer></map>`), ".tmx")
	f.Add([]byte(`{"width":1,"height":1}`), ".tmj")
	f.Add([]byte("tBIN10\x00\x00\x00\x00spring_town"), ".tbin")
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
		scan, err := MapScanFor(path, entries[0], ext)
		if err != nil {
			return
		}
		for _, key := range scan.Keys {
			if key != strings.ToLower(key) || strings.Contains(key, "\\") {
				t.Fatalf("MapScanFor key %q", key)
			}
		}
		for _, run := range scan.Runs {
			if run != strings.ToLower(run) {
				t.Fatalf("MapScanFor run %q", run)
			}
		}
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
		got := NormalizeAsset(s)
		if strings.Contains(got, "\\") || strings.HasPrefix(got, "./") || strings.HasPrefix(got, "../") {
			t.Fatalf("NormalizeAsset(%q) = %q still has a prefix to fold", s, got)
		}
		if got != strings.ToLower(got) {
			t.Fatalf("NormalizeAsset(%q) = %q is not lower-case", s, got)
		}
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

// FuzzAnalyze checks a profile of two packs whose content.json files are fuzzed. A conflict names at least two of
// them and a winner among them, and a second check from a clean slate finds the same.
func FuzzAnalyze(f *testing.F) {
	for _, a := range fuzzContentSeeds {
		f.Add(a, []byte(`{"Changes":[{"Action":"Load","Target":"Maps/Town","FromFile":"x.tmx"}]}`))
	}
	f.Add([]byte(`{"Changes":[{"Action":"EditData","Target":"Data/Objects","Entries":{"1":"a"}}]}`),
		[]byte(`{"Changes":[{"Action":"EditData","Target":"Data/Objects","Entries":{"1":"b"},"Priority":"Late"}]}`))
	f.Fuzz(func(t *testing.T, a, b []byte) {
		if len(a) > 1<<16 || len(b) > 1<<16 || !utf8.Valid(a) || !utf8.Valid(b) {
			t.Skip()
		}
		Reset()
		t.Cleanup(Reset)
		mods := []Mod{
			diskPack(t, "Fuzz.A", map[string]string{"manifest.json": packManifest("Fuzz.A"), "content.json": string(a)}),
			diskPack(t, "Fuzz.B", map[string]string{"manifest.json": packManifest("Fuzz.B"), "content.json": string(b)}),
		}
		got := check(mods)
		for _, c := range got.Conflicts {
			if len(c.PackIDs) < 2 {
				t.Fatalf("conflict of one pack: %+v", c)
			}
			for _, id := range c.PackIDs {
				if !Equal(id, mods[0].UniqueID) && !Equal(id, mods[1].UniqueID) {
					t.Fatalf("conflict names %q, not in the profile", id)
				}
			}
			if c.WinnerID != "" && !slices.ContainsFunc(c.PackIDs, func(id ModID) bool { return Equal(id, c.WinnerID) }) {
				t.Fatalf("winner %q is not one of %v", c.WinnerID, c.PackIDs)
			}
		}
		Reset()
		if again := check(mods); !reflect.DeepEqual(again, got) {
			t.Fatalf("a second check differs:\n%+v\n%+v", got, again)
		}
	})
}

func FuzzFoldedRel(f *testing.F) {
	for _, seed := range []string{"", "a.png", "data.json", "../decoy", "/etc/passwd", `assets\a.png`, "a/./b", "..", ".", "a/../../x"} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, rel string) {
		if len(rel) > 1<<12 || !utf8.ValidString(rel) {
			t.Skip()
		}
		parent := t.TempDir()
		root, decoy := filepath.Join(parent, "pack"), filepath.Join(parent, "decoy")
		writeAt(t, filepath.Join(root, "a.png"), []byte("x"))
		writeAt(t, filepath.Join(decoy, "secret"), []byte("leak"))
		got, ok := foldedRel(root, rel)
		if !ok {
			return
		}
		slash := filepath.ToSlash(got)
		if filepath.IsAbs(got) || got == ".." || strings.HasPrefix(slash, "../") {
			t.Fatalf("foldedRel(%q) = %q escaped", rel, got)
		}
		abs := filepath.Join(root, filepath.FromSlash(got))
		relToRoot, err := filepath.Rel(root, abs)
		if err != nil || strings.HasPrefix(relToRoot, "..") {
			t.Fatalf("foldedRel(%q) = %q is not under root", rel, got)
		}
		if strings.Contains(slash, "decoy") {
			t.Fatalf("foldedRel(%q) = %q named the decoy", rel, got)
		}
	})
}

func FuzzDecodeMap(f *testing.F) {
	f.Add([]byte(`<map orientation="orthogonal" width="1" height="1" tilewidth="16" tileheight="16"><layer name="Back" width="1" height="1"><data encoding="csv">0</data></layer></map>`), ".tmx")
	f.Add([]byte(`{"width":1,"height":1,"layers":[{"name":"Back","data":[0]}]}`), ".tmj")
	f.Fuzz(func(t *testing.T, raw []byte, ext string) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		switch ext {
		case ".tmx", ".tmj":
		default:
			ext = ".tmx"
		}
		root := t.TempDir()
		name := "map" + ext
		writeAt(t, filepath.Join(root, name), raw)
		decoded, ok := decodeMap(root, name)
		if !ok {
			return
		}
		if decoded.Width < 0 || decoded.Height < 0 {
			t.Fatalf("decodeMap size %d x %d", decoded.Width, decoded.Height)
		}
		n := 0
		for _, layer := range decoded.Layers {
			if layer.Name != strings.ToLower(strings.TrimSpace(layer.Name)) {
				t.Fatalf("layer name %q not folded", layer.Name)
			}
			n += len(layer.Tiles)
		}
		if int64(n) > tilemask.DefaultMaxTiles {
			t.Fatalf("decodeMap kept %d tiles, cap %d", n, tilemask.DefaultMaxTiles)
		}
	})
}

func FuzzCropImageDataURL(f *testing.F) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, 1, 1))); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes(), 0, 0, 1, 1)
	f.Add([]byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR\x00\x00\x75\x30\x00\x00\x75\x30\x08\x02\x00\x00\x00\x00\x00\x00\x00"), 0, 0, 1, 1)
	f.Fuzz(func(t *testing.T, raw []byte, x, y, w, h int) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		url, err := cropImageDataURL(raw, x, y, w, h)
		if err != nil {
			return
		}
		if !strings.HasPrefix(url, "data:image/png;base64,") {
			t.Fatalf("crop returned %q", url[:min(len(url), 64)])
		}
	})
}

func FuzzParseWhen(f *testing.F) {
	for _, seed := range []string{
		`{"HasMod":"X.Y"}`,
		`{"HasMod |contains=A.B": false}`,
		`{"HasMod":"A, B"}`,
		`{"Mode":"A"}`,
		`{"Season":"spring"}`,
		`{"HasMod |contains=Fixture.LoadB": false}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		var doc map[string]json.RawMessage
		if json.Unmarshal(raw, &doc) != nil {
			return
		}
		schema := map[string]Schema{
			"mode":   {Key: "Mode", AllowValues: []string{"A", "B"}, DefaultValue: "A"},
			"season": {Key: "Season", AllowValues: []string{"spring", "summer"}},
		}
		when := parseWhen(doc, map[string]bool{}, schema)
		_ = when.Holds(map[string]bool{})
		_ = DynamicWhenHolds(when, nil, map[string]bool{}, schema, nil)
	})
}

func FuzzAreaOf(f *testing.F) {
	for _, seed := range []string{
		`{"X":0,"Y":0,"Width":2,"Height":2}`,
		`{"X":"1","Y":"2","Width":"3","Height":"4"}`,
		`{"x":0,"y":0}`,
		`{}`,
	} {
		f.Add([]byte(seed))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		got, ok := areaOf(raw)
		if !ok {
			return
		}
		if got.Kind != 'r' {
			t.Fatalf("areaOf kind %q", got.Kind)
		}
	})
}

func FuzzContentPatcherPriority(f *testing.F) {
	for _, seed := range []string{"", "0", "1", "-1", "Low", "Late", "Exclusive", "Low+10", "High-5", "medium", "Early+100"} {
		f.Add("load", seed)
		f.Add("editimage", seed)
	}
	f.Fuzz(func(t *testing.T, kind, priority string) {
		if len(kind) > 1<<12 || len(priority) > 1<<12 {
			t.Skip()
		}
		got := contentPatcherPriority(kind, priority)
		if trimmed := strings.TrimSpace(priority); trimmed != "" {
			if n, err := strconv.Atoi(trimmed); err == nil && got != n {
				t.Fatalf("contentPatcherPriority(%q) = %d, want %d", priority, got, n)
			}
		}
		_ = strongerContentPatcherPriority(priority, "Late", kind)
	})
}

func cellSetKeys(s string) []string {
	var out []string
	seen := map[string]bool{}
	for p := range strings.SplitSeq(s, ";") {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		out = append(out, p)
	}
	slices.Sort(out)
	return out
}

func FuzzDecodeCellSet(f *testing.F) {
	for _, seed := range []string{"", "0,0", "1,2;3,4", "32767,32767", "-1,-1;2,2"} {
		f.Add(encodeCellSet(seed))
	}
	f.Add([]byte{0, 0, 0, 0, 0, 1, 0, 1, 0, 1})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<12 {
			t.Skip()
		}
		decoded := decodeCellSet(raw)
		if decoded == "" {
			return
		}
		round := decodeCellSet(encodeCellSet(decoded))
		if !slices.Equal(cellSetKeys(decoded), cellSetKeys(round)) {
			t.Fatalf("decode/encode diverged: %q -> %q", decoded, round)
		}
	})
}

func FuzzDecodeDiskPack(f *testing.F) {
	raw, err := encodeDiskPack(diskCachedPack{
		Patches:  []diskPatch{{Kind: "load", Target: "maps/town", FromFile: "a.tmx"}},
		Mentions: map[string]bool{"smapi:x.y": true},
	})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(raw)
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		pack, ok := decodeDiskPack(raw)
		if !ok {
			return
		}
		encoded, err := encodeDiskPack(pack)
		if err != nil {
			t.Fatalf("re-encode: %v", err)
		}
		if _, ok := decodeDiskPack(encoded); !ok {
			t.Fatal("re-encoded pack does not decode")
		}
	})
}

func FuzzReadPackCache(f *testing.F) {
	var buf bytes.Buffer
	if err := writePackCache(&buf, diskPackCache{
		Version: contentPackParserVersion,
		Packs: map[string]diskPackEntry{
			"pack": {Fingerprint: "abc", Files: []FileStamp{{Path: "content.json", Size: 1}}},
		},
	}); err != nil {
		f.Fatal(err)
	}
	f.Add(buf.Bytes())
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		_, _ = readPackCache(bytes.NewReader(raw))
	})
}

func FuzzUnzipPackCache(f *testing.F) {
	packed, err := zipPackCache([]byte(`{"version":2}`))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(packed)
	f.Add([]byte(`{"version":2}`))
	f.Add([]byte{0x1f, 0x8b})
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		payload, ok := unzipPackCache(raw)
		if !ok {
			return
		}
		if len(payload) > int(maxPackFileBytes) {
			t.Fatalf("unzipped %d bytes, cap %d", len(payload), maxPackFileBytes)
		}
	})
}

func FuzzReadConfigSchema(f *testing.F) {
	for _, seed := range fuzzContentSeeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		if len(raw) > 1<<16 {
			t.Skip()
		}
		root := t.TempDir()
		if err := os.WriteFile(filepath.Join(root, "content.json"), raw, 0o600); err != nil {
			t.Skip()
		}
		for k, s := range readConfigSchema(root) {
			if k != strings.ToLower(k) || k == "" || hasToken(k) {
				t.Fatalf("schema key %q", k)
			}
			if s.Key == "" {
				t.Fatalf("empty schema Key for %q", k)
			}
		}
	})
}
