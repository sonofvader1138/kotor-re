// Package kotor holds the readers and converters for the game's file formats (KEY, BIF, ERF, GFF, TPC, TGA, MDL,
// stream audio, IMA ADPCM and Bink), a minimal PNG encoder, the project paths, and helpers that every command uses.
package kotor

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
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
	TypeTpc = 3007
	TypeMdx = 3008
)

// File extension of each known resource type.
var extensions = map[uint32]string{
	TypeTga: "tga",
	TypeMdl: "mdl",
	TypeTxi: "txi",
	TypeGui: "gui",
	TypeTpc: "tpc",
	TypeMdx: "mdx",
}

// Ext returns the file extension of a resource type, or the type code in decimal when the type is unknown.
func Ext(resourceType uint32) string {
	ext, ok := extensions[resourceType]
	if !ok {
		return strconv.FormatUint(uint64(resourceType), 10)
	}
	return ext
}

// TexturePacks lists the texture packs used for model materials, in priority order. tpb and tpc hold the
// same textures at lower resolution and are not converted.
var TexturePacks = []string{"swpc_tex_tpa", "swpc_tex_gui"}

// TgaGroup is a group of BIF archives that hold TGA images, with the output folder name of the group.
type TgaGroup struct {
	Name    string
	Pattern *regexp.Regexp
}

var TgaGroups = []TgaGroup{
	{"lightmaps", regexp.MustCompile(`^data/lightmaps\d*\.bif$`)},
	{"textures", regexp.MustCompile(`^data/textures\.bif$`)},
	{"legacy", regexp.MustCompile(`^data/legacy\.bif$`)},
}

// ReportOrder is the order in which convert and verify print their stage summaries.
var ReportOrder = []string{"models", "textures", "tga", "music", "sounds", "waves", "movies", "gui"}

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

// Verdict ends a report line: "PASS" when there are no errors, otherwise "FAIL: " and the errors.
func Verdict(errors []string) string {
	if len(errors) == 0 {
		return "PASS"
	}
	return "FAIL: " + strings.Join(errors, "; ")
}

// Latin1 decodes bytes as ISO 8859-1, which maps each byte to the code point of the same value.
func Latin1(data []byte) string {
	text := make([]rune, len(data))
	for i, b := range data {
		text[i] = rune(b)
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

// cutNulLine cuts a NUL-padded Latin-1 field at its first NUL byte that has no line feed or carriage
// return after it. The JS readers used /\0.*$/ without the s flag, whose "." stops at line breaks.
func cutNulLine(data []byte) string {
	lastBreak := bytes.LastIndexAny(data, "\n\r")
	end := bytes.IndexByte(data[lastBreak+1:], 0)
	if end >= 0 {
		data = data[:lastBreak+1+end]
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
	data, err := os.ReadFile(file)
	if err != nil {
		Fatal(err.Error())
	}
	if string(Slice(data, 0, signatureLength)) != keySignature {
		Fatal(file + ": not KEY V1")
	}

	bifCount := int(u32(data, 8))
	keyCount := int(u32(data, 12))
	bifTable := int(u32(data, 16))
	keyTable := int(u32(data, 20))

	key := &Key{}
	for i := 0; i < bifCount; i++ {
		entry := bifTable + keyBifEntrySize*i
		nameOffset := int(u32(data, entry+4))
		nameLength := int(u16(data, entry+8))
		name := cutNulLine(Slice(data, nameOffset, nameOffset+nameLength))
		key.Bifs = append(key.Bifs, strings.ReplaceAll(name, `\`, "/"))
	}

	for i := 0; i < keyCount; i++ {
		entry := keyTable + keyEntrySize*i
		id := u32(data, entry+18)
		key.Keys = append(key.Keys, KeyEntry{
			Resref: strings.ToLower(cutNulLine(data[entry : entry+ResrefLength])),
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
	data, err := os.ReadFile(file)
	if err != nil {
		Fatal(err.Error())
	}
	if string(Slice(data, 0, signatureLength)) != bifSignature {
		Fatal(file + ": not BIFFV1")
	}

	entryCount := int(u32(data, 8))
	table := int(u32(data, 16))

	entries := make([]BifEntry, 0, entryCount)
	for i := 0; i < entryCount; i++ {
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
	file   *os.File
}

// Data reads the entry's resource from the archive. ReadErf has checked that the resource lies inside
// the file, so any read error is fatal.
func (entry ErfEntry) Data() []byte {
	data := make([]byte, entry.Size)
	readAt(entry.file, data, int64(entry.Offset))
	return data
}

// readAt fills buffer from file at offset and exits when the file ends early or the read fails.
func readAt(file *os.File, buffer []byte, offset int64) {
	if _, err := file.ReadAt(buffer, offset); err != nil {
		Fatal(err.Error())
	}
}

type Erf struct {
	Signature string
	Entries   []ErfEntry
	Size      int64
}

var erfSignature = regexp.MustCompile(`^(ERF |MOD |HAK |SAV )V1\.0$`)

// ReadErf reads the tables of an ERF V1.0 archive. Each entry reads its resource on demand, since the
// texture packs are up to 400 MB. The file stays open for the life of the process.
// ERF V1.0: 160-byte header. 0x10 u32 entryCount, 0x18 u32 keyListOff, 0x1C u32 resListOff.
// Key entry 24 bytes: char[16] resref, u32 id, u16 type, u16 unused. Resource entry 8 bytes: u32 offset, u32 size.
func ReadErf(path string) *Erf {
	file, err := os.Open(path)
	if err != nil {
		Fatal(err.Error())
	}

	info, err := file.Stat()
	if err != nil {
		Fatal(err.Error())
	}

	header := make([]byte, erfHeaderSize)
	readAt(file, header, 0)

	signature := Latin1(header[:signatureLength])
	if !erfSignature.MatchString(signature) {
		Fatal(path + ": not ERF V1.0")
	}

	entryCount := int(u32(header, 0x10))
	keyListOffset := int64(u32(header, 0x18))
	resourceListOffset := int64(u32(header, 0x1c))
	keyList := make([]byte, erfKeyEntrySize*entryCount)
	resourceList := make([]byte, erfResourceEntrySize*entryCount)
	readAt(file, keyList, keyListOffset)
	readAt(file, resourceList, resourceListOffset)

	erf := &Erf{Signature: signature, Size: info.Size()}
	for i := 0; i < entryCount; i++ {
		resourceEntry := erfResourceEntrySize * i
		keyEntry := erfKeyEntrySize * i
		offset := u32(resourceList, resourceEntry)
		size := u32(resourceList, resourceEntry+4)
		if int64(offset)+int64(size) > info.Size() {
			Fatal(fmt.Sprintf("%s: entry %d out of bounds", path, i))
		}

		resourceType := u16(keyList, keyEntry+20)
		erf.Entries = append(erf.Entries, ErfEntry{
			Resref: strings.ToLower(CString(keyList[keyEntry : keyEntry+ResrefLength])),
			Type:   resourceType,
			Ext:    Ext(resourceType),
			Offset: offset,
			Size:   size,
			file:   file,
		})
	}

	return erf
}

// ExecResult is the outcome of running a program. Status is -1 when the program did not start or was
// ended by a signal. StartError is set only when the program could not be started.
type ExecResult struct {
	Status     int
	StartError error
	Stdout     []byte
	Stderr     string
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
		result.StartError = err
	}
	return result
}

// Has reports whether a program can be found on the search path.
func Has(program string) bool {
	_, err := exec.LookPath(program)
	return err == nil
}
