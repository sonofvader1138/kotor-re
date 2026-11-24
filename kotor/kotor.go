// Package kotor holds the readers and converters for the game's file formats (KEY, BIF, ERF, RIM, GFF, TPC, TGA, MDL,
// LIP, LTR, stream audio, IMA ADPCM and Bink), a minimal PNG encoder, the project paths, and helpers that every command uses.
package kotor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"sync"
)

// Root is the working directory ~/re. It is found from this source file, which is in kotor.
var Root = func() string {
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..")
}()

var (
	Game    = filepath.Join(Root, "swkotor")
	Assets  = filepath.Join(Root, "assets")
	Reports = filepath.Join(Root, "temp", "reports")
)

// Resource type codes used in KEY, BIF and ERF tables.
const (
	TypeTga = 3
	TypeMdl = 2002
	TypeTxi = 2022
	TypeGui = 2047
	TypeLip = 3004
	TypeTpc = 3007
	TypeMdx = 3008
)

// File extension of each known resource type.
var extensions = map[uint32]string{
	TypeTga: "tga",
	TypeMdl: "mdl",
	TypeTxi: "txi",
	TypeGui: "gui",
	TypeLip: "lip",
	TypeTpc: "tpc",
	TypeMdx: "mdx",

	// These types are named after the lowercase file type at the start of the resource. Each type has
	// one signature in every module archive.
	2010: "ncs", 2012: "are", 2014: "ifo", 2023: "git", 2025: "uti", 2027: "utc", 2029: "dlg", 2032: "utt",
	2035: "uts", 2038: "fac", 2040: "ute", 2042: "utd", 2044: "utp", 2051: "utm", 2056: "jrl", 2058: "utw",
	3003: "pth",

	// These types occur only in data/templates.bif and are named after their signatures in the same way.
	2015: "bic", 2024: "bti", 2026: "btc", 2030: "itp", 2036: "ltr",

	// These types are named after their content: a RIFF WAVE file, the signatures "2DA V2.b" and
	// "SSF V1.1", and NWScript source text, whose routine declarations nwscript.nss holds.
	4: "wav", 2017: "2da", 2060: "ssf", TypeNss: "nss",

	// The room visibility lists are named after the extension that swkotor.exe appends to an area name.
	3001: "vis",

	// The room layouts are named after the extension ".lyt", which swkotor.exe stores beside the keywords
	// "roomcount" and "trackcount".
	3000: "lyt",

	// The walkmeshes are named after the extension pool of swkotor.exe, which lists the extensions in
	// descending type order in places: "utw sav jrl utg btg pwk dwk utm" runs from 2058 down to 2051, so
	// "pwk" is 2053 and "dwk" is 2052. "wok" stands between "2da" (2017) and "bic" (2015).
	TypeWok: "wok", TypeDwk: "dwk", TypePwk: "pwk",

	// These types occur only in saved games. They are named after the extension pool of swkotor.exe, where
	// "sav" stands between "utw" (2058) and "jrl" (2056), and "res" stands beside "bmp", "mve" and "tga"
	// (1 to 3). A saved module is an ERF archive, and a type 0 resource is a GFF file of any file type.
	0: "res", 2057: "sav",
}

// Ext returns the file extension of a resource type, or the type code in decimal when the type is unknown.
func Ext(resourceType uint32) string {
	ext, ok := extensions[resourceType]
	if !ok {
		return strconv.FormatUint(uint64(resourceType), 10)
	}
	return ext
}

// TexturePacks lists the texture packs that are converted. tpb and tpc hold the textures of tpa at lower
// resolution.
var TexturePacks = []string{"swpc_tex_tpa", "swpc_tex_tpb", "swpc_tex_tpc", "swpc_tex_gui"}

// MaterialPacks lists the texture packs that model materials refer to, in priority order.
var MaterialPacks = []string{"swpc_tex_tpa", "swpc_tex_gui"}

// TgaGroup is a group of BIF archives that hold TGA images, with the output folder name of the group.
type TgaGroup struct {
	Name    string
	Pattern *regexp.Regexp
}

// ResourceBifs lists the BIF archives whose resources are converted one by one, without models: tables,
// sounds, scripts, templates, the journal and the room layouts. Each archive becomes the folder
// assets/data/<name>.
var ResourceBifs = []string{"2da", "sounds", "scripts", "templates", "_newbif", "layouts"}

var TgaGroups = []TgaGroup{
	{"lightmaps", regexp.MustCompile(`^data/lightmaps\d*\.bif$`)},
	{"textures", regexp.MustCompile(`^data/textures\.bif$`)},
	{"legacy", regexp.MustCompile(`^data/legacy\.bif$`)},
}

// reportOrder is the order in which RunStages prints the stage summaries.
var reportOrder = []string{"models", "textures", "tga", "music", "sounds", "waves", "movies", "gui", "patch", "lips", "modules", "rims", "bifs", "tlk", "saves"}

// A resource reference (resref) is a resource name of at most 16 bytes, padded with NUL bytes.
const ResrefLength = 16

// Signatures are 8 bytes: a 4-byte file type and a 4-byte version.
const (
	signatureLength = 8
	keySignature    = "KEY V1  "
	bifSignature    = "BIFFV1  "
	keyBifEntrySize = 12
	keyEntrySize    = 22
)

// A resource id holds the BIF index in its top 12 bits and the resource index in its low 20 bits.
const (
	resourceIndexBits = 20
	ResourceIndexMask = 1<<resourceIndexBits - 1
	bifEntrySize      = 16
)

const (
	erfHeaderSize        = 160
	erfKeyEntrySize      = 24
	erfResourceEntrySize = 8
)

// Fatal prints an error message and ends the process with exit status 1. Commands use it for errors they
// cannot recover from.
func Fatal(message string) {
	fmt.Fprintln(os.Stderr, message)
	os.Exit(1)
}

// ReadFile reads a whole file and ends the process when it cannot.
func ReadFile(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		Fatal(err.Error())
	}
	return data
}

// WriteFile writes a whole file and ends the process when it cannot.
func WriteFile(path string, data []byte) {
	if err := os.WriteFile(path, data, 0o644); err != nil {
		Fatal(err.Error())
	}
}

// MakeDir creates a directory and its parents and ends the process when it cannot.
func MakeDir(path string) {
	if err := os.MkdirAll(path, 0o755); err != nil {
		Fatal(err.Error())
	}
}

// ListFiles returns the files under dir whose extension is ext, ignoring case, as sorted paths relative to
// dir. recursive includes subfolders. A missing dir yields no files.
func ListFiles(dir, ext string, recursive bool) []string {
	var names []string
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == dir {
			return err
		}

		if entry.IsDir() && !recursive {
			return filepath.SkipDir
		}
		if !entry.IsDir() && strings.EqualFold(filepath.Ext(path), ext) {
			relative, _ := filepath.Rel(dir, path)
			names = append(names, relative)
		}
		return nil
	})
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		Fatal(err.Error())
	}

	slices.Sort(names)
	return names
}

// Verdict ends a report line: "PASS" when there are no errors, otherwise "FAIL: " and the errors. Empty
// strings are not errors, so a single error string that may be empty can be passed as it is.
func Verdict(errors ...string) string {
	var found []string
	for _, text := range errors {
		if text != "" {
			found = append(found, text)
		}
	}

	if len(found) == 0 {
		return "PASS"
	}
	return "FAIL: " + strings.Join(found, "; ")
}

// Latin1 decodes bytes as ISO 8859-1, which maps each byte to the code point of the same value.
func Latin1(data []byte) string {
	text := make([]rune, len(data))
	for i, b := range data {
		text[i] = rune(b)
	}
	return string(text)
}

// windows1252High holds the code points of bytes 0x80 to 0x9F in Windows-1252. The five bytes that
// Windows-1252 leaves undefined keep the code point of the same value, as in Latin-1.
var windows1252High = [32]rune{
	0x20AC, 0x81, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0x8D, 0x017D, 0x8F,
	0x90, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0x9D, 0x017E, 0x0178,
}

// Windows1252 decodes bytes as Windows-1252, which differs from Latin-1 only in bytes 0x80 to 0x9F.
func Windows1252(data []byte) string {
	text := make([]rune, len(data))
	for i, b := range data {
		text[i] = rune(b)
		if b >= 0x80 && b < 0xA0 {
			text[i] = windows1252High[b-0x80]
		}
	}
	return string(text)
}

// CString decodes a NUL-padded Latin-1 field and cuts it at the first NUL byte.
func CString(data []byte) string {
	end := bytes.IndexByte(data, 0)
	if end >= 0 {
		data = data[:end]
	}
	return Latin1(data)
}

// Slice returns data[start:end] with both bounds clamped to the data, like Buffer.subarray.
func Slice(data []byte, start, end int) []byte {
	end = min(max(end, 0), len(data))
	start = min(max(start, 0), end)
	return data[start:end]
}

func u16(data []byte, offset int) uint32 {
	return uint32(binary.LittleEndian.Uint16(data[offset:]))
}

func u32(data []byte, offset int) uint32 {
	return binary.LittleEndian.Uint32(data[offset:])
}

type KeyEntry struct {
	Resref string
	Type   uint32
	Bif    uint32
	Index  uint32
}

type Key struct {
	Bifs []string
	Keys []KeyEntry
}

// ReadKey reads chitin.key.
// KEY: header 8 magic, u32 bifCount, keyCount, bifTableOff, keyTableOff.
// Bif entry 12 bytes: u32 size, u32 nameOff, u16 nameLen, u16 drives.
// Key entry 22 bytes: char[16] resref, u16 type, u32 id (bif << 20 | index).
func ReadKey() *Key {
	file := filepath.Join(Game, "chitin.key")
	data := ReadFile(file)
	if string(Slice(data, 0, signatureLength)) != keySignature {
		Fatal(file + ": not KEY V1")
	}

	bifCount := int(u32(data, 8))
	keyCount := int(u32(data, 12))
	bifTable := int(u32(data, 16))
	keyTable := int(u32(data, 20))

	key := &Key{}
	for i := range bifCount {
		entry := bifTable + keyBifEntrySize*i
		nameOffset := int(u32(data, entry+4))
		nameLength := int(u16(data, entry+8))
		name := CString(Slice(data, nameOffset, nameOffset+nameLength))
		key.Bifs = append(key.Bifs, strings.ReplaceAll(name, `\`, "/"))
	}

	for i := range keyCount {
		entry := keyTable + keyEntrySize*i
		id := u32(data, entry+18)
		key.Keys = append(key.Keys, KeyEntry{
			Resref: strings.ToLower(CString(data[entry : entry+ResrefLength])),
			Type:   u16(data, entry+ResrefLength),
			Bif:    id >> resourceIndexBits,
			Index:  id & ResourceIndexMask,
		})
	}

	return key
}

type BifEntry struct {
	ID     uint32
	Offset uint32
	Size   uint32
	Type   uint32
}

// ReadBif reads a whole BIF archive and its variable resource table.
// BIF: 8 magic, u32 varCount, u32 fixedCount, u32 tableOff.
// Var entry 16 bytes: u32 id, u32 offset, u32 size, u32 type.
func ReadBif(file string) ([]byte, []BifEntry) {
	data := ReadFile(file)
	if string(Slice(data, 0, signatureLength)) != bifSignature {
		Fatal(file + ": not BIFFV1")
	}

	entryCount := int(u32(data, 8))
	table := int(u32(data, 16))

	entries := make([]BifEntry, 0, entryCount)
	for i := range entryCount {
		entry := table + bifEntrySize*i
		offset := u32(data, entry+4)
		size := u32(data, entry+8)
		if uint64(offset)+uint64(size) > uint64(len(data)) {
			Fatal(fmt.Sprintf("%s: entry %d out of bounds", file, i))
		}

		entries = append(entries, BifEntry{
			ID:     u32(data, entry),
			Offset: offset,
			Size:   size,
			Type:   u32(data, entry+12),
		})
	}

	return data, entries
}

type Resource struct {
	Resref string
	Type   uint32
	Ext    string
	Data   []byte
}

// BifResources returns the resources of one BIF archive, named through the KEY file.
func BifResources(bifName string, key *Key) []Resource {
	bifIndex := -1
	for i, name := range key.Bifs {
		if name == bifName {
			bifIndex = i
			break
		}
	}
	if bifIndex < 0 {
		Fatal(bifName + " not in chitin.key")
	}

	// A later KEY entry for the same index and type replaces an earlier one.
	names := map[uint64]string{}
	for _, entry := range key.Keys {
		if entry.Bif == uint32(bifIndex) {
			names[uint64(entry.Index)<<32|uint64(entry.Type)] = entry.Resref
		}
	}

	data, entries := ReadBif(filepath.Join(Game, bifName))
	resources := make([]Resource, 0, len(entries))
	for _, entry := range entries {
		index := entry.ID & ResourceIndexMask
		resref, ok := names[uint64(index)<<32|uint64(entry.Type)]
		if !ok {
			resref = fmt.Sprintf("unnamed_%d", index)
		}

		resources = append(resources, Resource{
			Resref: resref,
			Type:   entry.Type,
			Ext:    Ext(entry.Type),
			Data:   data[entry.Offset : entry.Offset+entry.Size],
		})
	}
	return resources
}

type ErfEntry struct {
	Resref string
	Type   uint32
	Ext    string
	Offset uint32
	Size   uint32
	file   io.ReaderAt
}

// Data reads the entry's resource from the archive. ReadErf has checked that the resource lies inside
// the file, so any read error is fatal.
func (entry ErfEntry) Data() []byte {
	data := make([]byte, entry.Size)
	readAt(entry.file, data, int64(entry.Offset))
	return data
}

// readAt fills buffer from file at offset and exits when the file ends early or the read fails.
func readAt(file io.ReaderAt, buffer []byte, offset int64) {
	if _, err := file.ReadAt(buffer, offset); err != nil {
		Fatal(err.Error())
	}
}

var erfSignature = regexp.MustCompile(`^(ERF |MOD |HAK |SAV )V1\.0$`)

// ReadErf reads the tables of an ERF V1.0 archive file (see ErfEntries). The file stays open for the life
// of the process.
func ReadErf(path string) []ErfEntry {
	file, err := os.Open(path)
	if err != nil {
		Fatal(err.Error())
	}

	info, err := file.Stat()
	if err != nil {
		Fatal(err.Error())
	}
	return ErfEntries(file, info.Size(), path)
}

// ErfEntries reads the tables of an ERF V1.0 archive of the given size from file. name is used in error
// messages. Each entry reads its resource on demand, since the texture packs are up to 400 MB. A saved
// game nests one archive in another, so file can also be a bytes.Reader.
// ERF V1.0: 160-byte header. 0x10 u32 entryCount, 0x18 u32 keyListOff, 0x1C u32 resListOff.
// Key entry 24 bytes: char[16] resref, u32 id, u16 type, u16 unused. Resource entry 8 bytes: u32 offset, u32 size.
func ErfEntries(file io.ReaderAt, fileSize int64, name string) []ErfEntry {
	header := make([]byte, erfHeaderSize)
	readAt(file, header, 0)

	if !erfSignature.MatchString(Latin1(header[:signatureLength])) {
		Fatal(name + ": not ERF V1.0")
	}

	entryCount := int(u32(header, 0x10))
	keyListOffset := int64(u32(header, 0x18))
	resourceListOffset := int64(u32(header, 0x1c))
	keyList := make([]byte, erfKeyEntrySize*entryCount)
	resourceList := make([]byte, erfResourceEntrySize*entryCount)
	readAt(file, keyList, keyListOffset)
	readAt(file, resourceList, resourceListOffset)

	var entries []ErfEntry
	for i := range entryCount {
		resourceEntry := erfResourceEntrySize * i
		keyEntry := erfKeyEntrySize * i
		offset := u32(resourceList, resourceEntry)
		size := u32(resourceList, resourceEntry+4)
		if int64(offset)+int64(size) > fileSize {
			Fatal(fmt.Sprintf("%s: entry %d out of bounds", name, i))
		}

		resourceType := u16(keyList, keyEntry+20)
		entries = append(entries, ErfEntry{
			Resref: strings.ToLower(CString(keyList[keyEntry : keyEntry+ResrefLength])),
			Type:   resourceType,
			Ext:    Ext(resourceType),
			Offset: offset,
			Size:   size,
			file:   file,
		})
	}

	return entries
}

// ErfLayoutProblem returns an error text when the resources of an ERF archive do not start directly after
// the resource table, lie in table order without gaps, and end exactly at the end of the file, or an empty
// string when they do. All archives of the game files have this layout.
func ErfLayoutProblem(entries []ErfEntry, fileSize int64) string {
	position := int64(erfHeaderSize + (erfKeyEntrySize+erfResourceEntrySize)*len(entries))
	for i, entry := range entries {
		if int64(entry.Offset) != position {
			return fmt.Sprintf("entry %d starts at %d, expected %d", i, entry.Offset, position)
		}
		position += int64(entry.Size)
	}

	if position != fileSize {
		return fmt.Sprintf("resources end at %d of %d bytes", position, fileSize)
	}
	return ""
}

const (
	rimSignature     = "RIM V1.0"
	rimHeaderSize    = 120
	rimEntrySize     = 32
	rimResourceAlign = 4
)

// RimEntry is one resource of a RIM archive. Data is a slice of the archive's bytes.
type RimEntry struct {
	Resref string
	Type   uint32
	Ext    string
	Offset uint32
	Data   []byte
}

// Rim is a parsed RIM archive. Flag is the u32 at 0x14, which is 1 only in the "dx" archives of rims/.
type Rim struct {
	Flag    uint32
	Entries []RimEntry
}

// ReadRim parses a RIM V1.0 archive and returns an error text when the file breaks any of the rules below.
// RIM V1.0: 120-byte header. 0x08 u32 zero, 0x0C u32 entryCount, 0x10 u32 tableOff (120), 0x14 u32 flag,
// 0x18 96 zero bytes. Entry 32 bytes: char[16] resref, u32 type, u32 id (= index), u32 offset, u32 size.
// Strict: resources lie inside the file in table order, start on a multiple of 4, do not overlap, and every
// byte between them and after the last one is zero.
func ReadRim(data []byte) (*Rim, string) {
	if len(data) < rimHeaderSize || string(data[:signatureLength]) != rimSignature {
		return nil, "not RIM V1.0"
	}

	entryCount := int(u32(data, 0x0c))
	tableOffset := int(u32(data, 0x10))
	if u32(data, 0x08) != 0 || tableOffset != rimHeaderSize || !allZero(data[0x18:rimHeaderSize]) {
		return nil, "unexpected header values"
	}
	if tableOffset+rimEntrySize*entryCount > len(data) {
		return nil, "table past end of file"
	}

	rim := &Rim{Flag: u32(data, 0x14)}
	position := tableOffset + rimEntrySize*entryCount
	for i := range entryCount {
		entry := tableOffset + rimEntrySize*i
		resourceType := u32(data, entry+16)
		offset := int(u32(data, entry+24))
		size := int(u32(data, entry+28))
		if int(u32(data, entry+20)) != i {
			return nil, fmt.Sprintf("entry %d has id %d", i, u32(data, entry+20))
		}
		if offset < position || offset%rimResourceAlign != 0 || offset+size > len(data) {
			return nil, fmt.Sprintf("entry %d at %d+%d is misplaced", i, offset, size)
		}
		if !allZero(data[position:offset]) {
			return nil, fmt.Sprintf("nonzero bytes before entry %d", i)
		}

		rim.Entries = append(rim.Entries, RimEntry{
			Resref: strings.ToLower(CString(data[entry : entry+ResrefLength])),
			Type:   resourceType,
			Ext:    Ext(resourceType),
			Offset: uint32(offset),
			Data:   data[offset : offset+size],
		})
		position = offset + size
	}

	if !allZero(data[position:]) {
		return nil, "nonzero bytes after last entry"
	}
	return rim, ""
}

// ExecResult is the outcome of running a program. Status is -1 when the program did not start or was
// ended by a signal. When the program could not be started, Stderr holds the reason.
type ExecResult struct {
	Status int
	Stdout []byte
	Stderr string
}

// Exec runs a program with input on its standard input and waits for it to end. A nil input gives the
// program an empty standard input.
func Exec(program string, args []string, input []byte) ExecResult {
	var stdout, stderr bytes.Buffer
	command := exec.Command(program, args...)
	command.Stdin = bytes.NewReader(input)
	command.Stdout = &stdout
	command.Stderr = &stderr

	err := command.Run()
	result := ExecResult{Status: -1, Stdout: stdout.Bytes(), Stderr: stderr.String()}
	if command.ProcessState != nil {
		result.Status = command.ProcessState.ExitCode()
	} else {
		result.Stderr = err.Error()
	}
	return result
}

// FirstLine returns the first line of text after surrounding white space is removed.
func FirstLine(text string) string {
	return strings.SplitN(strings.TrimSpace(text), "\n", 2)[0]
}

// Has reports whether a program can be found on the search path.
func Has(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}

// Summary holds the summary lines of one stage of the convert or verify command.
type Summary []string

// Note adds a line to the summary.
func (summary *Summary) Note(text string) {
	*summary = append(*summary, text)
}

// Tally writes a report to Reports and adds its pass and fail counts to the summary.
func (summary *Summary) Tally(name string, report []string) {
	WriteFile(filepath.Join(Reports, name+".txt"), []byte(strings.Join(report, "\n")+"\n"))

	pass := 0
	for _, line := range report {
		if strings.HasSuffix(line, "PASS") {
			pass++
		}
	}
	summary.Note(fmt.Sprintf("%s: %d pass, %d fail", name, pass, len(report)-pass))
}

// Stage is one named step of the convert or verify command.
type Stage struct {
	Name string
	Run  func(*Summary)
}

// RunStages runs the stages whose names are given, or all of them when none are, at the same time. It then
// prints their summaries in reportOrder.
func RunStages(stages []Stage, names []string) {
	MakeDir(Reports)

	results := map[string]*Summary{}
	var group sync.WaitGroup
	for _, stage := range stages {
		if len(names) > 0 && !slices.Contains(names, stage.Name) {
			continue
		}

		summary := &Summary{}
		results[stage.Name] = summary
		group.Go(func() { stage.Run(summary) })
	}
	group.Wait()

	var output []string
	for _, name := range reportOrder {
		if summary, ok := results[name]; ok {
			output = append(output, *summary...)
		}
	}
	fmt.Println(strings.Join(output, "\n"))
}

func allZero(data []byte) bool {
	return bytes.Count(data, []byte{0}) == len(data)
}
