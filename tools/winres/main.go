// Command winres writes the Windows resources of pasla.exe (icon, version
// information, manifest) as .syso object files that `go build` links in
// automatically. Standard library only, so it also runs offline.
//
//	go run ./tools/winres -version 1.2.3 -icon cmd/pasla/icon.ico -out cmd/pasla
package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	rtIcon      = 3
	rtGroupIcon = 14
	rtVersion   = 16
	rtManifest  = 24
	langNeutral = 0x0409 // en-US; what Explorer expects for version info
)

const manifest = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<assembly xmlns="urn:schemas-microsoft-com:asm.v1" manifestVersion="1.0">
  <assemblyIdentity type="win32" name="Pasla" version="%s" processorArchitecture="*"/>
  <description>Pasla</description>
  <trustInfo xmlns="urn:schemas-microsoft-com:asm.v3">
    <security><requestedPrivileges>
      <requestedExecutionLevel level="asInvoker" uiAccess="false"/>
    </requestedPrivileges></security>
  </trustInfo>
  <compatibility xmlns="urn:schemas-microsoft-com:compatibility.v1">
    <application>
      <supportedOS Id="{8e0f7a12-bfb3-4fe8-b9a5-48fd50a15a9a}"/>
    </application>
  </compatibility>
  <application xmlns="urn:schemas-microsoft-com:asm.v3">
    <windowsSettings>
      <activeCodePage xmlns="http://schemas.microsoft.com/SMI/2019/WindowsSettings">UTF-8</activeCodePage>
      <dpiAware xmlns="http://schemas.microsoft.com/SMI/2005/WindowsSettings">true</dpiAware>
    </windowsSettings>
  </application>
</assembly>
`

func main() {
	version := flag.String("version", "0.0.0", "product version (a leading v is ignored)")
	iconPath := flag.String("icon", "cmd/pasla/icon.ico", "multi-size .ico file")
	out := flag.String("out", "cmd/pasla", "directory for rsrc_windows_*.syso")
	flag.Parse()

	ico, err := os.ReadFile(*iconPath)
	if err != nil {
		log.Fatal(err)
	}
	ver := parseVersion(*version)
	res, err := buildResources(ico, ver, strings.TrimPrefix(*version, "v"))
	if err != nil {
		log.Fatal(err)
	}
	for _, arch := range []struct {
		name    string
		machine uint16
		reloc   uint16
	}{
		{"amd64", 0x8664, 3}, // IMAGE_REL_AMD64_ADDR32NB
		{"arm64", 0xAA64, 2}, // IMAGE_REL_ARM64_ADDR32NB
		{"386", 0x014C, 7},   // IMAGE_REL_I386_DIR32NB
	} {
		path := filepath.Join(*out, "rsrc_windows_"+arch.name+".syso")
		if err := os.WriteFile(path, coff(res, arch.machine, arch.reloc), 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Println("wrote", path)
	}
}

// ---- resource tree ----------------------------------------------------------

type resource struct {
	typ, id uint32
	data    []byte
}

func buildResources(ico []byte, ver [4]uint16, verStr string) ([]resource, error) {
	icons, group, err := splitIcon(ico)
	if err != nil {
		return nil, err
	}
	res := []resource{}
	for i, img := range icons {
		res = append(res, resource{rtIcon, uint32(i + 1), img})
	}
	res = append(res,
		resource{rtGroupIcon, 1, group},
		resource{rtVersion, 1, versionInfo(ver, verStr)},
		resource{rtManifest, 1, []byte(fmt.Sprintf(manifest, fmt.Sprintf("%d.%d.%d.%d", ver[0], ver[1], ver[2], ver[3])))},
	)
	return res, nil
}

// splitIcon turns an .ico file into RT_ICON images and the RT_GROUP_ICON
// directory that points at them by id.
func splitIcon(ico []byte) ([][]byte, []byte, error) {
	if len(ico) < 6 || binary.LittleEndian.Uint16(ico[2:]) != 1 {
		return nil, nil, errors.New("not an .ico file")
	}
	n := int(binary.LittleEndian.Uint16(ico[4:]))
	var images [][]byte
	var grp bytes.Buffer
	grp.Write(ico[:6])
	for i := 0; i < n; i++ {
		e := ico[6+16*i : 6+16*(i+1)]
		size := binary.LittleEndian.Uint32(e[8:])
		off := binary.LittleEndian.Uint32(e[12:])
		if int(off+size) > len(ico) {
			return nil, nil, errors.New("truncated .ico file")
		}
		images = append(images, ico[off:off+size])
		grp.Write(e[:12])
		binary.Write(&grp, binary.LittleEndian, uint16(i+1))
	}
	return images, grp.Bytes(), nil
}

func parseVersion(s string) [4]uint16 {
	s = strings.TrimPrefix(s, "v")
	s = strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '+' })[0]
	var v [4]uint16
	for i, p := range strings.SplitN(s, ".", 4) {
		n, _ := strconv.Atoi(p)
		v[i] = uint16(n)
	}
	return v
}

// ---- VS_VERSIONINFO ---------------------------------------------------------

func utf16z(s string) []byte {
	var b bytes.Buffer
	for _, u := range utf16.Encode([]rune(s)) {
		binary.Write(&b, binary.LittleEndian, u)
	}
	b.Write([]byte{0, 0})
	return b.Bytes()
}

func pad4(b *bytes.Buffer) {
	for b.Len()%4 != 0 {
		b.WriteByte(0)
	}
}

// node encodes one VERSIONINFO block: header, key, value, children.
func node(key string, value []byte, valueLen uint16, text bool, children ...[]byte) []byte {
	var b bytes.Buffer
	b.Write(make([]byte, 6)) // wLength, wValueLength, wType filled below
	b.Write(utf16z(key))
	pad4(&b)
	b.Write(value)
	for _, c := range children {
		pad4(&b)
		b.Write(c)
	}
	out := b.Bytes()
	binary.LittleEndian.PutUint16(out[0:], uint16(len(out)))
	binary.LittleEndian.PutUint16(out[2:], valueLen)
	if text {
		binary.LittleEndian.PutUint16(out[4:], 1)
	}
	return out
}

func versionInfo(v [4]uint16, verStr string) []byte {
	ms := uint32(v[0])<<16 | uint32(v[1])
	ls := uint32(v[2])<<16 | uint32(v[3])
	var fixed bytes.Buffer
	for _, x := range []uint32{
		0xFEEF04BD, 0x00010000, ms, ls, ms, ls,
		0x3F, 0, // flags mask, flags
		0x00040004, // VOS_NT_WINDOWS32
		1, 0,       // VFT_APP
		0, 0,
	} {
		binary.Write(&fixed, binary.LittleEndian, x)
	}
	strs := [][2]string{
		{"CompanyName", "Pasla"},
		{"FileDescription", "Pasla - yerel ağda dosya aktarımı"},
		{"FileVersion", verStr},
		{"InternalName", "pasla"},
		{"LegalCopyright", "MIT Lisansı"},
		{"OriginalFilename", "pasla.exe"},
		{"ProductName", "Pasla"},
		{"ProductVersion", verStr},
	}
	var kids [][]byte
	for _, kv := range strs {
		val := utf16z(kv[1])
		kids = append(kids, node(kv[0], val, uint16(len(val)/2), true))
	}
	table := node("040904B0", nil, 0, true, kids...)
	sfi := node("StringFileInfo", nil, 0, true, table)
	trans := []byte{0x09, 0x04, 0xB0, 0x04}
	vfi := node("VarFileInfo", nil, 0, true, node("Translation", trans, 4, false))
	return node("VS_VERSION_INFO", fixed.Bytes(), uint16(fixed.Len()), false, sfi, vfi)
}

// ---- COFF object with a .rsrc section ---------------------------------------

// rsrcSection lays out the three-level resource directory (type → id →
// language → data). It returns the section bytes and the offsets of the
// DataRVA fields, which need ADDR32NB relocations against the section.
func rsrcSection(res []resource) ([]byte, []uint32) {
	byType := map[uint32][]resource{}
	var types []uint32
	for _, r := range res {
		if _, ok := byType[r.typ]; !ok {
			types = append(types, r.typ)
		}
		byType[r.typ] = append(byType[r.typ], r)
	}
	sort.Slice(types, func(i, j int) bool { return types[i] < types[j] })

	dirSize := func(n int) uint32 { return uint32(16 + 8*n) }
	// Offsets: root dir, then type dirs, then language dirs, then data entries, then data.
	off := dirSize(len(types))
	typeDirOff := map[uint32]uint32{}
	for _, t := range types {
		typeDirOff[t] = off
		off += dirSize(len(byType[t]))
	}
	langDirOff := map[[2]uint32]uint32{}
	for _, t := range types {
		for _, r := range byType[t] {
			langDirOff[[2]uint32{t, r.id}] = off
			off += dirSize(1)
		}
	}
	entryOff := map[[2]uint32]uint32{}
	for _, t := range types {
		for _, r := range byType[t] {
			entryOff[[2]uint32{t, r.id}] = off
			off += 16
		}
	}
	dataOff := map[[2]uint32]uint32{}
	for _, t := range types {
		for _, r := range byType[t] {
			off = (off + 7) &^ 7
			dataOff[[2]uint32{t, r.id}] = off
			off += uint32(len(r.data))
		}
	}
	buf := make([]byte, (off+7)&^7)
	le := binary.LittleEndian
	writeDir := func(at uint32, entries [][2]uint32) {
		le.PutUint16(buf[at+14:], uint16(len(entries)))
		for i, e := range entries {
			le.PutUint32(buf[at+16+uint32(8*i):], e[0])
			le.PutUint32(buf[at+20+uint32(8*i):], e[1])
		}
	}
	var root [][2]uint32
	for _, t := range types {
		root = append(root, [2]uint32{t, 0x80000000 | typeDirOff[t]})
	}
	writeDir(0, root)
	var relocs []uint32
	for _, t := range types {
		var ids [][2]uint32
		rs := byType[t]
		sort.Slice(rs, func(i, j int) bool { return rs[i].id < rs[j].id })
		for _, r := range rs {
			k := [2]uint32{t, r.id}
			ids = append(ids, [2]uint32{r.id, 0x80000000 | langDirOff[k]})
			writeDir(langDirOff[k], [][2]uint32{{langNeutral, entryOff[k]}})
			e := entryOff[k]
			le.PutUint32(buf[e:], dataOff[k]) // RVA relative to section; fixed by relocation
			le.PutUint32(buf[e+4:], uint32(len(r.data)))
			relocs = append(relocs, e)
			copy(buf[dataOff[k]:], r.data)
		}
		writeDir(typeDirOff[t], ids)
	}
	return buf, relocs
}

func coff(res []resource, machine, relocType uint16) []byte {
	sec, relocs := rsrcSection(res)
	const hdr, sechdr = 20, 40
	rawOff := uint32(hdr + sechdr)
	relocOff := rawOff + uint32(len(sec))
	symOff := relocOff + uint32(10*len(relocs))

	var b bytes.Buffer
	le := binary.LittleEndian
	binary.Write(&b, le, machine)
	binary.Write(&b, le, uint16(1)) // sections
	binary.Write(&b, le, uint32(0)) // timestamp: reproducible builds
	binary.Write(&b, le, symOff)
	binary.Write(&b, le, uint32(1)) // symbols
	binary.Write(&b, le, uint16(0)) // optional header
	binary.Write(&b, le, uint16(0)) // characteristics

	b.WriteString(".rsrc\x00\x00\x00")
	binary.Write(&b, le, uint32(0)) // virtual size
	binary.Write(&b, le, uint32(0)) // virtual address
	binary.Write(&b, le, uint32(len(sec)))
	binary.Write(&b, le, rawOff)
	binary.Write(&b, le, relocOff)
	binary.Write(&b, le, uint32(0))
	binary.Write(&b, le, uint16(len(relocs)))
	binary.Write(&b, le, uint16(0))
	binary.Write(&b, le, uint32(0x40000040)) // initialized data, readable

	b.Write(sec)
	for _, r := range relocs {
		binary.Write(&b, le, r)
		binary.Write(&b, le, uint32(0)) // symbol 0: the section
		binary.Write(&b, le, relocType)
	}
	b.WriteString(".rsrc\x00\x00\x00")
	binary.Write(&b, le, uint32(0))
	binary.Write(&b, le, int16(1)) // section number
	binary.Write(&b, le, uint16(0))
	b.WriteByte(3) // IMAGE_SYM_CLASS_STATIC
	b.WriteByte(0)
	binary.Write(&b, le, uint32(4)) // empty string table
	return b.Bytes()
}
