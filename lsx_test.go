package larian

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"reflect"
	"strings"
	"testing"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()

	b, err := fs.ReadFile(os.DirFS("testdata"), name)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

var tiny = ModuleInfo{
	UUID: "11111111-2222-3333-4444-555555555555", Name: "Tiny & Mod", Folder: "TinyMod",
	Version64: NewVersion64(4, 0, 0, 0), MD5: "abc123", PublishHandle: 1234567,
}

func TestParseMeta(t *testing.T) {
	t.Parallel()

	m, err := ParseMeta(fixture(t, "meta.lsx"))
	if err != nil || m != tiny {
		t.Fatalf("%+v %v", m, err)
	}

	if m.Version64.Major() != 4 {
		t.Errorf("major %d", m.Version64.Major())
	}
}

func TestParseMetaRejects(t *testing.T) {
	t.Parallel()

	for name, in := range map[string]string{
		"not xml":     "nope",
		"no module":   `<save><region id="Config"><node id="root"/></region></save>`,
		"no uuid":     `<save><region id="Config"><node id="ModuleInfo"/></region></save>`,
		"bad version": `<save><region id="Config"><node id="ModuleInfo"><attribute id="UUID" value="u"/><attribute id="Version64" value="-1"/></node></region></save>`,
		"too big":     strings.Repeat(" ", maxLSXSize+1),
	} {
		if _, err := ParseMeta([]byte(in)); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestPakReadModuleInfo(t *testing.T) {
	t.Parallel()

	meta := fixture(t, "meta.lsx")
	p, err := openBytes(t, buildPak(t, 18, []fixtureFile{
		{name: "Public/TinyMod/x.txt", data: []byte("x"), comp: CompressionNone},
		{name: "Mods/TinyMod/meta.lsx", data: meta, comp: CompressionLZ4},
	}))
	if err != nil {
		t.Fatal(err)
	}

	if m, err := p.ReadModuleInfo(); err != nil || m != tiny {
		t.Fatalf("%+v %v", m, err)
	}

	none, _ := openBytes(t, buildPak(t, 18, []fixtureFile{{name: "a", data: []byte("x"), comp: CompressionNone}}))
	if _, err := none.ReadModuleInfo(); !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound, got %v", err)
	}
}

func TestModSettingsRoundTrip(t *testing.T) {
	t.Parallel()

	mods, err := ParseModSettings(fixture(t, "modsettings.lsx"))
	if err != nil {
		t.Fatal(err)
	}

	if len(mods) != 2 || mods[0].UUID != GustavDevUUID || mods[0].Folder != "GustavDev" || mods[1] != tiny {
		t.Fatalf("%+v", mods)
	}

	out := FormatModSettings(mods)

	back, err := ParseModSettings(out)
	if err != nil || !reflect.DeepEqual(back, mods) {
		t.Fatalf("round trip: %+v %v", back, err)
	}

	if !bytes.Equal(FormatModSettings(back), out) {
		t.Error("format is not stable")
	}
}

func TestModSettingsKeepsFileOrder(t *testing.T) {
	t.Parallel()

	in := `<save><region id="ModuleSettings"><node id="root"><children>
<node id="Mods"><children>
 <node id="ModuleShortDesc"><attribute id="UUID" value="b"/><attribute id="Name" value="B"/></node>
 <node id="ModuleShortDesc"><attribute id="UUID" value="a"/><attribute id="Name" value="A"/></node>
</children></node></children></node></region></save>`

	mods, err := ParseModSettings([]byte(in))
	if err != nil || len(mods) != 2 || mods[0].UUID != "b" || mods[1].UUID != "a" {
		t.Fatalf("%+v %v", mods, err)
	}
}

func TestFormatModSettingsEscapes(t *testing.T) {
	t.Parallel()

	m := ModuleInfo{UUID: "u", Name: `a"<&>b`, Folder: "f\nx"}

	back, err := ParseModSettings(FormatModSettings([]ModuleInfo{m}))
	if err != nil || len(back) != 1 || back[0].Name != m.Name || back[0].Folder != m.Folder {
		t.Fatalf("%+v %v", back, err)
	}
}
