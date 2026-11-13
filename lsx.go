package larian

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// maxLSXSize bounds the .lsx documents this package parses.
const maxLSXSize = 8 << 20

// Base-game module UUIDs (BG3ModManager src/Core/DivinityApp.cs GUSTAVDEV_UUID and
// GUSTAVX_UUID; that manager treats GustavX as the main campaign).
const (
	GustavDevUUID = "28ac9ce2-2aba-8cda-b3b5-6e922f71b6b8"
	GustavXUUID   = "cb555efe-2d9e-131f-8195-a89329d218ea"
)

// ModuleInfo identifies a mod, as meta.lsx's ModuleInfo node and
// modsettings.lsx's ModuleShortDesc node carry it.
type ModuleInfo struct {
	UUID          string
	Name          string
	Folder        string
	Version64     Version64
	MD5           string
	PublishHandle uint64
}

type lsxAttr struct {
	ID    string `xml:"id,attr"`
	Value string `xml:"value,attr"`
}

type lsxNode struct {
	ID       string    `xml:"id,attr"`
	Attrs    []lsxAttr `xml:"attribute"`
	Children []lsxNode `xml:"children>node"`
}

type lsxDoc struct {
	Regions []struct {
		ID    string    `xml:"id,attr"`
		Nodes []lsxNode `xml:"node"`
	} `xml:"region"`
}

func parseLSX(data []byte) (lsxDoc, error) {
	var doc lsxDoc

	if len(data) > maxLSXSize {
		return doc, fmt.Errorf("%w: lsx document is %d bytes", ErrInvalid, len(data))
	}

	if err := xml.Unmarshal(data, &doc); err != nil {
		return doc, fmt.Errorf("%w: lsx: %w", ErrInvalid, err)
	}

	return doc, nil
}

// find returns the first node with the given id, searching depth first.
func find(nodes []lsxNode, id string) *lsxNode {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}

		if n := find(nodes[i].Children, id); n != nil {
			return n
		}
	}

	return nil
}

func moduleFromNode(n *lsxNode) (ModuleInfo, error) {
	var m ModuleInfo

	for _, a := range n.Attrs {
		switch a.ID {
		case "UUID":
			m.UUID = a.Value
		case "Name":
			m.Name = a.Value
		case "Folder":
			m.Folder = a.Value
		case "MD5":
			m.MD5 = a.Value
		case "Version64", "Version":
			v, err := strconv.ParseUint(a.Value, 10, 64)
			if err != nil {
				return m, fmt.Errorf("%w: Version64 %q", ErrInvalid, a.Value)
			}

			m.Version64 = Version64(v)
		case "PublishHandle":
			v, err := strconv.ParseUint(a.Value, 10, 64)
			if err != nil {
				return m, fmt.Errorf("%w: PublishHandle %q", ErrInvalid, a.Value)
			}

			m.PublishHandle = v
		}
	}

	return m, nil
}

// ParseMeta parses a meta.lsx document into its ModuleInfo node.
func ParseMeta(data []byte) (ModuleInfo, error) {
	doc, err := parseLSX(data)
	if err != nil {
		return ModuleInfo{}, err
	}

	for _, r := range doc.Regions {
		if n := find(r.Nodes, "ModuleInfo"); n != nil {
			m, err := moduleFromNode(n)
			if err != nil {
				return ModuleInfo{}, err
			}

			if m.UUID == "" {
				return ModuleInfo{}, fmt.Errorf("%w: ModuleInfo has no UUID", ErrInvalid)
			}

			return m, nil
		}
	}

	return ModuleInfo{}, fmt.Errorf("%w: no ModuleInfo node", ErrInvalid)
}

// MetaPath returns the path of the package's Mods/<Folder>/meta.lsx, matched
// case-insensitively, or "" if there is none.
func (p *Pak) MetaPath() string {
	for _, e := range p.entries {
		parts := strings.Split(e.Path, "/")
		if len(parts) == 3 && strings.EqualFold(parts[0], "Mods") && strings.EqualFold(parts[2], "meta.lsx") {
			return e.Path
		}
	}

	return ""
}

// ReadModuleInfo reads and parses the package's Mods/<Folder>/meta.lsx.
// It returns [ErrNotFound] if the package has none.
func (p *Pak) ReadModuleInfo() (ModuleInfo, error) {
	path := p.MetaPath()
	if path == "" {
		return ModuleInfo{}, fmt.Errorf("%w: Mods/<Folder>/meta.lsx", ErrNotFound)
	}

	if e := p.entries[p.index[path]]; e.Size > maxLSXSize {
		return ModuleInfo{}, fmt.Errorf("%w: %s is %d bytes", ErrInvalid, path, e.Size)
	}

	data, err := p.Read(path)
	if err != nil {
		return ModuleInfo{}, err
	}

	return ParseMeta(data)
}

// ParseModSettings reads modsettings.lsx into the load order: the
// ModuleShortDesc children of the ModuleSettings region's Mods node, in file
// order. Base entries such as GustavDev are returned like any other.
func ParseModSettings(data []byte) ([]ModuleInfo, error) {
	doc, err := parseLSX(data)
	if err != nil {
		return nil, err
	}

	var out []ModuleInfo

	for _, r := range doc.Regions {
		if r.ID != "ModuleSettings" {
			continue
		}

		descs := find(r.Nodes, "Mods")
		if descs == nil {
			continue
		}

		for i := range descs.Children {
			if descs.Children[i].ID != "ModuleShortDesc" {
				continue
			}

			m, err := moduleFromNode(&descs.Children[i])
			if err != nil {
				return nil, err
			}

			out = append(out, m)
		}
	}

	return out, nil
}

// FormatModSettings renders modsettings.lsx for mods in load order, in the shape
// BG3ModManager writes (XML_MOD_SETTINGS_TEMPLATE, XML_MODULE_SHORT_DESC in
// src/Core/DivinityApp.cs): a Mods node only, no ModOrder. It is pure:
// pass the base entries (GustavDev) yourself, as [ParseModSettings] returns them.
func FormatModSettings(mods []ModuleInfo) []byte {
	var b bytes.Buffer

	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n<save>\n")
	b.WriteString("  <version major=\"4\" minor=\"8\" revision=\"0\" build=\"100\"/>\n")
	b.WriteString("  <region id=\"ModuleSettings\">\n    <node id=\"root\">\n      <children>\n")
	b.WriteString("        <node id=\"Mods\">\n          <children>\n")

	for _, m := range mods {
		b.WriteString("            <node id=\"ModuleShortDesc\">\n")
		writeAttr(&b, "Folder", "LSString", m.Folder)
		writeAttr(&b, "MD5", "LSString", m.MD5)
		writeAttr(&b, "Name", "LSString", m.Name)
		writeAttr(&b, "PublishHandle", "uint64", strconv.FormatUint(m.PublishHandle, 10))
		writeAttr(&b, "UUID", "guid", m.UUID)
		writeAttr(&b, "Version64", "int64", strconv.FormatUint(uint64(m.Version64), 10))
		b.WriteString("            </node>\n")
	}

	b.WriteString("          </children>\n        </node>\n")
	b.WriteString("      </children>\n    </node>\n  </region>\n</save>\n")

	return b.Bytes()
}

func writeAttr(b *bytes.Buffer, id, typ, value string) {
	b.WriteString("              ")
	fmt.Fprintf(b, "<attribute id=%q type=%q value=\"", id, typ)

	_ = xml.EscapeText(b, []byte(value))

	b.WriteString("\"/>\n")
}
