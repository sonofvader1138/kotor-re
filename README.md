# File formats of Star Wars: Knights of the Old Republic

This document describes the game's file formats as determined from the files in `swkotor`. Each field lists the evidence for it and a confidence level. Fields that have not been verified are marked UNKNOWN.

All integers and floats are little-endian. Offsets are in hexadecimal and count from the start of the structure they belong to, unless a section states otherwise.

## Contents

- [Archives: KEY, BIF, ERF and RIM](#archives-key-bif-erf-and-rim)
  - [KEY file](#key-file)
  - [BIF archive](#bif-archive)
  - [ERF archive](#erf-archive)
  - [RIM archive](#rim-archive)
- [Generic file format: GFF](#generic-file-format-gff)
  - [Header](#header)
  - [Struct, field and label tables](#struct-field-and-label-tables)
  - [Field types](#field-types)
  - [Conversion to JSON](#conversion-to-json)
- [Modules: modules/\*.rim](#modules-modulesrim)
  - [Archive pairs](#archive-pairs)
  - [Module and area names](#module-and-area-names)
  - [Contents of the module files](#contents-of-the-module-files)
  - [Links to other files](#links-to-other-files)
  - [Compiled scripts: NCS](#compiled-scripts-ncs)
  - [Conversion and verification](#conversion-and-verification)
- [Global resources: rims/\*.rim](#global-resources-rimsrim)
  - [Archives](#archives)
  - [2DA table](#2da-table)
  - [SSF sound set](#ssf-sound-set)
  - [WAV sounds](#wav-sounds)
  - [Conversion and verification](#conversion-and-verification-1)
- [Tables, sounds, scripts and templates in the BIF archives](#tables-sounds-scripts-and-templates-in-the-bif-archives)
  - [2DA tables](#2da-tables)
  - [WAV sounds](#wav-sounds-1)
  - [NWScript sources: NSS](#nwscript-sources-nss)
  - [Templates](#templates)
  - [LTR letter tables](#ltr-letter-tables)
  - [Conversion and verification](#conversion-and-verification-2)
- [String table: dialog.tlk](#string-table-dialogtlk)
  - [Flags](#flags)
  - [Sound names](#sound-names)
  - [Conversion and verification](#conversion-and-verification-3)
- [3D models: MDL and MDX](#3d-models-mdl-and-mdx)
  - [MDL file header](#mdl-file-header)
  - [Model header](#model-header)
  - [Node](#node)
  - [Mesh header](#mesh-header)
  - [Face](#face)
  - [MDX file](#mdx-file)
  - [Walkmeshes in models.bif](#walkmeshes-in-modelsbif)
- [Walkmeshes: BWM](#walkmeshes-bwm)
  - [Header](#header-1)
  - [Vertices](#vertices)
  - [Faces, materials, normals and plane distances](#faces-materials-normals-and-plane-distances)
  - [AABB tree](#aabb-tree)
  - [Adjacency table](#adjacency-table)
  - [Edge table](#edge-table)
  - [Perimeter table](#perimeter-table)
  - [Conversion and verification](#conversion-and-verification-4)
- [Textures: TPC](#textures-tpc)
  - [TPC header](#tpc-header)
  - [Pixel data layout](#pixel-data-layout)
  - [TXI trailer](#txi-trailer)
  - [Reduced texture packs: tpb and tpc](#reduced-texture-packs-tpb-and-tpc)
  - [Conversion to model materials](#conversion-to-model-materials)
- [Images: TGA in the BIF archives](#images-tga-in-the-bif-archives)
  - [TGA header](#tga-header)
  - [Trailer](#trailer)
  - [Lightmaps](#lightmaps)
  - [Other resources in the same archives](#other-resources-in-the-same-archives)
- [Room layouts: LYT](#room-layouts-lyt)
  - [Text layout](#text-layout)
  - [Comparison with the module door instances](#comparison-with-the-module-door-instances)
  - [Model names](#model-names)
  - [Irregularities](#irregularities)
  - [Conversion and verification](#conversion-and-verification-5)
- [Room visibility lists: VIS](#room-visibility-lists-vis)
  - [Text layout](#text-layout-1)
  - [Relation to room layouts](#relation-to-room-layouts)
  - [Conversion and verification](#conversion-and-verification-6)
- [User interface layouts: GUI](#user-interface-layouts-gui)
  - [Resources in patch.erf](#resources-in-patcherf)
  - [GUI layout content](#gui-layout-content)
- [Patch archive: patch.erf](#patch-archive-patcherf)
  - [Textures](#textures)
  - [Tables](#tables)
  - [Model](#model)
  - [Conversion and verification](#conversion-and-verification-7)
- [Music: streammusic/\*.wav](#music-streammusicwav)
  - [Layout A: MP3 behind a fake WAV header](#layout-a-mp3-behind-a-fake-wav-header)
  - [Layout B: WAV behind an MP3 stub](#layout-b-wav-behind-an-mp3-stub)
  - [IMA ADPCM encoding](#ima-adpcm-encoding)
  - [Conversion and verification](#conversion-and-verification-8)
- [Sound effects and voices: streamsounds/\*.wav](#sound-effects-and-voices-streamsoundswav)
  - [Embedded WAV](#embedded-wav)
  - [File names](#file-names)
  - [Verification](#verification)
- [Voice-over: streamwaves/\*\*/\*.wav](#voice-over-streamwaveswav)
  - [Fake header](#fake-header)
  - [MP3 stream](#mp3-stream)
  - [Folder structure and file names](#folder-structure-and-file-names)
  - [Verification](#verification-1)
- [Lip sync: lips/\*.mod](#lip-sync-lipsmod)
  - [Archives](#archives-1)
  - [LIP resource](#lip-resource)
  - [Link to the voice files](#link-to-the-voice-files)
  - [Conversion and verification](#conversion-and-verification-9)
- [Movies: movies/\*.bik](#movies-moviesbik)
  - [Header](#header-2)
  - [Frame index](#frame-index)
  - [Frame](#frame)
  - [Conversion and verification](#conversion-and-verification-10)
- [Saved games: saves/\*/](#saved-games-saves)
  - [Files of a saved game](#files-of-a-saved-game)
  - [Saved game archive: SAVEGAME.sav](#saved-game-archive-savegamesav)
  - [GFF field types in saved games](#gff-field-types-in-saved-games)
  - [Global variables: GLOBALVARS.res](#global-variables-globalvarsres)
  - [Party table: PARTYTABLE.res](#party-table-partytableres)
  - [Save summary: savenfo.res](#save-summary-savenfores)
  - [Screenshot: Screen.tga](#screenshot-screentga)
  - [Conversion and verification](#conversion-and-verification-11)

# Archives: KEY, BIF, ERF and RIM

Most game resources are stored in 26 BIF archives in `data/`. A BIF archive holds resource data but no names. The file `chitin.key` (the KEY file) lists every resource, gives it a name, and records which BIF archive holds it. Both files are needed to extract a named resource. The reader is `kotor/kotor.go`.

A resource name is called a resref. It is at most 16 characters long. A resource is identified by its resref together with a numeric resource type.

## KEY file

The KEY file begins with a 64-byte header.

| Offset | Type    | Meaning                                | Evidence                                                        | Confidence |
| ------ | ------- | -------------------------------------- | --------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "KEY V1 "                    | Present in the only KEY file.                                   | high       |
| 0x08   | u32     | Number of BIF archives (26)            | Equals the number of entries in the BIF table.                  | high       |
| 0x0C   | u32     | Number of resources (25,836)           | Equals the total number of entries in all 26 archives.          | high       |
| 0x10   | u32     | Offset of the BIF table (64)           | The table parses consistently from this offset.                 | high       |
| 0x14   | u32     | Offset of the resource table (847)     | The table ends exactly at the end of the file.                  | high       |
| 0x18   | u32     | Build year minus 1900 (103, i.e. 2003) | The ERF texture packs hold the same value (see the ERF header). | medium     |
| 0x1C   | u32     | Build day of the year (309)            | The ERF texture packs hold the same value (see the ERF header). | medium     |
| 0x20   | u8[32]  | Reserved                               | All bytes are zero.                                             | medium     |

The BIF table has one 12-byte entry per archive.

| Offset | Type | Meaning                                         | Evidence                                                    | Confidence |
| ------ | ---- | ----------------------------------------------- | ----------------------------------------------------------- | ---------- |
| 0x00   | u32  | Size of the archive file in bytes               | Matches the size on disk for all 26 archives.               | high       |
| 0x04   | u32  | Offset of the archive's file name               | The names fill bytes 376 to 847 without gaps.               | high       |
| 0x08   | u16  | Length of the file name, including the NUL byte | Every name ends in a NUL byte at this length.               | high       |
| 0x0A   | u16  | UNKNOWN                                         | The value is 1 in all 26 entries. Its meaning is not known. | low        |

File names are relative paths that use backslashes, for example `data\models.bif`.

The resource table has one 22-byte entry per resource.

| Offset | Type     | Meaning                                                                   | Evidence                                                         | Confidence |
| ------ | -------- | ------------------------------------------------------------------------- | ---------------------------------------------------------------- | ---------- |
| 0x00   | char[16] | Resref, padded with NUL bytes                                             | The longest name uses all 16 bytes.                              | high       |
| 0x10   | u16      | Resource type                                                             | Matches the type in the archive entry for all 25,836 resources.  | high       |
| 0x12   | u32      | Resource ID. Bits 20 to 31 give the archive; bits 0 to 19 give the entry. | Every ID resolves to an existing archive entry of the same type. | high       |

No two resources share both a resref and a type.

## BIF archive

A BIF archive begins with a 20-byte header.

| Offset | Type    | Meaning                        | Evidence                                                | Confidence |
| ------ | ------- | ------------------------------ | ------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "BIFFV1 "            | Present in all 26 archives.                             | high       |
| 0x08   | u32     | Number of variable resources   | The entry table parses consistently in all 26 archives. | high       |
| 0x0C   | u32     | Number of fixed resources      | The value is 0 in all 26 archives.                      | medium     |
| 0x10   | u32     | Offset of the entry table (20) | The value is 20 in all 26 archives.                     | high       |

No archive contains fixed resources, so the layout of a fixed resource entry is UNKNOWN.

The entry table has one 16-byte entry per resource.

| Offset | Type | Meaning                                                                                                     | Evidence                                     | Confidence |
| ------ | ---- | ----------------------------------------------------------------------------------------------------------- | -------------------------------------------- | ---------- |
| 0x00   | u32  | Resource ID. Bits 0 to 19 equal the entry's index; bits 20 to 31 equal the archive's index in the KEY file. | True for all 25,836 entries.                 | high       |
| 0x04   | u32  | Offset of the resource data                                                                                 | Every resource lies inside its archive.      | high       |
| 0x08   | u32  | Size of the resource data in bytes                                                                          | Every resource lies inside its archive.      | high       |
| 0x0C   | u32  | Resource type                                                                                               | Matches the KEY file for all 25,836 entries. | high       |

Resource data follows the entry table directly. The resources are stored in order, without gaps or overlaps, and the last one ends exactly at the end of the file. This holds for all 26 archives. The data is not compressed.

The archives contain 29 resource types, and every one of them is described in this document. The number of resources of each type is listed below. Type 3007 (TPC texture) does not occur in the BIF archives; it occurs in the ERF texture packs and in `patch.erf`.

| Type | Count | Type | Count | Type | Count | Type | Count |
| ---- | ----- | ---- | ----- | ---- | ----- | ---- | ----- |
| 3    | 5,601 | 2016 | 1,202 | 2029 | 32    | 2052 | 156   |
| 4    | 1,928 | 2017 | 209   | 2030 | 16    | 2053 | 196   |
| 2002 | 2,832 | 2022 | 5,615 | 2032 | 21    | 2056 | 1     |
| 2009 | 1,774 | 2024 | 1     | 2036 | 3     | 2058 | 9     |
| 2010 | 1,784 | 2025 | 557   | 2040 | 65    | 2060 | 106   |
| 2015 | 1     | 2026 | 3     | 2042 | 50    | 3000 | 124   |
|      |       | 2027 | 205   | 2044 | 317   | 3001 | 112   |
|      |       |      |       | 2047 | 84    | 3008 | 2,832 |

Many types begin with a signature of four characters followed by a version. The table below lists the signatures found at the start of the resources of each type. The "V3.2" container is described in the section "Generic file format: GFF", "BWM V1.0" in "Walkmeshes: BWM", "NCS V1.0" in "Compiled scripts: NCS", "SSF V1.1" in "SSF sound set" and "LTR V1.0" in "LTR letter tables".

| Signature | Types and counts                                                                                                                                                                                               |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| "V3.2"    | 2015 BIC (1), 2024 BTI (1), 2025 UTI (557), 2026 BTC (3), 2027 UTC (205), 2029 DLG (32), 2030 ITP (16), 2032 UTT (21), 2040 UTE (65), 2042 UTD (50), 2044 UTP (317), 2047 GUI (84), 2056 JRL (1), 2058 UTW (9) |
| BWM V1.0  | 2016 (1,202), 2052 (156), 2053 (196)                                                                                                                                                                           |
| NCS V1.0  | 2010 (1,784)                                                                                                                                                                                                   |
| SSF V1.1  | 2060 (106)                                                                                                                                                                                                     |
| LTR V1.0  | 2036 (3)                                                                                                                                                                                                       |

In the "V3.2" row, the four characters before the version differ by type and are given beside each type number. Every resource of these types carries its signature. The other types carry no such signature: 3 is a TGA image, 4 a RIFF WAVE sound, 2002 an MDL model and 3008 its MDX vertex data, 2009 NWScript source text, 2017 a binary 2DA table (signature "2DA V2.b"), 2022 a TXI text file of texture options, 3000 a LYT room layout and 3001 a VIS room visibility list.

The table below lists what each archive holds and where its conversion is described. Every archive is converted except `items.bif`, `party.bif` and `player.bif`. Their 387 models pass the checks of the model converter (see "3D models: MDL and MDX"), but they are not written.

| Archives                                                               | Contents                                           | Section                                                         |
| ---------------------------------------------------------------------- | -------------------------------------------------- | --------------------------------------------------------------- |
| `models.bif`                                                           | 2,445 models and 1,554 walkmeshes                  | "3D models: MDL and MDX", "Walkmeshes: BWM"                     |
| `items.bif`, `party.bif`, `player.bif`                                 | 305, 19 and 63 models                              | Not converted                                                   |
| `lightmaps.bif`, `lightmaps2.bif` to `13.bif`                          | 5,586 TGA lightmaps, their TXI files, 112 VIS      | "Images: TGA in the BIF archives", "Room visibility lists: VIS" |
| `textures.bif`, `legacy.bif`                                           | 15 TGA images and 28 TXI files                     | "Images: TGA in the BIF archives"                               |
| `gui.bif`                                                              | 84 GUI layouts                                     | "User interface layouts: GUI"                                   |
| `layouts.bif`                                                          | 124 room layouts                                   | "Room layouts: LYT"                                             |
| `2da.bif`, `sounds.bif`, `scripts.bif`, `templates.bif`, `_newbif.bif` | Tables, sounds, scripts, templates and the journal | "Tables, sounds, scripts and templates in the BIF archives"     |

## ERF archive

An ERF archive is a self-contained archive: unlike a BIF archive, it stores the name of each resource itself. The four texture packs in `TexturePacks/` are ERF archives. The reader is `ReadErf` in `kotor/kotor.go`. `patch.erf` is also an ERF archive, and `ReadErf` reads its tables without error. It holds 97 resources: 84 TPC textures, 7 GUI layouts, 4 2DA tables (type 2017), and the MDL and MDX resources of one model. The section "Patch archive: patch.erf" describes them. The 123 files in `lips/` are ERF archives with the signature "MOD V1.0" instead of "ERF V1.0" (see the lip sync section). A saved game also stores its resources in "MOD V1.0" archives (see "Saved games" below); the counts in this section do not include them. The files in `modules/` and `rims/` are not ERF archives (see the RIM section below).

An ERF archive begins with a 160-byte header. The evidence below covers 128 archives: the 4 texture packs, `patch.erf` and the 123 MOD archives in `lips/`.

| Offset | Type    | Meaning                                    | Evidence                                                                                                          | Confidence |
| ------ | ------- | ------------------------------------------ | ----------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "ERF V1.0" or "MOD V1.0"         | "ERF V1.0" in the texture packs and `patch.erf`; "MOD V1.0" in the 123 archives of `lips/`.                       | high       |
| 0x08   | u32     | Number of localized strings                | The value is 0 in all 128 archives, so the string format is UNKNOWN.                                              | low        |
| 0x0C   | u32     | Size of the localized string table         | The value is 0 in all 128 archives.                                                                               | low        |
| 0x10   | u32     | Number of resources                        | The key and resource tables parse consistently in all 128 archives.                                               | high       |
| 0x14   | u32     | Offset of the localized string table (160) | The value is 160 in all 128 archives.                                                                             | low        |
| 0x18   | u32     | Offset of the key table (160)              | The value is 160 in all 128 archives.                                                                             | high       |
| 0x1C   | u32     | Offset of the resource table               | Equals the key table offset plus 24 times the resource count in all 128 archives.                                 | high       |
| 0x20   | u32     | Build year minus 1900                      | The value is 103 (2003) in the texture packs and 104 (2004) in `patch.erf`. The KEY file also holds 103.          | medium     |
| 0x24   | u32     | Build day of the year                      | The value is 309 in the texture packs, 47 in `patch.erf`, and 309 or 145 in `lips/`. The KEY file also holds 309. | medium     |
| 0x28   | u32     | UNKNOWN, probably uninitialized            | The value is 0 in the texture packs, 0x45207A6C in `patch.erf`, and takes 56 different values in `lips/`.         | low        |
| 0x2C   | u8[116] | Padding                                    | All bytes are 0 in all 128 archives.                                                                              | medium     |

The build date fields agree between the KEY file and the archives built with it, and `patch.erf`, which was presumably built later, holds a later date. The two archives of the saved game hold year 126 (2026) and day 324, which is 20 November 2026 when 1 January is day 1. The save files were copied on 22 November 2026, so the fields are the date when the archive was written. Whether the day count starts at 0 or 1 is UNKNOWN. The field at 0x28 is 0 in both archives of the saved game. Because the game wrote them itself, a value other than 0 probably comes from the tool that built the other archives.

The key table has one 24-byte entry per resource.

| Offset | Type     | Meaning            | Evidence                                                | Confidence |
| ------ | -------- | ------------------ | ------------------------------------------------------- | ---------- |
| 0x00   | char[16] | Resref, NUL-padded | Names match the texture names that MDL meshes refer to. | high       |
| 0x10   | u32      | Resource ID        | Equals the entry's index for all 29,756 entries.        | high       |
| 0x14   | u16      | Resource type      | Every value is a known type (see below).                | high       |
| 0x16   | u16      | UNKNOWN, always 0  | The value is 0 in all 128 archives.                     | low        |

The resource table has one 8-byte entry per resource, in the same order as the key table.

| Offset | Type | Meaning                            | Evidence                                | Confidence |
| ------ | ---- | ---------------------------------- | --------------------------------------- | ---------- |
| 0x00   | u32  | Offset of the resource data        | Every resource lies inside its archive. | high       |
| 0x04   | u32  | Size of the resource data in bytes | Every resource lies inside its archive. | high       |

The 29,756 entries are the 11,453 of the texture packs, the 97 of `patch.erf` and the 18,206 of `lips/`.

In all 128 archives, resource data starts directly after the resource table, is stored in table order without gaps, and ends exactly at the end of the file. The data is not compressed.

The contents of the texture packs are listed below.

| Pack             | Resources | Contents                                                                                                          |
| ---------------- | --------- | ----------------------------------------------------------------------------------------------------------------- |
| swpc_tex_tpa.erf | 3,294     | 3,294 TPC textures at full resolution.                                                                            |
| swpc_tex_tpb.erf | 3,294     | The same 3,294 names as tpa. 2,724 are half the width and height of their tpa version; 570 are the same size.     |
| swpc_tex_tpc.erf | 3,294     | The same 3,294 names as tpa. 2,712 are a quarter of the width and height, 12 are half, and 570 are the same size. |
| swpc_tex_gui.erf | 1,571     | 1,570 TPC textures for the user interface and one resource of type 2022.                                          |

The tpb and tpc packs hold reduced copies of the tpa textures and are probably the lower texture quality settings. Most reduced textures are the lower mipmap levels of the tpa texture. All four packs are converted. The section "Reduced texture packs: tpb and tpc" below gives the details.

## RIM archive

A RIM archive is a self-contained archive like ERF, with a simpler header and a single table. All 246 files in `modules/` and `rims/` begin with "RIM V1.0". They hold 22,511 resources in total. The reader is `ReadRim` in `kotor/kotor.go`. The tool `cmd/unpack` writes the resources of one RIM, ERF or BIF archive to a folder. `ReadRim` checks that every resource lies inside the file in table order, starts on a multiple of 4 and does not overlap the next one, and that every byte between resources and after the last one is zero. All 246 files pass. The contents of the 234 archives in `modules/` are described in the section on modules below, and the contents of the 12 archives in `rims/` in the section "Global resources: rims/\*.rim".

A RIM archive begins with a 120-byte header.

| Offset | Type    | Meaning                                       | Evidence                                                                                                                                                          | Confidence |
| ------ | ------- | --------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "RIM V1.0"                          | Present in all 246 files.                                                                                                                                         | high       |
| 0x08   | u32     | UNKNOWN, always 0                             | The value is 0 in all 246 files.                                                                                                                                  | low        |
| 0x0C   | u32     | Number of resources                           | The table parses consistently in all 246 files. Four files have 0 resources.                                                                                      | high       |
| 0x10   | u32     | Offset of the resource table                  | The value is 120 in all 246 files.                                                                                                                                | high       |
| 0x14   | u32     | Flag, probably marking an archive of MDX data | The value is 1 in the six files whose names end in "dx", such as `globaldx.rim`, and 0 in the other 240. The four nonempty "dx" archives hold only MDX resources. | medium     |
| 0x18   | u8[96]  | Padding                                       | All bytes are 0 in all 246 files.                                                                                                                                 | medium     |

The resource table has one 32-byte entry per resource.

| Offset | Type     | Meaning                            | Evidence                                                    | Confidence |
| ------ | -------- | ---------------------------------- | ----------------------------------------------------------- | ---------- |
| 0x00   | char[16] | Resref, NUL-padded                 | Names are readable, for example "acbonus" and "aiscripts".  | high       |
| 0x10   | u32      | Resource type                      | 22 types occur (see below). Type 2047 (GUI) does not occur. | medium     |
| 0x14   | u32      | Resource ID                        | Equals the entry's index for all 22,511 entries.            | high       |
| 0x18   | u32      | Offset of the resource data        | Every resource lies inside its archive.                     | high       |
| 0x1C   | u32      | Size of the resource data in bytes | Every resource lies inside its archive.                     | high       |

Resources are not stored back to back. Every gap between resources consists of zero bytes, and every resource offset is a multiple of 4. The first resource starts 8 bytes after the resource table in 240 of the 242 nonempty files and 72 bytes after it in the other 2. In 22,021 of the 22,269 later gaps, the next resource starts 16 bytes after the end of the previous one, rounded up to a multiple of 4. The other 248 gaps are between 24 and 136 bytes long. In all 242 nonempty files, the last resource is followed by exactly 10 zero bytes before the end of the file. The purpose of the padding is UNKNOWN.

The 246 archives hold the 22 resource types below. The table gives the number of resources of each type and the signature at the start of the resource, where there is one.

| Type | Count  | Signature | Type | Count | Signature           |
| ---- | ------ | --------- | ---- | ----- | ------------------- |
| 4    | 375    | RIFF      | 2035 | 635   | UTS V3.2            |
| 2002 | 239    | none      | 2038 | 20    | FAC V3.2            |
| 2010 | 11,355 | NCS V1.0  | 2040 | 68    | UTE V3.2            |
| 2012 | 117    | ARE V3.2  | 2042 | 525   | UTD V3.2            |
| 2014 | 117    | IFO V3.2  | 2044 | 1,467 | UTP V3.2            |
| 2017 | 306    | 2DA V2.b  | 2051 | 38    | UTM V3.2            |
| 2023 | 117    | GIT V3.2  | 2056 | 3     | JRL V3.2            |
| 2025 | 498    | UTI V3.2  | 2058 | 2,049 | UTW V3.2            |
| 2027 | 1,753  | UTC V3.2  | 2060 | 210   | SSF V1.1            |
| 2029 | 1,135  | DLG V3.2  | 3003 | 132   | PTH V3.2            |
| 2032 | 1,100  | UTT V3.2  | 3008 | 252   | none, 153 are empty |

Types 2012, 2014 and 2023 each occur 117 times, once in each of the 117 main module archives (see the section on modules below). Types 4, 2002, 2017, 2060 and 3008 occur only in `rims/`.

# Generic file format: GFF

GFF (generic file format) is the tree container of most structured resources: user interface layouts, module files, object templates, the journal and the files of a saved game. The layout of the content is not fixed. Each value is a field that carries its own type and its own label. The first four bytes of a file name its content, called the file type here, such as "GUI " or "UTC ". The next four bytes are the version, which is "V3.2" in every file. The reader is `ReadGff` in `kotor/gff.go`.

The game holds 11,152 GFF files, listed in the table below. No other archive holds one.

| Source               | Files | File types                                                                     |
| -------------------- | ----- | ------------------------------------------------------------------------------ |
| `data/gui.bif`       | 84    | GUI                                                                            |
| `patch.erf`          | 7     | GUI                                                                            |
| `modules/`           | 9,712 | ARE, DLG, FAC, GIT, IFO, JRL, PTH, UTC, UTD, UTE, UTI, UTM, UTP, UTS, UTT, UTW |
| `rims/`              | 62    | UTI                                                                            |
| `data/templates.bif` | 1,277 | BIC, BTC, BTI, DLG, ITP, UTC, UTD, UTE, UTI, UTP, UTT, UTW                     |
| `data/_newbif.bif`   | 1     | JRL                                                                            |
| The saved game       | 9     | ARE, FAC, GIT, GVT, IFO, INV, NFO, PT, UTC                                     |

These are 25 file types in all. Each resource type always carries the same file type, and the lowercase file type is the extension that this document uses for the resource type, such as "utc" for type 2027.

All 11,152 files parse under the strict rules of the reader. The six regions chain from offset 56 to the end of the file. Every field and every struct except the root is used exactly once. No byte of the three data regions is used twice, and no read leaves its region.

## Header

A GFF file begins with a 56-byte header. After the signature, it consists of six pairs of u32 values, and each pair locates one region of the file. The evidence below holds for all 11,152 files.

| Offset | Type    | Meaning                                 | Evidence                                                                    | Confidence |
| ------ | ------- | --------------------------------------- | --------------------------------------------------------------------------- | ---------- |
| 0x00   | char[4] | File type, such as "GUI "               | Matches the resource type in every archive.                                 | high       |
| 0x04   | char[4] | Version, "V3.2"                         | Present in all files.                                                       | high       |
| 0x08   | u32     | Offset of the struct table (always 56)  | The value is 56 in all files.                                               | high       |
| 0x0C   | u32     | Number of structs                       | The next region starts at the struct table offset plus 12 bytes per struct. | high       |
| 0x10   | u32     | Offset of the field table               | Equals the struct table offset plus 12 times the struct count.              | high       |
| 0x14   | u32     | Number of fields                        | The next region starts at the field table offset plus 12 bytes per field.   | high       |
| 0x18   | u32     | Offset of the label table               | Equals the field table offset plus 12 times the field count.                | high       |
| 0x1C   | u32     | Number of labels                        | The next region starts at the label table offset plus 16 bytes per label.   | high       |
| 0x20   | u32     | Offset of the field data region         | Equals the label table offset plus 16 times the label count.                | high       |
| 0x24   | u32     | Size of the field data region in bytes  | The next region starts at its end.                                          | high       |
| 0x28   | u32     | Offset of the field index region        | Directly follows the field data region.                                     | high       |
| 0x2C   | u32     | Size of the field index region in bytes | The next region starts at its end.                                          | high       |
| 0x30   | u32     | Offset of the list index region         | Directly follows the field index region.                                    | high       |
| 0x34   | u32     | Size of the list index region in bytes  | The region ends exactly at the end of the file.                             | high       |

## Struct, field and label tables

A struct is a group of fields. Struct 0 is the root of the tree. Each entry of the struct table is 12 bytes.

| Offset | Type | Meaning                                                                                                                                                               | Evidence                                                                                                                | Confidence |
| ------ | ---- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u32  | Struct type                                                                                                                                                           | 0xFFFFFFFF on the root of all 11,152 files. See below.                                                                  | low        |
| 0x04   | u32  | For a struct with one field, the index of that field. Otherwise a byte offset into the field index region, where the struct's field indices are stored as u32 values. | Under this reading, every field is used exactly once in all files. The module files hold 40,631 structs with one field. | high       |
| 0x08   | u32  | Number of fields                                                                                                                                                      | Every field index that a struct reads is valid.                                                                         | high       |

No struct in any file has zero fields, so the encoding of an empty struct is UNKNOWN.

Each entry of the field table is 12 bytes.

| Offset | Type | Meaning                                                    | Evidence                                                          | Confidence |
| ------ | ---- | ---------------------------------------------------------- | ----------------------------------------------------------------- | ---------- |
| 0x00   | u32  | Field type (see below)                                     | Sixteen values occur. Each decodes consistently across all files. | high       |
| 0x04   | u32  | Index into the label table                                 | Every index is valid.                                             | high       |
| 0x08   | u32  | The value itself, or an offset or index, depending on type | See the field type table.                                         | high       |

Each entry of the label table is a 16-byte name, padded with NUL bytes, for example "EXTENT". A label does not determine the field type: in the GUI files, "TEXT" is a string in 1,631 fields and a struct in 2,016 fields.

The meaning of the struct type is UNKNOWN, but its values follow patterns. The root has 0xFFFFFFFF in every file. In the GUI files, all other structs have 0, except 17 structs in `debug.gui` with 14. In the module files, the struct type of a list item follows one of three patterns. In 30 list labels it is the same for every item of a list, and in 11 it equals the item's index in the list. In `Equip_ItemList` of the UTC files it takes values that are single bits, such as 2, 16 and 131,072, which suggests flags for equipment slots. The GIT lists of the modules use a fixed struct type per list (see "Contents of the module files").

## Field types

The table below lists the 18 field types. The count is the number of fields of the type in all 11,152 files. Types 0 to 5 and 8 store their value in the field entry itself, with signed types sign-extended to 32 bits. The other types store an offset into the field data or the list index region, or an index into the struct table.

| Type | Count   | Value                                                                                              | Evidence                                                                                                                                                                                                                                                           | Confidence |
| ---- | ------- | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------- |
| 0    | 403,541 | u8 stored in the field entry                                                                       | All values are at most 0xFF. In the GUI files the values are only 0, 1 and 2, with labels for flags such as PULSING and Obj_Locked.                                                                                                                                | medium     |
| 1    | 407     | i8 stored in the field entry                                                                       | Occurs only in the saved game. Values such as 0xFFFFFFFB (-5) occur in labels such as `Modifier` and `RefSaveThrow`. All values lie between -5 and 16.                                                                                                             | high       |
| 2    | 45,192  | u16 stored in the field entry                                                                      | All values are at most 0xFFFF. Labels include Feat, Spell and PortraitId.                                                                                                                                                                                          | medium     |
| 3    | 22,964  | i16 stored in the field entry                                                                      | In the module files all values lie between 0 and 1,000, with labels such as HitPoints and ClassLevel. In the saved game, 150 DamageValue fields hold 0xFFFFFFFF, which is -1 sign-extended.                                                                        | high       |
| 4    | 282,652 | u32 stored in the field entry                                                                      | In the GUI files it is used only by STRREF, where 0xFFFFFFFF marks no string.                                                                                                                                                                                      | medium     |
| 5    | 110,836 | i32 stored in the field entry                                                                      | GUI coordinates such as LEFT and WIDTH are plausible, and Obj_ParentID uses -1.                                                                                                                                                                                    | high       |
| 6    | 128     | u64: offset into field data, where 8 bytes are stored                                              | Occurs only in the saved game: in `Id` of effect lists (127 fields) and in `Mod_Effect_NxtId`. The 127 `Id` values are distinct and at most 504, the value of `Mod_Effect_NxtId`. Whether the type is signed is UNKNOWN, because no value is large enough to tell. | medium     |
| 7    | 0       | UNKNOWN, probably i64 in 8 bytes of field data                                                     | Occurs in no file. `ReadGff` reads it by analogy with types 0 to 6.                                                                                                                                                                                                | low        |
| 8    | 161,273 | f32 stored in the field entry                                                                      | In the GUI files ALPHA is always 1.0 and ROTATE is always 0.0.                                                                                                                                                                                                     | medium     |
| 9    | 0       | UNKNOWN, probably f64 in 8 bytes of field data                                                     | Occurs in no file. `ReadGff` reads it by analogy with type 8.                                                                                                                                                                                                      | low        |
| 10   | 256,816 | String: offset into field data, where a u32 length is followed by that many characters             | Decodes to tags and text such as "LBL_BARKTEXT".                                                                                                                                                                                                                   | high       |
| 11   | 360,371 | Resref: offset into field data, where a u8 length (at most 16) is followed by that many characters | Decodes to names of resources that exist in the game files (see "Links to other files" and "GUI layout content").                                                                                                                                                  | high       |
| 12   | 88,225  | Localized string: offset into field data (layout below)                                            | The size field matches the content in all fields. Labels include Text, Description and LocalizedName.                                                                                                                                                              | high       |
| 13   | 128     | Binary data: offset into field data, where a u32 length is followed by that many bytes             | In the module files it is used only by Mod_ID, where the length is always 16. The saved game also uses it for packed variables (see "Global variables: GLOBALVARS.res").                                                                                           | medium     |
| 14   | 10,770  | Struct: index into the struct table                                                                | Every non-root struct is referenced exactly once by a type 14 or type 15 field.                                                                                                                                                                                    | high       |
| 15   | 130,468 | List: offset into the list index region, where a u32 count is followed by that many struct indices | The list index region is covered exactly, except in the files described below.                                                                                                                                                                                     | high       |
| 16   | 1,177   | Orientation: offset into field data, where four f32 values are stored                              | Used only by Orientation in the GIT camera lists. The squared length of all 1,137 module values lies between 0.99999 and 1.00001, so the four values are probably a unit quaternion. Their order is UNKNOWN.                                                       | medium     |
| 17   | 7,869   | Vector: offset into field data, where three f32 values are stored                                  | In the GUI files it is used only by COLOR, whose components lie between 0 and 1, except the value (-1, -1, -1).                                                                                                                                                    | high       |

A localized string stores a string number and any number of literal substrings. Its layout in the field data is described below.

| Offset | Type   | Meaning                                         | Evidence                                                                                                                                                                                                 | Confidence |
| ------ | ------ | ----------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u32    | Number of bytes that follow this field          | Equals the length of the rest of the value in all 88,225 fields.                                                                                                                                         | high       |
| 0x04   | u32    | String number in `dialog.tlk`, or 0xFFFFFFFF    | Every number other than 0xFFFFFFFF names an entry with text, such as 35,782, "Taris - South Apartments", for the area name of `tar_m02aa`. 0xFFFFFFFF occurs where a literal substring is given instead. | high       |
| 0x08   | u32    | Number of substrings                            | The value is 0 in 77,091 and 1 in 170 of the 77,261 module fields.                                                                                                                                       | high       |
| 0x0C   | varies | Substrings: u32 ID, u32 length, then characters | All 170 substrings of the module files have ID 0. Their text is plausible, for example the IFO Mod_Name "tar_m02aa".                                                                                     | medium     |

The meaning of the substring ID is UNKNOWN, because only the value 0 occurs in the module files. It may encode a language.

In 34 files, some bytes of the field index or list index region are not referenced by any struct or list. These are 2 DLG, 11 GIT and 16 UTM files in the modules, with between 32 and 476 unreferenced bytes each, and the BIC file (44 bytes) and four ITP files (8 to 252 bytes) in `templates.bif`. The field data region never has unreferenced bytes. The unreferenced bytes are probably left over from editing, but this has not been confirmed. `ReadGff` therefore counts unreferenced bytes instead of rejecting the file. The GUI conversion still rejects them, and all 91 GUI files have none.

## Conversion to JSON

Every GFF file is converted to JSON by `gffJson` in `cmd/extract/json.go`. Each struct becomes an object keyed by field label in file order, a struct type other than 0 is kept as `__struct_type`, and lists become arrays. The values are written as described in the table below.

| Type      | JSON value                                                                                                                                                                                             |
| --------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| 0 to 6, 8 | A number. An f32 uses the shortest decimal that reads back as the same f32, and a 64-bit integer is written exactly.                                                                                   |
| 10, 11    | A string, decoded as Latin-1.                                                                                                                                                                          |
| 12        | An object with `strref`, the string number, or null for 0xFFFFFFFF; `text`, the text of that entry in `dialog.tlk`; and `substrings`, when there are any, as an array of objects with `id` and `text`. |
| 13        | A string of lowercase hexadecimal digits.                                                                                                                                                              |
| 14, 15    | An object, or an array of objects.                                                                                                                                                                     |
| 16, 17    | An array of four or three numbers in file order.                                                                                                                                                       |

The following example shows the area name in `extracted/modules/tar_m02aa/m02aa.are.json`.

```
	"Name": {
		"strref": 35782,
		"text": "Taris - South Apartments"
	},
```

The JSON does not record the binary field type. The type can still be recovered from the label: in the 91 GUI files each of the 55 combinations of label and JSON value kind occurs with only one binary type, and in the 9,712 module files each of the 675 combinations of file type, label and JSON value kind does. In the module files the hexadecimal string of type 13 occurs only in Mod_ID, so it cannot be confused with a text string. The unreferenced bytes of the 34 files above are not written.

The only string byte above 0x7F in the GUI files is 0xB1 in six strings, which is "±" in both Latin-1 and Windows-1252, so the choice of code page does not change the output.

# Modules: modules/\*.rim

A module is one playable location of the game, such as one district of a planet. The folder `modules/` holds 117 modules in 234 RIM archives, 70 MB in total. Each module consists of two archives: `<module>.rim`, called the main archive here, and `<module>_s.rim`, called the support archive here. The main archive describes the area and the objects placed in it. The support archive holds the templates, dialogs and scripts that those objects use. The area's 3D geometry is not in the module; it is in the BIF archives (see "Links to other files" below).

The reader is `ReadRim` in `kotor/kotor.go`. Every resource except the scripts is read with `ReadGff` in `kotor/gff.go`, and the scripts are read with `ParseNcs` in `kotor/ncs.go`. The `modules` stage of `cmd/extract` converts every resource to `extracted/modules/` and writes one report line per resource to `temp/reports/modules.txt` (see "Conversion and verification" below). All 234 archives pass: each RIM table parses, all 9,712 GFF resources parse with the signature that matches their resource type and version "V3.2", and all 10,863 scripts decode completely.

## Archive pairs

The two archives of a module divide the resource types without overlap. The table below gives the number of resources of each type in the 117 main archives and the 117 support archives. The extension is the lowercase content type at the start of the resource. Each type has exactly one signature in all 234 archives.

| Type | Extension | Main | Support | Probable content (see below) |
| ---- | --------- | ---- | ------- | ---------------------------- |
| 2010 | ncs       | 0    | 10,863  | Compiled script              |
| 2012 | are       | 117  | 0       | Area properties              |
| 2014 | ifo       | 117  | 0       | Module information           |
| 2023 | git       | 117  | 0       | Instances placed in the area |
| 2025 | uti       | 0    | 436     | Item template                |
| 2027 | utc       | 0    | 1,753   | Creature template            |
| 2029 | dlg       | 0    | 1,135   | Dialog                       |
| 2032 | utt       | 0    | 1,100   | Trigger template             |
| 2035 | uts       | 0    | 635     | Sound template               |
| 2038 | fac       | 0    | 20      | Factions                     |
| 2040 | ute       | 0    | 68      | Encounter template           |
| 2042 | utd       | 0    | 525     | Door template                |
| 2044 | utp       | 0    | 1,467   | Placeable object template    |
| 2051 | utm       | 0    | 38      | Store (merchant) template    |
| 2056 | jrl       | 0    | 3       | Journal                      |
| 2058 | utw       | 0    | 2,049   | Waypoint template            |
| 3003 | pth       | 0    | 132     | Path network                 |

Every main archive holds exactly three resources: `module.ifo`, `<area>.are` and `<area>.git`. A support archive holds between 5 and 513 resources; the median is 163. No archive holds the same name and type twice. In all 234 archives the header field at 0x14 is 0, and the first resource starts 8 bytes after the resource table.

The 20,575 resources have 16,059 distinct contents. 14,642 distinct name and type pairs occur. Of these, 12,569 occur in only one module, and 1,236 have different contents in different modules. For example, `invisible001.utp` occurs in 59 modules with 45 different contents. `module.ifo` occurs in all 117 modules with 117 different contents. A module therefore cannot rely on another module's copy of a resource with the same name.

## Module and area names

A module name begins with a prefix, followed in most cases by "m", a two-digit number and two letters, for example `tar_m02aa`. The prefixes and their counts are `tar` (23), `STUNT` (20), `tat` (12), `man` (11), `korr` (9), `unk` (9), `dan` (8), `kas` (8), `ebo` (5), `lev` (4), `sta` (4), `end` (2), `liv` (1) and `M12ab` (1). The prefixes name the planet or location. The area name of every module, a string of `dialog.tlk`, begins with the same place name for all modules of a prefix: "Taris" for `tar`, "Tatooine" for `tat`, "Manaan" for `man`, "Korriban" for `korr`, "Unknown World" for `unk`, "Dantooine" for `dan`, "Kashyyyk" for `kas`, "Leviathan" for `lev`, "Star Forge" for `sta`, "Endar Spire" for `end`, "Yavin Station" for `liv` and "Ebon Hawk" for `M12ab` and four of the five `ebo` modules. The fifth, `ebo_m46ab`, is named "Prison".

The 20 `STUNT` modules are probably stages for cutscenes, but this has not been confirmed in the game. Sixteen of them have the area name "Untitled" and four have an area code, "m40ad" or "M12aa", as their area name. Fifteen list one room with the placeholder name `****` among their rooms (see below), and their room layouts are copies of other areas (see "Room layouts: LYT").

The IFO file names the module's area in three places: the single entry of `Mod_Area_list` (Area_Name), `Mod_Entry_Area`, and the resrefs of the ARE and GIT files. These four names agree in all 117 modules, and `Mod_Area_list` has exactly one entry in all 117. Each module therefore has exactly one area.

In 86 modules, the area name is the module name without its prefix, for example area `m02aa` in module `tar_m02aa`. The other 31 are the 20 `STUNT` modules, whose areas have names such as `stunt_ebocom` or reuse another area's name such as `m40ad`, and 11 others: `danm13`, `danm15` and `danm16` (areas `m13aa`, `m15aa`, `m16aa`), `ebo_m40aa`, `ebo_m40ad` and `ebo_m41aa` (all area `m12aa`), `ebo_m46ab` (`m46aa`), `korr_m38aa` (`m38ab`), `korr_m38ab` (`m38aa`), `liv_m99aa` (`m50aa`) and `tat_m17ag` (`m17ae`). Several modules thus share an area name and its geometry.

Each of the 117 modules has a matching lip sync archive `lips/<module>_loc.mod`. The IFO field `Mod_VO_ID` is empty in 19 modules and equals the area name in 76. It is the module part of the names of the module's voice files: 30,923 of the 34,673 voice references in the module dialogs begin with "n" or "\_" followed by the `Mod_VO_ID` of their module (see "Voice-over: streamwaves/\*\*/\*.wav"). The others include references to shared conversations under `globe`, references to conversations of other modules, and the references of modules without a `Mod_VO_ID`. In 90 modules, `Mod_VO_ID` is also the name of a folder in `streamwaves/`.

## Contents of the module files

The probable content of each resource type in the table above comes from two sources: the field labels, and the GIT list that refers to the type. For example, the items of the GIT list "Creature List" name UTC templates, and a UTC file has fields such as FirstName, HitPoints and FeatList. The contents have not been confirmed in the running game. Only the main structure is described below; the individual fields are UNKNOWN unless stated.

The IFO file (module information) holds the entry area and the entry position (Mod_Entry_X, Mod_Entry_Y, Mod_Entry_Z) and direction (Mod_Entry_Dir_X, Mod_Entry_Dir_Y), the area list, a 16-byte Mod_ID, time settings such as Mod_DawnHour, and module script hooks such as Mod_OnPlrDeath.

The ARE file (area properties) holds the area name as a string number, map calibration points (the Map struct), fog, lighting and weather settings, script hooks such as OnEnter, and the Rooms list. Each Rooms item has a RoomName, which names a room model (see "Links to other files" below).

The GIT file (instances) lists the objects placed in the area. Each list holds items of one kind. The table below gives the list label, the struct type of its items, the number of items in all 117 files, and the template type that the items name. The items name their template in TemplateResRef, or in ResRef for stores.

| List label     | Struct type | Items | Template type |
| -------------- | ----------- | ----- | ------------- |
| Creature List  | 4           | 1,619 | utc           |
| Door List      | 8           | 724   | utd           |
| Encounter List | 7           | 51    | ute           |
| List           | 0           | 50    | uti           |
| Placeable List | 9           | 3,214 | utp           |
| SoundList      | 6           | 1,103 | uts           |
| StoreList      | 11          | 36    | utm           |
| TriggerList    | 1           | 996   | utt           |
| WaypointList   | 5           | 4,064 | utw           |
| CameraList     | 14          | 1,137 | none (camera) |

All 117 GIT files have all ten lists, some of them empty. The list "List" probably holds items lying in the area, because all 50 of its entries name UTI templates. CameraList items hold a Position vector and an Orientation (type 16) and name no template.

The DLG file (dialog) holds the lists EntriesList and RepliesList, with fields such as Speaker, Listener, Text (a localized string), VO_ResRef and Script. The FAC file holds FactionList and RepList, the JRL file holds Categories, and the PTH file holds Path_Points and Path_Conections. These spellings, including "Conections", are those of the files.

## Links to other files

Module files refer to other resources by resref. The table below shows where these references lead. "Module" means the module's own two archives, "key" means a resource listed in `chitin.key`, and "missing" means that neither these nor the archives in `rims/` hold the resource.

| Reference                                             | Count  | Module | Key    | Missing |
| ----------------------------------------------------- | ------ | ------ | ------ | ------- |
| GIT template (TemplateResRef or ResRef)               | 11,857 | 8,272  | 3,585  | 0       |
| Script (resref fields named On\*, Script\* or Active) | 37,806 | 13,365 | 24,380 | 61      |
| Conversation (Conversation fields)                    | 1,278  | 1,215  | 59     | 4       |
| ARE room model (RoomName as an MDL resource)          | 1,308  | 0      | 1,293  | 15      |

The 15 rooms without a model all have the placeholder name `****`, and all are in `STUNT` modules. Every other room name resolves to an MDL resource in `chitin.key`. The 61 missing scripts and 4 missing conversations are probably unused references. For example, `dan13_cort.dlg` in `danm13` names scripts such as `k_pdan_cort01` that exist nowhere.

The area's geometry and layout are in BIF archives under the area name. The table below lists the resource types that hold them.

| Type | Archive               | Content                                                                                           |
| ---- | --------------------- | ------------------------------------------------------------------------------------------------- |
| 3000 | `data/layouts.bif`    | Room layout (LYT), described in the section "Room layouts: LYT".                                  |
| 3001 | `data/lightmaps*.bif` | Room visibility list (VIS), described in the section "Room visibility lists: VIS".                |
| 2016 | `data/models.bif`     | Area room walkmesh (BWM), named after its room model, described in the section "Walkmeshes: BWM". |

## Compiled scripts: NCS

An NCS resource (type 2010) is a compiled script. It holds bytecode for a stack machine that runs NWScript, the game's scripting language. Module objects run these scripts through their script fields, such as OnEnter. The decoder is `ParseNcs` in `kotor/ncs.go`. It decodes all 10,863 scripts in the modules and all 1,784 scripts in `data/scripts.bif`, and no script fails.

An NCS file is the signature "NCS V1.0" followed by instructions up to the end of the file. There is no other header. The byte 0x42 at offset 0x08 is the first instruction, which carries the file size. Every operand is big-endian, unlike the other formats in this document.

| Offset | Type    | Meaning                                                  | Evidence                                                                                                                              | Confidence |
| ------ | ------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "NCS V1.0"                                     | Present in all 12,647 scripts.                                                                                                        | high       |
| 0x08   | u8      | Opcode 0x42, called T here: the program size instruction | The value is 0x42 in all 12,647 scripts. Opcode 0x42 occurs nowhere else, and the instructions that follow decode to the end of file. | high       |
| 0x09   | u32 BE  | Size of the resource in bytes                            | Equals the resource size in all 12,647 scripts.                                                                                       | high       |
| 0x0D   | varies  | Further instructions                                     | Decoded as described below.                                                                                                           | high       |

### Instructions

An instruction is a u8 opcode, a u8 type and the operands. The type names the kind of the values the instruction works on. Only the T instruction has no type byte. The decoder accepts a script only if every instruction has a known opcode, a type that the opcode accepts, and operands that end inside the file; if the last instruction ends exactly at the end of the file; and if every jump lands on the first byte of an instruction. All 12,647 scripts meet these rules.

The opcode names and operand layouts come from the published description of the NWScript virtual machine of Neverwinter Nights. The game files do not contain them. They were taken as a hypothesis and tested against the files. The evidence is that all 5,680,941 instructions of the 12,647 scripts decode under these layouts with no byte left over, and that all 817,355 jumps land on instructions. A wrong operand size would shift every following instruction, so the decoding would fail.

The table below lists the opcodes. The operand letters stand for i32, u32, i16, u16 and u8; "jump" is an i32 offset relative to the start of the instruction, and "routine" is a u16 routine number followed by a u8 argument count. The count is the number of instructions in all 12,647 scripts. The meanings are those of the published description and have not been confirmed in the running game, except where the evidence column says otherwise.

| Opcode | Name       | Types            | Operands              | Count     | Meaning                                                                   | Evidence and confidence                                                                                        |
| ------ | ---------- | ---------------- | --------------------- | --------- | ------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- |
| 0x01   | CPDOWNSP   | 1                | i32 offset, u16 size  | 690,883   | Copy the top of the stack down to an offset                               | Decodes exactly. High.                                                                                         |
| 0x02   | RSADD      | I F S O E0–E3    | none                  | 611,495   | Reserve a stack slot for a value of the type                              | Decodes exactly. High.                                                                                         |
| 0x03   | CPTOPSP    | 1                | i32 offset, u16 size  | 589,468   | Copy a value from an offset to the top of the stack                       | Decodes exactly. High.                                                                                         |
| 0x04   | CONST      | I F S O          | value of the type     | 1,144,589 | Push a constant                                                           | Strings are plausible text such as "Carth". High.                                                              |
| 0x05   | ACTION     | 0                | routine               | 336,059   | Call an engine routine                                                    | Every routine number exists in `nwscript.nss`, and the argument counts fit the declarations (see below). High. |
| 0x06   | LOGAND     | II               | none                  | 64,611    | Logical and                                                               | Decodes exactly. Medium.                                                                                       |
| 0x07   | LOGOR      | II               | none                  | 47,941    | Logical or                                                                | Decodes exactly. Medium.                                                                                       |
| 0x08   | INCOR      | II               | none                  | 26        | Bitwise or                                                                | Decodes exactly. Medium.                                                                                       |
| 0x09   | EXCOR      | II               | none                  | 0         | Bitwise exclusive or                                                      | Never occurs. UNKNOWN.                                                                                         |
| 0x0A   | BOOLAND    | II               | none                  | 12        | Bitwise and                                                               | Decodes exactly. Medium.                                                                                       |
| 0x0B   | EQUAL      | pairs            | none; u16 size for TT | 169,313   | Compare for equality                                                      | Decodes exactly. Medium.                                                                                       |
| 0x0C   | NEQUAL     | pairs            | none; u16 size for TT | 13,136    | Compare for inequality                                                    | One NEQUALTT with size 12 decodes exactly. Medium.                                                             |
| 0x0D   | GEQ        | II FF            | none                  | 2,920     | Greater than or equal                                                     | Decodes exactly. Medium.                                                                                       |
| 0x0E   | GT         | II FF            | none                  | 20,806    | Greater than                                                              | Decodes exactly. Medium.                                                                                       |
| 0x0F   | LT         | II FF            | none                  | 12,130    | Less than                                                                 | Decodes exactly. Medium.                                                                                       |
| 0x10   | LEQ        | II FF            | none                  | 5,928     | Less than or equal                                                        | Decodes exactly. Medium.                                                                                       |
| 0x11   | SHLEFT     | II               | none                  | 1         | Shift left                                                                | One occurrence. Low.                                                                                           |
| 0x12   | SHRIGHT    | II               | none                  | 0         | Shift right                                                               | Never occurs. UNKNOWN.                                                                                         |
| 0x13   | USHRIGHT   | II               | none                  | 0         | Unsigned shift right                                                      | Never occurs. UNKNOWN.                                                                                         |
| 0x14   | ADD        | arithmetic pairs | none                  | 38,938    | Add; SS concatenates strings                                              | Decodes exactly. Medium.                                                                                       |
| 0x15   | SUB        | arithmetic pairs | none                  | 2,223     | Subtract                                                                  | Decodes exactly. Medium.                                                                                       |
| 0x16   | MUL        | arithmetic pairs | none                  | 3,090     | Multiply                                                                  | Decodes exactly. Medium.                                                                                       |
| 0x17   | DIV        | arithmetic pairs | none                  | 1,974     | Divide                                                                    | Decodes exactly. Medium.                                                                                       |
| 0x18   | MOD        | II               | none                  | 1         | Remainder                                                                 | One occurrence. Low.                                                                                           |
| 0x19   | NEG        | I F              | none                  | 23,848    | Negate                                                                    | Decodes exactly. Medium.                                                                                       |
| 0x1A   | COMP       | I                | none                  | 0         | Bitwise complement                                                        | Never occurs. UNKNOWN.                                                                                         |
| 0x1B   | MOVSP      | 0                | i32                   | 849,059   | Move the stack pointer, which removes values                              | All values are negative multiples of 4. High.                                                                  |
| 0x1D   | JMP        | 0                | jump                  | 301,917   | Jump                                                                      | All targets are instructions. High.                                                                            |
| 0x1E   | JSR        | 0                | jump                  | 142,691   | Call a subroutine of the script                                           | All targets are instructions. High.                                                                            |
| 0x1F   | JZ         | 0                | jump                  | 350,504   | Jump if the top of the stack is zero                                      | All targets are instructions. High.                                                                            |
| 0x20   | RETN       | 0                | none                  | 79,559    | Return from a subroutine                                                  | All 12,647 scripts contain one. High.                                                                          |
| 0x21   | DESTRUCT   | 1                | u16, i16, u16         | 2,480     | Remove a block from the stack but keep part of it                         | Decodes exactly. Medium.                                                                                       |
| 0x22   | NOT        | I                | none                  | 13,668    | Logical not                                                               | Decodes exactly. Medium.                                                                                       |
| 0x23   | DECISP     | I                | i32 offset            | 11,065    | Decrement an integer on the stack                                         | Decodes exactly. Medium.                                                                                       |
| 0x24   | INCISP     | I                | i32 offset            | 11,180    | Increment an integer on the stack                                         | Decodes exactly. Medium.                                                                                       |
| 0x25   | JNZ        | 0                | jump                  | 22,243    | Jump if the top of the stack is not zero                                  | All targets are instructions. High.                                                                            |
| 0x26   | CPDOWNBP   | 1                | i32 offset, u16 size  | 2,471     | Copy down relative to the base pointer (global variables)                 | Decodes exactly. Medium.                                                                                       |
| 0x27   | CPTOPBP    | 1                | i32 offset, u16 size  | 75,275    | Copy up relative to the base pointer                                      | Decodes exactly. Medium.                                                                                       |
| 0x28   | DECIBP     | I                | i32 offset            | 0         | Decrement a global integer                                                | Never occurs. UNKNOWN.                                                                                         |
| 0x29   | INCIBP     | I                | i32 offset            | 0         | Increment a global integer                                                | Never occurs. UNKNOWN.                                                                                         |
| 0x2A   | SAVEBP     | 0                | none                  | 4,497     | Set the base pointer for global variables                                 | Occurs exactly as often as RESTOREBP. Medium.                                                                  |
| 0x2B   | RESTOREBP  | 0                | none                  | 4,497     | Restore the base pointer                                                  | Occurs exactly as often as SAVEBP. Medium.                                                                     |
| 0x2C   | STORESTATE | 0x10             | u32, u32              | 17,796    | Save the state for a deferred action, such as an argument of DelayCommand | Each one is followed by a forward JMP. Medium.                                                                 |
| 0x2D   | NOP        | 0                | none                  | 0         | No operation                                                              | Never occurs. UNKNOWN.                                                                                         |
| 0x42   | T          | none             | u32 size              | 12,647    | Program size                                                              | Only at offset 0x08. High.                                                                                     |

Opcode 0x1C does not occur and is not accepted by the decoder. Opcodes with a count of 0 are accepted with the published layout, but this layout is unverified.

The type byte takes the values in the table below. The names are those of the listing. Types 0 and 1 carry no meaning: each opcode always uses the same one of the two, and no opcode uses both. The binary operations use a pair type, whose two letters name the types of the two operands. The decoder accepts some pairs that do not occur, such as VV for ADD. These are part of the published description and are unverified. In particular, no vector operation occurs in any of the 12,647 scripts.

| Type       | Name                 | Meaning                                                                           | Evidence                                                                                                                    |
| ---------- | -------------------- | --------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| 0x00, 0x01 | none                 | No value type                                                                     | See above.                                                                                                                  |
| 0x03–0x06  | I F S O              | Integer, float, string, object                                                    | CONST reads an i32, an f32, a u16-prefixed string and an i32 for them. The strings are plausible text, for example "Carth". |
| 0x10–0x13  | E0–E3                | Engine structures 0 to 3: effect, event, location and talent                      | `nwscript.nss` defines ENGINE_STRUCTURE_0 to 3 with these names. The order of the type codes is assumed to follow it.       |
| 0x20–0x26  | II FF OO SS TT IF FI | Pairs of the types above. TT is a structure, whose size in bytes follows as a u16 | Decode exactly. The meaning of each pair is taken from the published description.                                           |
| 0x30–0x33  | E0E0–E3E3            | Pairs of engine structures                                                        | One NEQUALE2E2 occurs.                                                                                                      |
| 0x3A–0x3C  | VV VF FV             | Pairs with vectors                                                                | Never occurs. UNKNOWN.                                                                                                      |

STORESTATE always has type 0x10, and its two operands are probably the stack sizes to save. The most common values are 472 and 4, 472 and 52, and 108 and 28.

### Program structure

All scripts share one entry pattern. After the T instruction, 8,166 of the 12,647 scripts begin with JSR to the main function, followed by RETN. The other 4,481 begin with RSADDI, then JSR and RETN. These reserve an integer for a return value, so they are conditional scripts, which return whether something applies. The fields that name the scripts confirm this. The dialog field `Active`, which decides whether a dialog line is available, names a script of the second kind in 11,614 of its 11,665 references in the module files. The event fields, such as `OnEnter` and `ScriptHeartbeat`, name a script of the first kind in 26,212 of 26,293 references. 4,497 scripts contain SAVEBP, each exactly once, which probably marks scripts with global variables.

### Engine routines: nwscript.nss

ACTION calls an engine routine by number. The names come from `nwscript.nss`, an NWScript source file in `data/scripts.bif` with resource type 2009. The other 1,773 resources of that type are script sources (see "NWScript sources: NSS"). `nwscript.nss` declares constants and 772 routines. Routine n is the n-th function declaration in the file. The evidence is that 771 declarations are preceded by a comment "// n:" with the same n. The exception is routine 28, GetFacing, whose comment says "// 0:".

The argument count of ACTION counts each argument once, including vectors, which take three stack values, and action arguments, which are passed through STORESTATE. For example, AssignCommand(object, action) is always called with 2 and Location(vector, float) with 2. The count lies between the number of required parameters and the number of all parameters for all but 16 of the 336,059 calls. The 16 others are 12 calls of AddPartyMember with 1 argument (declared with 2), 2 of ActionBarkString with 2 (declared with 1), 1 of StartCreditSequence with 2 (declared with 1) and 1 of EffectBlasterDeflectionIncrease with 2 (declared with 1). Fifteen of them are in module scripts, such as `k_act_carthjoin` in `tar_m02aa`. These scripts were probably compiled against an older version of `nwscript.nss`. In one of them, `nw_s0_lghtnbolt` in `scripts.bif`, the call is also absent from the script's own source, which confirms that the compiled script does not match the shipped source. The decoder reports these calls as warnings, not errors.

The module scripts call 401 of the 772 routines. The most frequent calls are GetIsObjectValid (22,673), GetHasSpell (20,161) and GetBaseItemType (12,102).

## Conversion and verification

The `modules` stage of `cmd/extract` converts every resource of the 234 archives. Its output keeps the layout of the game files: one folder `extracted/modules/<archive>/` per archive, named after the archive without `.rim`, so a module has a main folder such as `tar_m02aa` and a support folder such as `tar_m02aa_s`. The report is `temp/reports/modules.txt`, with one line per resource. All 20,575 resources pass.

A GFF resource becomes `<resref>.<ext>.json` in the JSON form described under "Generic file format: GFF", for example `m02aa.are.json`. The extension stays in the name because the ARE and GIT files of an area share a resref. The report lines of the 29 module files with unreferenced bytes give the number of those bytes.

The localized strings resolve completely. The module files hold 77,261 localized strings. Of these, 34,261 have no string number, and the other 43,000 name 33,882 distinct entries of `dialog.tlk`. Every one of these entries exists and has text. The largest string number is 49,137, below the 49,265 entries of the table.

A compiled script becomes `<resref>.ncs.txt`, a disassembly listing. No standard format exists for NWScript bytecode, so a text listing is the most widely readable form. The following example shows `extracted/modules/tar_m02aa_s/k_act_carthjoin.ncs.txt`.

```
; warning: ACTION AddPartyMember at 0x29 passes 1 arguments
00000008  T            54
0000000d  JSR          8                 ; -> 00000015
00000013  RETN
00000015  CONSTI       0
0000001b  CONSTS       "Carth"
00000024  ACTION       200 2             ; object GetObjectByTag(string, int)
00000029  ACTION       574 1             ; int AddPartyMember(int, object)
0000002e  MOVSP        -4
00000034  RETN
```

Each line holds the offset of the instruction in hexadecimal, the mnemonic and the operands in decimal. The mnemonic is the opcode name, followed by the type name when the opcode accepts more than one type. A float is written as the shortest decimal that reads back as the same f32, and a string is written as a quoted Go string literal. A comment after ";" gives the target of a jump or the declaration of the called routine. Warnings come first, as lines that begin with ";". The listing therefore holds every byte of the script.

The `modules` stage of `cmd/verify` writes two reports. In `temp/reports/jq_modules.txt`, jq parses all 9,712 JSON files without error. This shows only that the files are valid JSON. In `temp/reports/ncs_modules.txt`, each listing is assembled again from its text, and the result must equal the source script byte for byte. All 10,863 listings pass. No independent tool reads NCS, so the assembler is part of `cmd/verify`. It shares only the tables of opcodes, types and operand layouts with the disassembler. The listings are therefore shown to be lossless, but the tables themselves are verified only by the decoding evidence described under "Compiled scripts: NCS". A test in which one character of a string constant was altered in one listing made that listing fail.

# Global resources: rims/\*.rim

The folder `rims/` holds 12 RIM archives with resources that do not belong to one module: models, sounds, tables, sound sets, item templates and scripts. Every one of these resources is a copy of a resource in the BIF archives, so the archives probably let the game load a fixed set of resources at once. The archives are read with `ReadRim` in `kotor/kotor.go`. Three formats occur here that the module archives do not hold: the 2DA table, the SSF sound set and the WAV sound. The `rims` stage of `cmd/extract` converts every resource (see "Conversion and verification" below).

## Archives

The archives form six pairs. Each pair consists of an archive `<name>.rim` and an archive `<name>dx.rim`. The "dx" archive holds the MDX resources of the models in the other archive and nothing else, and its header field at 0x14 is 1. The table below lists the 12 archives with their sizes and contents.

| Archive          | Size in bytes | Resources | Contents                                            |
| ---------------- | ------------- | --------- | --------------------------------------------------- |
| chargen.rim      | 2,555,540     | 30        | 30 MDL, the player heads `pfha01` to `pmhc05`       |
| chargendx.rim    | 1,969,674     | 30        | 30 MDX                                              |
| global.rim       | 20,718,578    | 949       | 246 NCS, 246 WAV, 168 MDL, 153 2DA, 105 SSF, 31 UTI |
| globaldx.rim     | 630,666       | 180       | 180 MDX                                             |
| legal.rim        | 120           | 0         | none                                                |
| legaldx.rim      | 120           | 0         | none                                                |
| mainmenu.rim     | 387,869       | 1         | 1 MDL, `mainmenu`                                   |
| mainmenudx.rim   | 265,082       | 2         | 2 MDX, `mainmenu` and `cgmain_light`                |
| miniglobal.rim   | 12,282,394    | 704       | 246 NCS, 129 WAV, 40 MDL, 153 2DA, 105 SSF, 31 UTI  |
| miniglobaldx.rim | 224,266       | 40        | 40 MDX                                              |
| subglobal.rim    | 120           | 0         | none                                                |
| subglobaldx.rim  | 120           | 0         | none                                                |

The four empty archives consist only of the 120-byte header. `miniglobal.rim` is a subset of `global.rim`: each of its 704 resources occurs in `global.rim` with the same name, type and bytes. Likewise, each of the 40 resources of `miniglobaldx.rim` occurs in `globaldx.rim` with the same bytes. Why both archives exist is UNKNOWN.

The resources of `global.rim`, `globaldx.rim`, `chargen.rim`, `chargendx.rim`, `mainmenu.rim` and `mainmenudx.rim` were compared with the resources of the same name and type in the BIF archives. Every resource has such a counterpart. All are byte-identical except 13 2DA tables, which differ only in their separator bytes (see "2DA table" below). The counterparts are in `2da.bif` (153 tables), `sounds.bif` (246 sounds), `scripts.bif` (246 scripts), `templates.bif` (105 sound sets and 31 item templates), `models.bif` (159 MDL and 172 MDX), `items.bif` (10 MDL and 10 MDX) and `player.bif` (30 MDL and 30 MDX).

Thirteen MDX resources have no MDL resource in the partner archive. Eleven of them, `fxc_droid01` to `fxc_droid17` with gaps, are in `globaldx.rim`, and their models are in `models.bif`. The other two, `fxmuzzle` in `globaldx.rim` and `cgmain_light` in `mainmenudx.rim`, are empty. These 13 resources are not converted.

## 2DA table

A 2DA file (type 2017) is a table with labelled columns and labelled rows, called a two-dimensional array. The game uses such tables for rules data, for example `appearance` (creature appearances) and `baseitems` (item types). All 153 tables in `global.rim` begin with the 9-byte signature "2DA V2.b" followed by a line feed. This is a binary form; the text form "2DA V2.0" does not occur. The parser is `ParseTwoDa` in `kotor/twoda.go`.

The file consists of six consecutive parts with no padding, described in the table below. Offsets depend on the lengths of the labels, so the table gives the order of the parts instead.

| Part | Type                  | Meaning                                                                        | Evidence                                                                                                                    | Confidence |
| ---- | --------------------- | ------------------------------------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 1    | char[9]               | Signature "2DA V2.b" and a line feed                                           | Present in all 153 tables.                                                                                                  | high       |
| 2    | strings, then u8 0    | Column labels, each followed by a separator byte, then one NUL byte            | The labels are readable names such as "label" and "name_strref". The NUL byte follows the last separator in all 153 tables. | high       |
| 3    | u32                   | Number of rows n                                                               | The following parts parse to the end of the file with this count in all 153 tables.                                         | high       |
| 4    | strings               | n row labels, each followed by the separator                                   | See above. The labels are "0" to "n-1" in 141 tables. The other 12 use other labels, such as "default" in `loadscreens`.    | high       |
| 5    | u16[n × columns]      | Offset of each cell's string within the string data, row by row                | Every offset is the start of a string in all 101,114 cells.                                                                 | high       |
| 6    | u16, then string data | Size of the string data in bytes, then the string data: NUL-terminated strings | The string data ends exactly at the end of the file in all 153 tables. The largest size is 17,629 bytes.                    | high       |

The separator is a tab in 140 tables and a NUL byte in the other 13: `appearance`, `appearancesndset`, `baseitems`, `bodybag`, `doortypes`, `genericdoors`, `heads`, `inventorysnds`, `placeableobjsnds`, `placeables`, `portraits`, `soundset` and `traps`. The copies of these 13 tables in `2da.bif` use tabs. Each RIM copy has the same size as its BIF copy and differs from it only where the BIF copy has a separator tab: for example, `appearance` has 80 columns and 509 rows, and exactly 589 bytes differ. Tab bytes that are part of a cell offset are unchanged. Why these 13 tables were written differently is UNKNOWN. A reader finds the separator as the first tab or NUL byte after the signature.

The string data stores each distinct string once, and several cells may refer to the same string. In all 153 tables, every string is referred to by at least one cell, no string occurs twice, and the strings appear in the order of the first cell that refers to them, row by row. An empty cell refers to an empty string; 37,256 of the 101,114 cells are empty. The text form of 2DA writes such cells as "\*\*\*\*", but that value does not occur here. All bytes of the labels and strings are below 0x80.

The tables have between 1 and 80 columns (median 4) and between 0 and 1,185 rows (median 12). Three tables have no rows: `iprp_base1`, `iprp_slotscost` and `iprp_staminacost`. Their string data is empty, so they end with the size 0.

## SSF sound set

An SSF file (type 2060) is a sound set: it names the voice line that a creature type speaks for each of a fixed list of game events, such as a battle cry or a death cry. All 105 files in `global.rim` begin with the signature "SSF V1.1". The parser is `ParseSsf` in `kotor/ssf.go`.

The file begins with a 12-byte header, described in the table below.

| Offset | Type    | Meaning                    | Evidence                                                                                           | Confidence |
| ------ | ------- | -------------------------- | -------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "SSF V1.1"       | Present in all 105 files.                                                                          | high       |
| 0x08   | u32     | Offset of the slot table   | The value is 12 in all 105 files, so the table follows the header directly.                        | medium     |
| 0x0C   | u32[]   | One string number per slot | The table ends exactly at the end of the file in all 105 files. 97 files have 40 slots, 8 have 49. | high       |

Each slot holds the number of an entry in `dialog.tlk`, or 0xFFFFFFFF for an empty slot. Every number in the 105 files names an existing entry. The entry gives the sound to play and a text. For creature sounds the text is a description, for example "Battlecry 1 - Do not translate this text" with the sound `c_bantha_bat1`. For party members the text is the spoken line, for example "Yes?".

The meaning of each slot follows from two independent sources: the texts of the entries it names, and the sound names of those entries, which end in a suffix such as `_bat1` or `_slct2`. The table below gives, for each slot range, the most common suffix with the number of uses that carry it among all uses whose entry names a sound, and an example text. In a few files one or two entries carry the suffix of a neighbouring slot, such as one `_slct` sound in slot 5. The slot meanings are not confirmed by any other source.

| Slots | Meaning                                 | Sound name suffixes                                                         | Example texts                                   | Confidence |
| ----- | --------------------------------------- | --------------------------------------------------------------------------- | ----------------------------------------------- | ---------- |
| 0–5   | Battle cries                            | `bat` in 265 of 266 uses                                                    | "Battlecry 1 - Do not translate this text"      | high       |
| 6–8   | Responses when the creature is selected | `slct` in 37 of 39 uses                                                     | "Yes?", "What?", "Huh?"                         | high       |
| 9–11  | Attack grunts                           | `atk` in 268 of 274 uses                                                    | "Attack Grunt 1 - Do not translate this text"   | high       |
| 12–13 | Pain grunts                             | `hit` in 177 of 184 uses                                                    | "Pain Grunt 1 - Do not translate this text"     | high       |
| 14    | Low health                              | `low` in 11 of 14 uses                                                      | "Can't take much more of this!"                 | medium     |
| 15    | Death                                   | `dead` in 86 of 92 uses                                                     | "Death - Do not translate this text"            | high       |
| 16    | Critical hit                            | `crit` in 11 of 14 uses                                                     | "Take that!"                                    | medium     |
| 17    | Target is immune                        | `tia` in 11 of 13 uses                                                      | "I am doing no damage!"                         | medium     |
| 18    | Mine laid; in eight files, death        | `lmin` in 11 uses, `die` or `DIE` in 8                                      | "Mine successfully laid.", "Player Death Sound" | medium     |
| 19    | Mine disarmed                           | `dmin` in 12 of 13 uses                                                     | "Disarmed."                                     | medium     |
| 20    | Stealth begins                          | `stlh` in 11 of 12 uses                                                     | "I shall move unseen."                          | medium     |
| 21    | Search begins                           | `srch` in 12 of 13 uses                                                     | "I shall be attentive."                         | medium     |
| 22    | Lock picking begins                     | `block` in 12 of 13 uses                                                    | "Affirmative."                                  | medium     |
| 23    | Lock picking fails                      | `flock` in 12 of 13 uses                                                    | "Damn!"                                         | medium     |
| 24    | Lock picking succeeds                   | `slock` in 12 of 13 uses                                                    | "Opened."                                       | medium     |
| 25    | Separated from the party                | `sprty` in 12 of 13 uses                                                    | "Hang back here for a minute."                  | medium     |
| 26    | Rejoins the party                       | `rprty` in all 13 uses                                                      | "Hey! Back again!"                              | medium     |
| 27    | Poisoned                                | `pois` in all 13 uses                                                       | "Agh. I think I've been poisoned..."            | medium     |
| 28–48 | UNKNOWN                                 | Empty in all files except slot 33, which holds a `slct` sound in two files. | none                                            | low        |

The death sound in slot 18 occurs in the eight files with 49 slots: `darkjedi`, `drdassassin`, `drdmkone`, `drdprobe`, `katarn`, `kinrath`, `player` and `sithtroop`. In each of them only slot 18 is used, and the purpose of the nine extra slots is UNKNOWN. Whether the game reads slot 18 of these files as a death sound or as a mine sound is UNKNOWN.

## WAV sounds

The 246 sounds (type 4) in `global.rim` are ordinary RIFF WAVE files. Unlike the files in `streamsounds/`, they have no MP3 stub before the RIFF header: the RIFF size equals the file size minus 8 in all 246 files, and the chunks end exactly at the end of the file. All are mono. The table below gives the encodings.

| Encoding                     | Sample rate | Files | Chunks                                  |
| ---------------------------- | ----------- | ----- | --------------------------------------- |
| 16-bit PCM (format 1)        | 22,050 Hz   | 233   | fmt, data, and in 16 files a LIST chunk |
| 16-bit PCM (format 1)        | 44,100 Hz   | 6     | fmt, data                               |
| 16-bit PCM (format 1)        | 11,025 Hz   | 3     | fmt, data, LIST                         |
| IMA ADPCM (format 17), 4-bit | 44,100 Hz   | 4     | fmt (20 bytes), fact, data              |

The four IMA ADPCM files are `cb_sh_blast1`, `cb_sh_hvybl1`, `cb_sh_medbl1` and `cb_sh_sonbl1`. Their blocks are 1,024 bytes long and decode with `DecodeAdpcm` in `kotor/adpcm.go`, which also decodes the stream sounds. Most names begin with `cb_` (92 files, probably combat), `fs_` (67, probably footsteps) or `v_` (45, probably visual effects).

## Conversion and verification

The `rims` stage of `cmd/extract` writes one folder `extracted/rims/<archive>/` per archive without "dx" and writes the report `temp/reports/rims.txt`. The report has one line per archive and one line per resource, except the MDX resources, which are read together with their models. All 1,696 lines pass: the 12 archive tables, 239 models, 375 sounds, 306 tables, 210 sound sets, 62 item templates and 492 scripts. The resources become the files below.

| Resource     | Output                         | Form                                                                                                                           |
| ------------ | ------------------------------ | ------------------------------------------------------------------------------------------------------------------------------ |
| MDL with MDX | `<resref>.obj`, `<resref>.mtl` | As for `data/models.bif` (see "3D models: MDL and MDX"). 151 models have no rendered geometry and are not written.             |
| WAV          | `<resref>.wav`                 | 16-bit PCM. PCM files are copied unchanged, and the four IMA ADPCM files are decoded.                                          |
| 2DA          | `<resref>.2da.csv`             | CSV. The first line holds an empty cell and the column labels. Each further line holds the row label and the cells.            |
| SSF          | `<resref>.ssf.json`            | JSON with an array `slots`. Each slot has `strref` (null when empty) and the `text` and `sound` of that entry in `dialog.tlk`. |
| UTI          | `<resref>.uti.json`            | The JSON form described under "Generic file format: GFF".                                                                      |
| NCS          | `<resref>.ncs.txt`             | The disassembly listing of the module files.                                                                                   |

The models refer to textures by the same relative path as the models of `data/models.bif`, and every texture they name is found in the texture packs. The CSV files do not record the separator byte of the source table. The following example shows `extracted/rims/global/aiscripts.2da.csv`.

```
,label,name_strref,description_strref,aistate
0,DEFAULT,1116,1109,0
1,GRENADE,1120,1112,4
2,JEDI,1121,1114,5
```

The `rims` stage of `cmd/verify` writes five reports, and every line passes. In `temp/reports/assimp_rims.txt`, assimp loads all 88 OBJ files with the face count of the file. In `temp/reports/ffprobe_rims.txt`, ffprobe reads all 375 WAV files as 16-bit PCM (`pcm_s16le`), and ffmpeg decodes each output and its source to 16-bit samples, which must be equal. For the four IMA ADPCM sources, the samples may differ by rounding (see "Conversion and verification" under "Tables, sounds, scripts and templates in the BIF archives"). In `temp/reports/csv_rims.txt`, Go's `encoding/csv` reader reads each of the 306 CSV files, and the table built again from it must equal the source table byte for byte. The builder is part of `cmd/verify` and shares no code with the parser; it takes the separator byte from the source. A test in which one cell of `aiscripts.2da.csv` was altered made that file fail. In `temp/reports/jq_rims.txt`, jq reads the string numbers of all 210 SSF JSON files, which must build again into the source sound set, and parses all 62 UTI JSON files. In `temp/reports/ncs_rims.txt`, all 492 script listings assemble to their source scripts.

# Tables, sounds, scripts and templates in the BIF archives

Five BIF archives hold rule tables, sound effects, scripts, object templates and the quest journal. Most of their formats are already described under modules and global resources; this section gives what is specific to these archives and describes two formats that occur only here, the NWScript source and the LTR letter table. The `bifs` stage of `cmd/extract` converts every resource of these five archives, and also the room layouts of `layouts.bif`, which are described in the section "Room layouts: LYT". The table below lists the five archives and their contents.

| Archive         | Resources | Contents                                                      | Output                      |
| --------------- | --------- | ------------------------------------------------------------- | --------------------------- |
| `2da.bif`       | 209       | 209 2DA tables                                                | `extracted/data/2da/`       |
| `sounds.bif`    | 1,928     | 1,928 WAV sounds                                              | `extracted/data/sounds/`    |
| `scripts.bif`   | 3,558     | 1,784 compiled scripts (NCS) and 1,774 script sources (NSS)   | `extracted/data/scripts/`   |
| `templates.bif` | 1,387     | 1,277 GFF files of 12 types, 106 SSF sound sets, 3 LTR, 1 TXI | `extracted/data/templates/` |
| `_newbif.bif`   | 1         | 1 journal (JRL), `global`                                     | `extracted/data/_newbif/`   |

## 2DA tables

All 209 tables use the binary form "2DA V2.b" described under global resources, and all parse with `ParseTwoDa`. All 209 use a tab as the separator; the NUL separator occurs only in the RIM copies. The tables have between 1 and 80 columns (median 4) and between 0 and 1,185 rows (median 11). Four tables have no rows: `iprp_base1`, `iprp_slotscost`, `iprp_staminacost` and `iprp_terraintype`.

153 tables also occur in `rims/global.rim`, and their CSV files are identical to the CSV files of the RIM copies. The other 56 occur only here. Examples are `actions`, `dialogtokens`, `keymap`, `soundeax` and `xpbaseconst`.

## WAV sounds

All 1,928 sounds are ordinary RIFF WAVE files. In every file the RIFF size equals the file size minus 8, the fmt chunk comes first, and the chunks end exactly at the end of the file. The table below gives the encodings, as reported by ffprobe.

| Encoding                     | Sample rate | Channels | Files |
| ---------------------------- | ----------- | -------- | ----- |
| 16-bit PCM (format 1)        | 22,050 Hz   | 1        | 1,839 |
| 16-bit PCM (format 1)        | 22,050 Hz   | 2        | 1     |
| 16-bit PCM (format 1)        | 44,100 Hz   | 1        | 72    |
| 16-bit PCM (format 1)        | 44,100 Hz   | 2        | 8     |
| 16-bit PCM (format 1)        | 11,025 Hz   | 1        | 3     |
| 8-bit PCM (format 1)         | 11,025 Hz   | 1        | 1     |
| IMA ADPCM (format 17), 4-bit | 44,100 Hz   | 1        | 4     |

The nine stereo files are named `mgs_`, such as `mgs_bigship`. The 8-bit file is `mgs_sith_hit`. Its samples are unsigned, with silence at 128, as is usual for 8-bit WAV. The four IMA ADPCM files are the same four as in `global.rim`.

The 246 sounds of `global.rim` are copies of sounds in this archive. 797 sounds have the name of a file in `streamsounds/`, compared without regard to case. For 785 of them, the converted file is identical to the converted stream sound. The other 12 differ, for example `c_drdmk1_atk1`, which has the same size, and `mgs_engine_01l`, which is 12 bytes shorter. Which copy the game plays is UNKNOWN. The most common name prefixes are `as_` (593 files), `p_` (419) and `n_` (246).

## NWScript sources: NSS

A resource of type 2009 is the source text of a script in NWScript, the scripting language that the compiled NCS scripts are built from. The type is named "nss" after the file `nwscript.nss`, which is one of these resources (see "Engine routines: nwscript.nss"). The text has no header or signature.

All 1,774 sources are text with CRLF line breaks, and none contains a NUL byte. 1,772 end with a line break. Seven files contain bytes of 0x80 or above. Five contain 0xA9 in the line "Copyright © 2002 Bioware Corp.", and two, `k_sup_bands` and `nwscript`, contain 0x92 where an apostrophe belongs, as in "don’t". In Windows-1252, 0xA9 is the copyright sign and 0x92 is the right single quotation mark; in Latin-1, 0x92 is a control character. The text is therefore probably Windows-1252. The confidence is medium, because only these two byte values occur.

1,754 names have both a source and a compiled script. Twenty sources have no compiled script. Seventeen of them are include files named `k_inc_*`, such as `k_inc_debug`, which other sources include with an `#include` directive; 964 sources contain such a directive, 615 of them for `k_inc_debug`; the other three are `nwscript`, `e3_scripts` and `fightback`. Five other include files, such as `k_inc_generic`, do have a compiled script of the same name; why is UNKNOWN. Thirty compiled scripts have no source. Ten of them are named `qa_*`, and others are named `test` and `asd`, so they are probably test scripts. Whether every compiled script matches its source has not been checked; one known mismatch, `nw_s0_lghtnbolt`, is described under "Engine routines: nwscript.nss".

## Templates

`templates.bif` holds the templates that the game creates objects from, together with the palettes and the name tables that support them. All 1,277 GFF files carry the signature of their type with version "V3.2" and parse with `ReadGff`. Five of them have unreferenced bytes in the field index or list index region, as described under "Generic file format: GFF": the BIC file (44 bytes) and four ITP files (8 to 252 bytes). The table below lists the types. The types that do not occur in the modules are named after their signatures; their meanings come from their field labels and are not confirmed.

| Type | Ext | Files | Probable content                                                                                                | Confidence |
| ---- | --- | ----- | --------------------------------------------------------------------------------------------------------------- | ---------- |
| 2015 | bic | 1     | A character, `temp_char`. It has the creature fields of a UTC file, such as Race, ClassList and Equip_ItemList. | low        |
| 2024 | bti | 1     | An item, `nw_wblhvy001`. It has the item fields of a UTI file, such as BaseItem, PropertiesList and Cost.       | low        |
| 2026 | btc | 3     | Creatures, such as `sw_krayt`. They have creature fields, such as FactionID, ScriptSpawn and SaveFortitude.     | low        |
| 2030 | itp | 16    | Palettes, described below.                                                                                      | medium     |
| 2029 | dlg | 32    | Dialogs, as in the modules.                                                                                     | high       |
| 2027 | utc | 205   | Creature templates, as in the modules.                                                                          | high       |
| 2042 | utd | 50    | Door templates, as in the modules.                                                                              | high       |
| 2040 | ute | 65    | Encounter templates, as in the modules.                                                                         | high       |
| 2025 | uti | 557   | Item templates, as in the modules.                                                                              | high       |
| 2044 | utp | 317   | Placeable templates, as in the modules.                                                                         | high       |
| 2032 | utt | 21    | Trigger templates, as in the modules.                                                                           | high       |
| 2058 | utw | 9     | Waypoint templates, as in the modules.                                                                          | high       |

The 16 ITP files form two groups. Nine are named `<kind>pal`, such as `creaturepal`, and hold a field RESTYPE with a resource type number, such as 2027 (UTC) in `creaturepal`, and a list MAIN of categories. Each category has the field DELETE_ME and some of the fields ID, TYPE and LIST; a LIST holds nested categories. The other seven are named `<kind>palstd`, such as `creaturepalstd`. Their categories hold lists of entries with the fields NAME, RESREF and, for creatures, CR. The seven files name 1,093 distinct resrefs, and all but one name a template in `templates.bif`. For example, `creaturepalstd` lists "Bantha" with the resref `c_bantha`. The palettes are probably the template lists of BioWare's level editor, because the game itself does not need them to create objects. This has not been confirmed.

The archive also holds the TXI file `grass`, which contains the two lines "downsamplemax 0" and "downsamplemin 0", and the three LTR files described below.

## LTR letter tables

An LTR file (type 2036) holds the letter probabilities for a random name generator. The three files are `humanf`, `humanm` and `humanl`, probably human female first names, male first names and last names, which the character creation screen can generate. This use is not confirmed. The parser is `ParseLtr` in `kotor/ltr.go`.

The file has a 9-byte header followed by tables of 28 f32 values, one value per letter. The letters are "a" to "z", the apostrophe and the hyphen, in this order. The tables come in sets of three: one for the start of a name, one for the middle and one for the end. The table below describes the layout.

| Offset | Type            | Meaning                                                                                         | Evidence                                                                                                    | Confidence |
| ------ | --------------- | ----------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8]         | Signature "LTR V1.0"                                                                            | Present in all three files.                                                                                 | high       |
| 0x08   | u8              | Number of letters n                                                                             | The value is 28 in all three files, and the size of each file is 9 + 4 × 3n × (1 + n + n²) = 273,177 bytes. | high       |
| 0x09   | f32[3][n]       | The set for single letters: start, middle and end                                               | See the evidence below.                                                                                     | high       |
| 0x159  | f32[n][3][n]    | One set per preceding letter, in letter order, giving the next letter                           | See the evidence below.                                                                                     | high       |
| 0x2619 | f32[n][n][3][n] | One set per preceding pair of letters, the first letter varying slowest, giving the next letter | The tables end exactly at the end of all three files.                                                       | high       |

Each nonzero value is the cumulative probability of the letters up to and including that letter, and 0 means that the letter cannot occur there. Of the 2,439 tables per file, 217 (`humanf`), 225 (`humanm`) and 192 (`humanl`) contain a nonzero value. In each of these tables except two per file, the nonzero values never decrease, and the last one is 1 within 0.0001. The largest value is 1.0000001, which is the rounding of a sum. The two exceptions in each file are the middle and the end tables of the single-letter set. In them, the values restart at a small value several times; for example, the middle table of `humanf` reaches 0.472 at "i" and continues with 0.009 at "k". The meaning of these two tables is UNKNOWN.

The order of the sets and of the letters follows from three checks, which hold in all three files. The start table after the letter "q" in `humanf` and `humanm` allows only "u", which is the 21st letter. The start table after a letter contains a nonzero value exactly when that letter has a nonzero value in the single-letter start table, for all 28 letters. The tables after a pair of letters contain nonzero values only when the tables after the first letter allow the second letter, for all 114, 127 and 103 such pairs.

## Conversion and verification

The `bifs` stage of `cmd/extract` writes one folder `extracted/data/<archive>/` per archive and the report `temp/reports/bifs.txt`. All 7,207 lines pass: 7,083 for the five archives and 124 for `layouts.bif`. The resources become the files below. The 2DA, SSF, NCS and GFF resources use the same forms as in `rims/` and the modules.

| Resource | Output                | Form                                                                                                                                                    |
| -------- | --------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------- |
| WAV      | `<resref>.wav`        | 16-bit PCM. 16-bit PCM files are copied unchanged. The IMA ADPCM files are decoded, and the 8-bit file is widened: each sample becomes (s − 128) × 256. |
| 2DA      | `<resref>.2da.csv`    | CSV, as for `rims/`.                                                                                                                                    |
| NCS      | `<resref>.ncs.txt`    | The disassembly listing of the module files.                                                                                                            |
| NSS      | `<resref>.nss`        | The source text, decoded as Windows-1252 and written as UTF-8. The line breaks are kept.                                                                |
| SSF      | `<resref>.ssf.json`   | JSON, as for `rims/`.                                                                                                                                   |
| LTR      | `<resref>.ltr.json`   | JSON, described below.                                                                                                                                  |
| TXI      | `<resref>.txi`        | Copied unchanged.                                                                                                                                       |
| GFF      | `<resref>.<ext>.json` | The JSON form described under "Generic file format: GFF", for all 13 GFF file types including the journal.                                              |

The JSON form of an LTR file is an object with "letters", the 28 letters in table order; "single", the single-letter set; "double", an object keyed by the preceding letter; and "triple", an object keyed by the preceding pair. Each set is an object with "start", "middle" and "end", each an array of the 28 stored values in file order. The values are the stored cumulative probabilities, written with the shortest decimal that reads back as the same f32. The example below shows the start of `extracted/data/templates/humanf.ltr.json`.

```json
{
	"letters": "abcdefghijklmnopqrstuvwxyz'-",
	"single": {
		"start": [
			0.06557377,
			0.114754096,
```

The `bifs` stage of `cmd/verify` writes six reports, and every line passes. The table below lists them.

| Report             | Files | Check                                                                                                                                                                                            |
| ------------------ | ----- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `ffprobe_bifs.txt` | 1,928 | ffprobe reads the WAV file as 16-bit PCM, and ffmpeg decodes the output and the source to the same 16-bit samples (see the note on ADPCM below).                                                 |
| `csv_bifs.txt`     | 209   | Go's `encoding/csv` reader reads the CSV file, and the table built again from it equals the source table byte for byte.                                                                          |
| `ncs_bifs.txt`     | 1,784 | The listing assembles to the source script.                                                                                                                                                      |
| `jq_bifs.txt`      | 1,511 | jq parses the 1,278 GFF JSON files. For the 106 SSF, 3 LTR and 124 LYT files, the values that jq reads build again into the source file (see "Room layouts: LYT" for the comparison of layouts). |
| `iconv_bifs.txt`   | 1,774 | iconv converts the NSS file from UTF-8 back to Windows-1252, and the result equals the source bytes.                                                                                             |
| `copy_bifs.txt`    | 1     | The TXI file equals the source bytes.                                                                                                                                                            |

A test altered one value in `humanf.ltr.json`, one character in `k_pman_casamad_s.nss` and one sample byte in `mgs_sith_hit.wav`. Each of the three files then failed its check, and no other file did.

The four IMA ADPCM files cannot be compared exactly with ffmpeg. `DecodeAdpcm` computes each step as a sum of shifted step sizes, as the IMA reference decoder does, while ffmpeg multiplies exactly. The two decodings agree exactly at the first sample of every block, which the block header stores, and drift apart by up to 29, 53, 86 and 65 within a block in the four files. The check therefore allows a difference of at most 128 for ADPCM sources. It also requires the output to hold exactly the number of samples in the fact chunk, which is fewer than ffmpeg decodes from the whole last block. Which rounding the game uses is UNKNOWN. The same check applies to the WAV files of `rims/`.

# String table: dialog.tlk

The file `dialog.tlk` is the string table of the game. It holds 49,265 entries, numbered from 0. A localized string in a GFF file and a slot of an SSF sound set name an entry by this number, the string reference (strref). An entry holds a text and the name of a sound, which is usually the voice file that speaks the text. The reader is `ReadTlk` in `kotor/tlk.go`. Only one TLK file exists, so the format could not be compared across files. The 49,265 entries were compared instead.

The file begins with a 20-byte header, described in the table below.

| Offset | Type    | Meaning                   | Evidence                                                                           | Confidence |
| ------ | ------- | ------------------------- | ---------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "TLK V3.0"      | Present in the only file.                                                          | high       |
| 0x08   | u32     | UNKNOWN, 0                | The value is 0. It may be a language number, but no other language file exists.    | low        |
| 0x0C   | u32     | Number of entries         | The entry table, 40 bytes per entry, ends exactly at the offset in the next field. | high       |
| 0x10   | u32     | Offset of the string data | The value is 0x1E11BC, which equals 20 + 40 × 49,265.                              | high       |

Each entry is 40 bytes long and is laid out as described in the table below.

| Offset | Type     | Meaning                                  | Evidence                                                                                                                                    | Confidence |
| ------ | -------- | ---------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u32      | Flags                                    | The values are 7 (48,560 entries), 6 (398) and 0x8000 (307). Bit 0 is set exactly in the 48,560 entries that have text.                     | medium     |
| 0x04   | char[16] | Sound resref, padded with NUL bytes      | 32,853 entries name a sound. Every byte after the name is NUL. The names use only digits, letters and the underscore.                       | high       |
| 0x14   | u32      | UNKNOWN, always 0                        | The value is 0 in all entries. It may be a volume variance.                                                                                 | low        |
| 0x18   | u32      | UNKNOWN, always 0                        | The value is 0 in all entries. It may be a pitch variance.                                                                                  | low        |
| 0x1C   | u32      | Offset of the text, from the string data | The texts lie in entry order with no gap or overlap, and the last ends exactly at the end of the file. All 705 entries without text hold 0. | high       |
| 0x20   | u32      | Length of the text in bytes              | See above. The length is 0 exactly when bit 0 of the flags is clear.                                                                        | high       |
| 0x24   | f32      | UNKNOWN, always 0.0                      | The bits are 0 in all entries. It may be the length of the sound.                                                                           | low        |

The text is not terminated by a NUL byte. All bytes of the texts are below 0x80, and the only control character is the line feed (5,191 times), so the code page does not matter. Entry 0 is "Bad StrRef", and entry 35,782 is "Taris - South Apartments", the area name of `tar_m02aa`.

## Flags

The flag values combine with text and sound as shown in the table below. A referenced entry is one whose number occurs in a localized string, an SSF slot or a GUI STRREF field of any converted resource: the modules, `rims/`, the BIF archives, `patch.erf` and the saved game. These name 40,913 distinct entries. Of them, 34,651 are named by the modules and `rims/` alone. String numbers in the cells of 2DA tables are not counted, because a CSV cell does not mark which columns hold string numbers.

| Flags  | Text | Sound | Entries | Referenced |
| ------ | ---- | ----- | ------- | ---------- |
| 7      | yes  | yes   | 32,494  | 29,607     |
| 7      | yes  | no    | 16,066  | 11,274     |
| 6      | no   | yes   | 359     | 30         |
| 6      | no   | no    | 39      | 1          |
| 0x8000 | no   | no    | 307     | 1          |

Bit 0 means that the entry has text. Bits 1 and 2 are set together in every entry except the 307 with flag 0x8000, even when the entry has no sound, so their meaning is UNKNOWN. The one referenced entry with flags 6 and neither text nor sound, 31,855, is the field DescIdentified of the item template `g_w_flashgren001.uti` in `templates.bif`. The item therefore has an empty description. The meaning of flag 0x8000 is UNKNOWN. These entries are scattered from entry 369 to entry 49,248, and they include the 100 consecutive entries 49,145 to 49,244. The one referenced entry, 42,058, is a slot of the sound set `p_trask.ssf` in `rims/global.rim` and `rims/miniglobal.rim`. The flag may mark removed entries, but this has not been confirmed.

## Sound names

The sound resref names a WAV file without its extension, compared without regard to case. The 32,853 entries with a sound resolve as described in the table below.

| Where the sound is found                                                       | Entries |
| ------------------------------------------------------------------------------ | ------- |
| A file in `streamwaves/`, `streamsounds/` or `streammusic/`                    | 20,320  |
| Only a WAV resource (type 4) in `data/sounds.bif`, for example `cs_footsteps`  | 5       |
| Nowhere: not in the stream folders, the KEY file, the RIM files or `patch.erf` | 12,528  |

Another 647 entries name a sound that exists both in `data/sounds.bif` and in `streamsounds/`, such as `p_bastila_bat1`. They are counted in the first row.

Most names without a file are player replies. Voiced lines start with `n`, for example `nglobeband01000_`. A player reply starts with `_` in the same position, for example `_m35aabast06535_`, and has no sound file because the player character is not voiced. The evidence comes from the 1,135 dialog (DLG) files in `extracted/modules`. Their NPC lines (EntryList) name 17,343 distinct entries whose sound starts with `n` and 1 whose sound starts with `_`. Their player replies (ReplyList) name 7,439 distinct entries whose sound starts with `_` and none whose sound starts with `n`. Of the 10,065 entries whose sound starts with `_`, only one has a file. The remaining 2,464 names without a file mostly start with `n` (2,351), and include `move2` and `droid2`. Why these files are missing is UNKNOWN.

## Conversion and verification

The `tlk` stage of `cmd/extract` writes `extracted/dialog.tlk.json`. It is an object with "language", the u32 at header offset 0x08, and "entries", one object per entry in file order. Each entry has "strref", its number; "flags"; "sound"; and "text". The sound and the text are empty strings when the entry has none. The form leaves out the three unknown fields and the text offsets, so `ReadTlk` rejects a file in which an unknown field is not 0 or an entry without text has an offset other than 0. The example below shows the start of the file.

```json
{
	"language": 0,
	"entries": [
		{
			"strref": 0,
			"flags": 7,
			"sound": "",
			"text": "Bad StrRef"
		},
```

The parser passes on the only sample: `temp/reports/tlk.txt` reports 49,265 entries, 48,560 with text and 32,853 with a sound. The `tlk` stage of `cmd/verify` reads the JSON file with jq and builds the TLK file again from jq's output: the header, the entries with offsets in text order, and the texts encoded as Latin-1. The result equals `dialog.tlk` byte for byte, as `temp/reports/jq_tlk.txt` reports. A test in which one character of the text of entry 0 was altered made the check fail.

# 3D models: MDL and MDX

Each 3D model is stored as two resources with the same resref. The MDL resource (type 2002) holds the model's structure: a tree of nodes, some of which carry meshes. The MDX resource (type 3008) holds the vertex data for those meshes. The converter is `ConvertMdl` in `kotor/mdl.go`, which writes Wavefront OBJ files. The `models` stage of `cmd/extract` runs it on every model of `data/models.bif`, and `cmd/mdl2obj` runs it on one model.

The models examined here are in `data/models.bif`, which contains 2,445 MDL resources and 2,445 MDX resources, including area room models such as `m01aa_01a`. Of the converted models, 1,329 have names of that form. The BIF archives hold 387 more models: 305 in `items.bif`, 63 in `player.bif` and 19 in `party.bif`. All 387 pass the same checks of the converter, with no MDL and MDX position mismatch, but they are not converted. The archives in `rims/` hold another 239 MDL and 252 MDX resources. All of them are copies of models in the BIF archives, and their conversion is described in the section "Global resources: rims/\*.rim". `patch.erf` holds one model, `w_lghtsbr_008`, a changed copy of a model in `items.bif` (see "Patch archive: patch.erf"). The MOD archives in `lips/` hold only LIP resources, and the texture packs hold no models.

All 2,445 models pass the converter's checks. Every read stays inside the file, every face index refers to an existing vertex, and the vertex positions stored in the MDL match those stored in the MDX for all 11,248,282 vertices. Of these models, 368 contain no meshes.

The converter omits meshes whose render flag is 0 (see the mesh header). This removes 14,055 meshes. After this, 473 models have no meshes left and are not written, and 1,972 models are written. Each OBJ file refers to an MTL file with its textures (see "Conversion to model materials" in the texture section).

The OBJ files are verified with assimp, an independent model loader. `cmd/verify` imports each of the 1,972 OBJ files and requires at least one mesh and the same face count as the file. All 1,972 pass.

## MDL file header

The MDL file begins with a 12-byte header.

| Offset | Type | Meaning                       | Evidence                  | Confidence |
| ------ | ---- | ----------------------------- | ------------------------- | ---------- |
| 0x00   | u32  | Always 0                      | True in all 2,445 models. | high       |
| 0x04   | u32  | Size of the MDL file minus 12 | True in all 2,445 models. | high       |
| 0x08   | u32  | Size of the matching MDX file | True in all 2,445 models. | high       |

The rest of the file is the model data. All offsets stored inside the model data are relative to its start, which is byte 12 of the file. The offsets in the remaining MDL tables are also relative to that point.

## Model header

The model header is at the start of the model data.

| Offset | Type     | Meaning                                                        | Evidence                                                                                                                                                                                         | Confidence |
| ------ | -------- | -------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------- |
| 0x00   | u32      | Constant 0x00413670                                            | Identical in all 2,445 models. It is likely an address in the game executable. Its purpose is not known.                                                                                         | medium     |
| 0x04   | u32      | Constant 0x00405520                                            | Identical in all 2,445 models. It is likely an address in the game executable. Its purpose is not known.                                                                                         | medium     |
| 0x08   | char[32] | Model name                                                     | Equals the resref in 2,444 of 2,445 models.                                                                                                                                                      | high       |
| 0x28   | u32      | Offset of the root node                                        | The node tree parses from this offset in all 2,445 models.                                                                                                                                       | high       |
| 0x2C   | u32      | Number of nodes                                                | Equals the number of nodes in the tree in 2,187 of 2,445 models. See below.                                                                                                                      | medium     |
| 0x68   | f32[6]   | Probably a bounding box: minimum x, y, z, then maximum x, y, z | The minimum is at most the maximum on every axis in all 2,445 models. The converted geometry lies inside the box in only 709 of the 1,972 converted models, so what the box encloses is UNKNOWN. | low        |
| 0x88   | char[32] | Name of the supermodel, or "NULL"                              | Contains model names such as S_Female02.                                                                                                                                                         | high       |
| 0xA8   | u32      | Probably the offset of the root node (repeated)                | Equals the value at 0x28 in 2,374 of 2,445 models. The other 71 hold other values between 2,138 and 45,068.                                                                                      | low        |
| 0xB0   | u32      | Size of the MDX file (repeated)                                | Equals the size of the MDX resource in all 2,445 models.                                                                                                                                         | high       |
| 0xB8   | u32      | Offset of the name table                                       | All node names resolve in all 2,445 models.                                                                                                                                                      | high       |
| 0xC0   | u32      | Number of entries in the name table                            | All node names resolve in all 2,445 models.                                                                                                                                                      | high       |

A supermodel is another model whose nodes this model appears to inherit. In 258 models the node count does not match the number of nodes in the tree. All 258 have a supermodel, and in all of them the count exceeds the number of nodes in the tree, so the count probably includes nodes of the supermodel. It is not the sum of the model's nodes and the supermodel's count in any of the 258 models, so how the count is formed is UNKNOWN.

The name table is an array of u32 offsets. Each offset points to a NUL-terminated node name.

## Node

A node is an 80-byte structure. Nodes form a tree that starts at the root node.

| Offset | Type   | Meaning                                                              | Evidence                                                                                                       | Confidence |
| ------ | ------ | -------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u16    | Flags                                                                | See the flag table below.                                                                                      | medium     |
| 0x02   | u16    | Node number                                                          | Exceeds the name count in 256 models, which suggests it includes supermodel nodes. This has not been verified. | low        |
| 0x04   | u16    | Index into the name table                                            | Within range for all 107,605 nodes.                                                                            | high       |
| 0x10   | f32[3] | Position relative to the parent                                      | Converted models render with correct part placement.                                                           | medium     |
| 0x1C   | f32[4] | Rotation relative to the parent, as a quaternion in w, x, y, z order | All 107,605 rotations have unit length, and all 2,445 root nodes hold (1, 0, 0, 0).                            | medium     |
| 0x2C   | u32    | Offset of the child offset array                                     | The tree stays inside the file in all 2,445 models.                                                            | high       |
| 0x30   | u32    | Number of children                                                   | The tree stays inside the file in all 2,445 models.                                                            | high       |

The child offset array is an array of u32 offsets. Each offset points to a child node.

The known flag bits are listed below.

| Bit  | Meaning                                                         | Evidence                                                                                                         | Confidence |
| ---- | --------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x01 | Set on every node                                               | Set on all 107,605 nodes. Its meaning is not known.                                                              | medium     |
| 0x20 | The node has a mesh header directly after the node              | Meshes parse correctly in all 2,445 models.                                                                      | high       |
| 0x40 | Probably a skinned mesh, that is, one deformed by several bones | Set on 649 nodes in 220 models, always together with 0x20. Most of these models are creatures, such as c_bantha. | low        |

The flags take eight combinations in models.bif. The table below counts the nodes with each combination.

| Flags | Nodes  | Flags | Nodes  |
| ----- | ------ | ----- | ------ |
| 0x001 | 19,685 | 0x021 | 67,231 |
| 0x003 | 5,550  | 0x061 | 649    |
| 0x005 | 8,848  | 0x121 | 2,586  |
| 0x011 | 1,954  | 0x221 | 1,102  |

Bits 0x02, 0x04 and 0x10 never occur together with 0x20, so these nodes carry no mesh. Bits 0x100 and 0x200 occur only together with 0x20. Bit 0x200 probably marks the walkmesh node of a room model: each of the 980 room models whose area walkmesh has faces has exactly one node with this bit, and that node's position is stored in the walkmesh header (see "Walkmeshes: BWM"). The names of these nodes vary: 698 contain "walk", such as `walkmesh_12` and `01awalkmesh`, and the others have generic names such as `Object125`. The other 122 nodes with bit 0x200 are one each in 122 models that have no area walkmesh, such as the swoop race obstacles `m03mg_mgo01` to `m03mg_mgo22` and the rooms `m12ad_01a` to `m12ah_01a`. The meanings of bits 0x02, 0x04, 0x10 and 0x100 are UNKNOWN. The converter treats all nodes with 0x20 alike, and it does not read any additional data that the other bits may announce.

## Mesh header

A mesh header is a 332-byte structure. It follows a node that has the 0x20 flag set.

| Offset | Type     | Meaning                                                                            | Evidence                                                                                                | Confidence |
| ------ | -------- | ---------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------- | ---------- |
| 0x08   | u32      | Offset of the face array                                                           | Face normals match the vertices (see the face table).                                                   | high       |
| 0x0C   | u32      | Number of faces                                                                    | Face normals match the vertices (see the face table).                                                   | high       |
| 0x58   | char[32] | Name of the texture                                                                | Contains texture names such as PLC_SithFlag.                                                            | high       |
| 0xFC   | u32      | Size of one vertex record in the MDX file                                          | MDX positions match MDL positions for every vertex.                                                     | high       |
| 0x100  | u32      | Bit field describing the vertex record                                             | The value 0x23 appears with position, normal and texture coordinates present. Its bits are not decoded. | low        |
| 0x104  | u32      | Offset of the position within a vertex record                                      | MDX positions match MDL positions for every vertex.                                                     | high       |
| 0x108  | u32      | Offset of the normal within a vertex record, or 0xFFFFFFFF if absent               | Face orientation agrees with these normals in the c_bantha meshes.                                      | medium     |
| 0x110  | u32      | Offset of the texture coordinates within a vertex record, or 0xFFFFFFFF if absent  | Textured models display plausible coordinates.                                                          | medium     |
| 0x130  | u16      | Number of vertices                                                                 | Every face index is below this value.                                                                   | high       |
| 0x139  | u8       | Render flag: 1 if the game draws the mesh, 0 if not                                | See below.                                                                                              | medium     |
| 0x144  | u32      | Offset of this mesh's vertex records in the MDX file                               | MDX positions match MDL positions for every vertex.                                                     | high       |
| 0x148  | u32      | Offset of the vertex positions in the MDL file. Each position is three f32 values. | MDX positions match MDL positions for every vertex.                                                     | high       |

The render flag at 0x139 is 0 or 1 in all 71,568 meshes. Of the 57,513 meshes with the value 1, 56,359 have a texture. Of the 14,055 meshes with the value 0, 13,381 have no texture. Most meshes with the value 0 are named with the suffix "\_g" or are walkmeshes and boxes; these look like helper geometry for animation and collision. Rendering one head model (p_bastilah) with and without these meshes shows untextured plates over the face when they are included. The flag has not been tested in the game itself.

The remaining bytes of the mesh header are UNKNOWN.

## Face

A face is a 32-byte structure that describes one triangle.

| Offset | Type   | Meaning                            | Evidence                                                                | Confidence |
| ------ | ------ | ---------------------------------- | ----------------------------------------------------------------------- | ---------- |
| 0x00   | f32[3] | Face normal                        | Agrees with the vertex order for 5,409,908 of 5,412,520 faces (99.95%). | high       |
| 0x0C   | f32    | UNKNOWN, likely the plane distance | Not tested.                                                             | low        |
| 0x10   | u32    | UNKNOWN, likely a surface material | Not tested.                                                             | low        |
| 0x14   | u16[3] | UNKNOWN, likely adjacent faces     | Not tested.                                                             | low        |
| 0x1A   | u16[3] | Vertex indices                     | Every index is below the vertex count.                                  | high       |

The cause of the 2,612 faces whose normal disagrees with the vertex order is not known.

## MDX file

An MDX file has no header. It is an array of vertex records, each built from f32 values. The mesh header in the MDL file gives the record size and the position of each component within a record. The MDX resources of 368 models are empty, for example those of v_polymorf_imp and m12ah_mgt04. These are exactly the 368 models that contain no meshes.

## Walkmeshes in models.bif

The archive also contains 1,554 walkmeshes, which are described in the section "Walkmeshes: BWM". An empty MDX resource has the offset of the resource that follows it. For 32 of the 368 empty MDX resources, such as that of `m03mg_01b`, the next resource is a walkmesh, so a reader that looks at the bytes at that offset finds "BWM V1.0".

# Walkmeshes: BWM

A walkmesh is a triangle mesh that defines where characters can walk and where they collide. Each face carries a surface material, such as dirt or a non-walkable wall. The format, binary walkmesh (BWM), has three resource types, all in `data/models.bif`. No other archive holds walkmeshes. The converter is part of the models stage of `cmd/extract`. The parser is `kotor/bwm.go`.

The three types and their counts are listed below. The extensions are taken from the extension pool of `swkotor.exe`. There the strings "utw sav jrl utg btg pwk dwk utm" run in descending type order from 2058 to 2051, so "pwk" is 2053 and "dwk" is 2052. The string "wok" stands between "2da" (2017) and "bic" (2015).

| Type | Extension | Count | Content                                                                |
| ---- | --------- | ----- | ---------------------------------------------------------------------- |
| 2016 | wok       | 1,202 | Area room walkmesh. Each has the resref of a room model in models.bif. |
| 2052 | dwk       | 156   | Door walkmesh. The resrefs form 52 sets ending in 0, 1 and 2.          |
| 2053 | pwk       | 196   | Placeable walkmesh. Every resref begins with "plc\_".                  |

The executable contains the strings "closed", "open1" and "open2". The three door walkmeshes of a set probably describe these three states of a door. This has not been verified.

All 1,554 walkmeshes pass the parser's checks: every table lies inside the file, the tables follow each other without gaps and end exactly at the end of the file, and every index refers to an existing entry. Together they hold 116,787 vertices and 191,329 faces. Of the walkmeshes, 222 area walkmeshes and 74 placeable walkmeshes have no faces and are not converted.

## Header

The header is 136 bytes (0x88). It holds the walkmesh type, five points and nine table descriptors.

| Offset | Type    | Meaning                                                      | Evidence                                                                         | Confidence |
| ------ | ------- | ------------------------------------------------------------ | -------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "BWM V1.0"                                         | Present in all 1,554 files.                                                      | high       |
| 0x08   | u32     | Walkmesh type: 1 for an area room, 0 for a door or placeable | 1 in all 1,202 area walkmeshes, 0 in all 352 door and placeable walkmeshes.      | high       |
| 0x0C   | f32[3]  | First use point, relative                                    | See below.                                                                       | low        |
| 0x18   | f32[3]  | Second use point, relative                                   | See below.                                                                       | low        |
| 0x24   | f32[3]  | First use point, absolute                                    | Equals the relative point plus the position in all 156 door walkmeshes.          | medium     |
| 0x30   | f32[3]  | Second use point, absolute                                   | Equals the relative point plus the position in all 156 door walkmeshes.          | medium     |
| 0x3C   | f32[3]  | Position                                                     | In area walkmeshes, equals the position of the model's walkmesh node. See below. | high       |
| 0x48   | u32     | Number of vertices                                           | Tables end exactly at the end of the file in all 1,554 files.                    | high       |
| 0x4C   | u32     | Offset of the vertex table                                   | As above.                                                                        | high       |
| 0x50   | u32     | Number of faces                                              | As above.                                                                        | high       |
| 0x54   | u32     | Offset of the face table                                     | As above.                                                                        | high       |
| 0x58   | u32     | Offset of the material table                                 | As above.                                                                        | high       |
| 0x5C   | u32     | Offset of the normal table                                   | As above.                                                                        | high       |
| 0x60   | u32     | Offset of the plane distance table                           | As above.                                                                        | high       |
| 0x64   | u32     | Number of AABB nodes                                         | As above.                                                                        | high       |
| 0x68   | u32     | Offset of the AABB table                                     | As above.                                                                        | high       |
| 0x6C   | u32     | Probably the index of the root AABB node                     | See below.                                                                       | low        |
| 0x70   | u32     | Number of adjacency rows                                     | As above.                                                                        | high       |
| 0x74   | u32     | Offset of the adjacency table                                | As above.                                                                        | high       |
| 0x78   | u32     | Number of perimeter edges                                    | As above.                                                                        | high       |
| 0x7C   | u32     | Offset of the edge table                                     | As above.                                                                        | high       |
| 0x80   | u32     | Number of perimeters                                         | As above.                                                                        | high       |
| 0x84   | u32     | Offset of the perimeter table                                | As above.                                                                        | high       |

All offsets count from the start of the file. The tables are stored in the order of the header, directly after it. An empty table may have the offset 0, or the offset where it would start. The material, normal and plane distance tables have one entry per face. Door and placeable walkmeshes hold only vertices, faces, materials, normals and plane distances: their AABB, adjacency, edge and perimeter tables are empty in all 352 files.

The fields from 0x0C to 0x3B are 0 in all 1,202 area walkmeshes. The executable contains the strings "pwk*use" and "pwk_dp_use*", so the two points are probably the places where a character stands to use a door or placeable. In placeable walkmeshes the absolute points are 0 in all 196 files, and the relative points are set in 188. The position is set in 146 of the 156 door walkmeshes and in 100 of the 122 placeable walkmeshes with faces. The meaning of the position in door and placeable walkmeshes is not known.

In area walkmeshes the position equals the position of one node in the room's MDL model. In all 980 area walkmeshes with faces, the model has exactly one node with flag bit 0x200, and its position equals the value at 0x3C. The node names vary, such as `01awalkmesh`, `walkmesh_12` and `Object125` (see "Node"). The 222 area walkmeshes without faces belong to models with no such node.

The value at 0x6C is 0 in all 980 area walkmeshes with faces and in all door and placeable walkmeshes. In those 980 files, node 0 is the only AABB node that is no node's child. In 29 area walkmeshes without faces the value is nonzero, for example 0x3F800000 or 0xFFFFFFFF. These values look like uninitialized memory.

## Vertices

A vertex is three f32 values: x, y and z. Area walkmeshes use world coordinates. Each vertex equals a vertex of the model's walkmesh node, plus that node's position, plus the room's position in the area's room layout (LYT). This holds within 1 mm for 845 of the 980 area walkmeshes with faces, within 1 cm for 954 and within 10 cm for 971. The cause of the larger differences in the other 9 is not known. The walkmesh has fewer vertices than the MDL node in 960 of the 980 files; the MDL node probably stores each vertex once per face corner.

## Faces, materials, normals and plane distances

A face is three u32 vertex indices. Every index is below the vertex count in all 1,554 files.

A material is a u32 row number of `surfacemat.2da`. That table gives each material a label and a "walk" column, which is 1 for walkable surfaces. The materials found are listed below with their face counts.

| Row | Label        | Walk | Area   | Door | Placeable |
| --- | ------------ | ---- | ------ | ---- | --------- |
| 1   | Dirt         | 1    | 57,142 | 56   | 3         |
| 2   | Obscuring    | 0    | 15,869 | 8    | 0         |
| 3   | Grass        | 1    | 12,143 | 0    | 0         |
| 4   | Stone        | 1    | 6,501  | 0    | 0         |
| 5   | Wood         | 1    | 882    | 0    | 0         |
| 6   | Water        | 1    | 47     | 0    | 0         |
| 7   | Nonwalk      | 0    | 74,009 | 912  | 762       |
| 8   | Transparent  | 0    | 103    | 0    | 0         |
| 9   | Carpet       | 1    | 20     | 0    | 0         |
| 10  | Metal        | 1    | 18,534 | 0    | 0         |
| 11  | Puddles      | 1    | 16     | 0    | 0         |
| 13  | Mud          | 1    | 327    | 0    | 0         |
| 17  | DeepWater    | 0    | 29     | 0    | 0         |
| 19  | NonWalkGrass | 0    | 3,966  | 0    | 0         |

In area walkmeshes the walkable faces come first, before all other faces. This holds in all 980 files with faces. Of the 14 door walkmeshes with walkable faces, 4 do not follow this order. In the 2 placeable walkmeshes with walkable faces, those faces come first.

A normal is three f32 values. In area and door walkmeshes it is the unit normal of the face, and it agrees with the vertex order: the cosine between the normal and the cross product (v1 − v0) × (v2 − v0) exceeds 0.99 for all 190,452 faces that are not degenerate. The 112 degenerate faces, whose vertices lie on one line, all occur in area walkmeshes. In placeable walkmeshes all 765 normals are (0, 0, 0).

A plane distance is an f32 value d. In area and door walkmeshes, n · v0 + d = 0 within 0.001 for every face, where n is its normal and v0 its first vertex. Only the first vertex was tested. In placeable walkmeshes the values are arbitrary numbers such as 1.7e−33 and 2.1e17, which look like uninitialized memory.

## AABB tree

An AABB (axis-aligned bounding box) tree is a binary tree of boxes that lets the game find the face under a point quickly. Only area walkmeshes have one. Each node is 44 bytes.

| Offset | Type   | Meaning                                       | Evidence                                                        | Confidence |
| ------ | ------ | --------------------------------------------- | --------------------------------------------------------------- | ---------- |
| 0x00   | f32[6] | Box: minimum x, y, z, then maximum x, y, z    | Inner boxes contain their children's boxes in 979 of 980 trees. | high       |
| 0x18   | i32    | Face index in a leaf, −1 in an inner node     | Each face has one leaf (with one exception, see below).         | high       |
| 0x1C   | u32    | UNKNOWN; 4 in every node                      | True for all 378,136 nodes.                                     | low        |
| 0x20   | u32    | Split direction in an inner node, 0 in a leaf | See below.                                                      | medium     |
| 0x24   | i32    | Index of the left child, −1 in a leaf         | Every node is reached exactly once from node 0.                 | high       |
| 0x28   | i32    | Index of the right child, −1 in a leaf        | As above.                                                       | high       |

A tree over n faces has 2n − 1 nodes in 979 of the 980 area walkmeshes with faces. The exception, `m02af_01a`, has 219 nodes for 140 faces. Its 30 faces from 110 to 139, all of material 7 (Nonwalk), have no leaf.

A leaf box contains its face with a margin, usually 0.005 to 0.015. The largest difference between a leaf box and its face's box is at most 2 cm in 951 files and at most 10 cm in 977. The three exceptions are `m02af_01a`, `m17mg_01a` and `m26mg_01a`, with differences of 0.75, 1,111 and 1,151. The cause is not known.

The split value is one of 1, 2, 4, 8, 16 and 32. These name the axes +x, +y, +z, −x, −y and −z. Along the named axis, the center of the right child's box lies in the named direction from the center of the left child's box in 183,627 of 188,578 inner nodes. In 4,950 other nodes the two centers are equal along that axis, and one node contradicts the rule. How the game uses the value is not known.

## Adjacency table

The adjacency table connects each walkable face to its walkable neighbours. Only area walkmeshes have one. It has one row of three i32 values for each walkable face. Row i belongs to face i, which is possible because the walkable faces come first. Entry k describes the edge from vertex k to vertex (k + 1) mod 3 of that face. Edges are numbered 3 × face + k. An entry holds the number of the edge of the neighbouring face that shares this edge, or −1 if no walkable face shares it.

The number of rows equals the number of walkable faces, as given by the "walk" column of `surfacemat.2da`, in all 980 area walkmeshes with faces.

The table is symmetric: if edge a names edge b, then edge b names edge a. The only exception is `m38aa_03`, where three faces share the edge between vertices 6 and 7, so two entries cannot point back.

## Edge table

The edge table lists every edge whose adjacency entry is −1, that is, the boundary of the walkable surface. Each entry is 8 bytes.

| Offset | Type | Meaning                                                 | Evidence                                                                      | Confidence |
| ------ | ---- | ------------------------------------------------------- | ----------------------------------------------------------------------------- | ---------- |
| 0x00   | u32  | Edge number, 3 × face + k                               | Each boundary edge occurs exactly once in all 980 area walkmeshes with faces. | high       |
| 0x04   | i32  | Transition: index of a room in the area's layout, or −1 | See below.                                                                    | high       |

Transitions occur on 3,926 of the 39,366 edges. A transition marks an edge that leads into another room. The value is an index into the room list of the area's LYT file, counted from 0. For all 3,926 transitions, the midpoint of the edge lies inside the horizontal extent of the target room's walkmesh, within 5 cm.

## Perimeter table

The perimeter table is a list of u32 values. Each value is the end of one perimeter in the edge table, exclusive; the next perimeter starts there. The last value equals the number of edges in all 969 files that have edges. The other 11 area walkmeshes with faces, such as `m17aa_18`, have no walkable faces, and therefore no adjacency rows, edges or perimeters. Each perimeter is a closed loop: each edge ends at the vertex where the next edge starts, and the last edge ends where the first starts. This holds for all 1,789 perimeters.

## Conversion and verification

The models stage of `cmd/extract` writes each walkmesh that has faces to `extracted/data/models/<resref>.<ext>.obj`, for example `m01aa_01a.wok.obj`. Faces keep their order. A "usemtl" line names the face material, such as `07_Nonwalk`, whenever it changes. All walkmesh OBJ files share the material library `walkmesh.mtl`, which gives each row of `surfacemat.2da` its own color: bright for walkable materials and dark for the others. The 1,258 files are 980 area, 156 door and 122 placeable walkmeshes.

An OBJ file cannot hold the other data, so each walkmesh also gets `<resref>.<ext>.json`. It holds the type, the position, the use points, and the perimeters, with each edge's face, edge number within the face and transition. The AABB tree and the adjacency table can be rebuilt from the faces and are not written.

The verify command checks both files of each walkmesh. assimp must load each OBJ file with the face count of the file; all 1,258 pass. jq must read each JSON file, and its values, packed again as binary, must equal the source's header values from 0x08 to 0x47, its edge table and its perimeter table byte for byte; all 1,258 pass.

# Textures: TPC

A TPC file (resource type 3007) is a texture. It holds pixel data for one image, or for several sub-images, each with its own mipmap chain. A mipmap chain is a series of copies of an image, each half the width and height of the previous one. The file ends with an optional TXI trailer. TXI (texture information) is plain ASCII text that sets rendering options. The parser is `ParseTpc` in `kotor/tpc.go`, and `DecodeTpc` decodes the top mipmap level to 8-bit RGBA. The `textures` stage of `cmd/extract` writes it as a PNG file, and `cmd/tpc2png` converts one texture.

All 11,452 TPC files in the four texture packs parse completely. In each file, the header, the pixel data and an ASCII trailer together account for every byte. The converted PNG files are verified with two independent tools. ImageMagick decodes each PNG to exactly the pixels of the decoded TPC texture, and pngcheck accepts each file. This holds for all 3,294 textures of each of tpa, tpb and tpc and for all 1,570 textures of gui. The 84 textures in `patch.erf` also parse completely and pass the same checks (see "Patch archive: patch.erf").

## TPC header

A TPC file begins with a 128-byte header.

| Offset | Type    | Meaning                                                     | Evidence                                                                                                                                                     | Confidence |
| ------ | ------- | ----------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------- |
| 0x00   | u32     | Size of the top mipmap level in bytes, or 0 if uncompressed | Matches the computed size in all plain and cube textures, with the exceptions described below.                                                               | high       |
| 0x04   | f32     | UNKNOWN, a value from 0 to 1                                | The value is 1.0 in 6,987 of 11,452 files. Every other value occurs in an encoding 4 texture, which has an alpha channel. It may be an alpha test threshold. | low        |
| 0x08   | u16     | Width in pixels                                             | Decoded images are coherent at this width.                                                                                                                   | high       |
| 0x0A   | u16     | Height in pixels                                            | Decoded images are coherent at this height.                                                                                                                  | high       |
| 0x0C   | u8      | Encoding (see below)                                        | Every file decodes, and the data sizes match.                                                                                                                | high       |
| 0x0D   | u8      | Number of mipmap levels                                     | The data sizes match in all plain and cube textures.                                                                                                         | high       |
| 0x0E   | u8[114] | Padding                                                     | All bytes are 0 in all 11,452 files.                                                                                                                         | high       |

The encoding byte and the size field at 0x00 together select the pixel format. The combinations are listed below.

| Encoding | Size at 0x00 | Pixel format                      | Files in tpa |
| -------- | ------------ | --------------------------------- | ------------ |
| 1        | 0            | 8-bit grayscale, 1 byte per pixel | 4            |
| 2        | 0            | RGB, 3 bytes per pixel            | 9            |
| 4        | 0            | RGBA, 4 bytes per pixel           | 244          |
| 2        | nonzero      | DXT1, 8 bytes per 4x4 block       | 1,743        |
| 4        | nonzero      | DXT5, 16 bytes per 4x4 block      | 1,294        |

DXT1 and DXT5 are the standard S3 block compression formats. A mipmap level of width w and height h occupies ceil(w/4) \* ceil(h/4) blocks, with at least one block.

For DXT1 textures of 4x4 pixels or smaller, the size field holds 16, but one block is 8 bytes. All such files still end exactly after the computed mipmap chain, so the converter accepts this value. This affects 4 files in tpa, 8 in tpb and 11 in tpc. The reason is not known.

## Pixel data layout

The pixel data starts at offset 128. Each sub-image is stored as its full mipmap chain, largest level first. Three layouts occur.

| Layout   | How to recognize it                                         | Sub-images                                                                                                                 | Files in tpa |
| -------- | ----------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------- | ------------ |
| plain    | Neither of the cases below                                  | One image with the header's mipmap count.                                                                                  | 3,191        |
| cube     | Height equals 6 times the width                             | Six square faces of width x width pixels, each with the header's mipmap count. The TXI holds "cube 1".                     | 63           |
| flipbook | The size field exceeds the top level size of the full image | numx \* numy animation frames. Each frame is (width / numx) x (height / numy) pixels with a full mipmap chain down to 1x1. | 40           |

In a flipbook texture, numx and numy come from the TXI trailer, which also holds "proceduretype cycle" and a frame rate ("fps"). The header's mipmap count is 1 and does not describe the frames. The size field equals the total size of all frames and their mipmaps. For example, c_holododonna is 512x512 DXT1 with numx 2 and numy 2. Each 256x256 frame with 9 mipmap levels is 43,704 bytes, and the four frames total 174,816 bytes, which equals its size field. The converter places the frames in a grid, frame 0 at the top left, in row order. That order is an assumption. Each frame is a complete texture, so a model that uses a flipbook texture appears with the texture tiled 2x2 or more in the converted output.

The order of the six cube faces is UNKNOWN. The converter stacks them vertically in file order.

Each image is stored with its bottom row first. In other words, the first stored row is at texture coordinate v = 0. Two tests support this. When the rows are reversed, the Bastila head model (p_bastilah) renders with its face texture correctly placed; without reversal the eyes and mouth are misplaced. The GUI logo lbl_bwlogo reads upside down unless the rows are reversed. The converter therefore reverses the rows, so the PNG files follow the usual top-row-first convention that OBJ viewers expect.

## TXI trailer

The TXI trailer is ASCII text with one "keyword value" pair per line, separated by CRLF line breaks. It is sometimes followed by NUL bytes. 1,576 of the 3,294 tpa textures have one. The converter writes it beside the PNG as a .txi file. Keywords seen include "proceduretype", "numx", "numy", "fps", "blending", "decal" and "cube". Only numx, numy and cube are used by the converter. The meaning of the other keywords has not been examined.

## Reduced texture packs: tpb and tpc

The packs `swpc_tex_tpb.erf` and `swpc_tex_tpc.erf` hold reduced copies of the textures in `swpc_tex_tpa.erf`. They are probably the medium and low texture quality settings, but the game's choice between packs has not been observed. A program written for this comparison (`temp/packcompare`) parses every texture of the three packs and compares each tpb and tpc texture with the tpa texture of the same name. Every one of the 6,588 reduced-pack textures parses completely.

The following properties hold for both reduced packs.

| Property                             | Evidence                                                                                                                          | Confidence |
| ------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| Same names in the same order         | Each pack has the same 3,294 resrefs as tpa, and each resref has the same index in the key table.                                 | high       |
| Same encoding and layout             | Every texture keeps the pixel format (gray, RGB, RGBA, DXT1 or DXT5) and the layout (plain, cube or flipbook) of its tpa version. | high       |
| Same header float at 0x04            | The value at offset 0x04 equals the tpa value in all 3,294 textures of each pack.                                                 | high       |
| Same TXI trailer                     | The trailer, with trailing NUL bytes removed, equals the tpa trailer in all 3,294 textures of each pack.                          | high       |
| Width and height scale by one factor | The width and the height of each texture shrink by the same factor of 1, 2 or 4. No reduced side is smaller than 2 pixels.        | high       |

The scale factors are listed below. "Unchanged" means that the file is byte-for-byte identical to the tpa file.

| Pack | Unchanged | Half size | Quarter size |
| ---- | --------- | --------- | ------------ |
| tpb  | 570       | 2,724     | 0            |
| tpc  | 570       | 12        | 2,712        |

The 12 textures that tpc reduces only to half size are all 4x4 pixels in tpa. A quarter-size copy would be 1x1, so the reduction appears to stop at 2 pixels per side.

The same 570 textures are unchanged in both packs. They are mostly area textures with the prefixes m50 (300 textures), plc (98), lts (80), lhr (24) and lsi (24), along with sky textures such as lda_dsky01 and lma_sky01. Their sizes range from 4x4 to 1024x1024. Why these textures are not reduced is UNKNOWN.

Most reduced textures are cut from the tpa mipmap chain. A texture reduced by a factor of 2 drops the top mipmap level, and one reduced by a factor of 4 drops the top two levels. The header mipmap count shrinks by the same number of levels, and the remaining pixel data is byte-for-byte identical to the lower levels of the tpa file. This holds for plain textures, for each of the six faces of cube textures and for each frame of flipbook textures. It applies to 2,689 of the 2,724 reduced textures in tpb and to 2,689 of the 2,724 in tpc.

The other 35 reduced textures in each pack are plain textures with a single mipmap level in tpa, so no lower level exists to copy. Their reduced pixels were computed in some other way, and the resampling filter is UNKNOWN. Examples are biowarelogo, plc_water01b and lta_tech01b.

The reduced packs are converted the same way as tpa, to `extracted/TexturePacks/swpc_tex_tpb/` and `extracted/TexturePacks/swpc_tex_tpc/`. Model materials still refer to the tpa and gui PNG files.

## Conversion to model materials

Each converted model has an MTL (material library) file beside its OBJ file. The MTL file defines one material per texture name, and each material points at the PNG in `extracted/TexturePacks/swpc_tex_tpa/`, or in `swpc_tex_gui/` if the name is only found there. Texture names are compared without regard to case. Of the 1,401 distinct texture names that rendered meshes refer to, 1,364 are found. The converter lists the remaining 37 in its summary and marks them in the MTL files. Eight of the 37 are resources of other types in the BIF archives. Five, `m17mg_01c_a0001l` to `m17mg_01c_a0001p`, are TGA images in `lightmaps6.bif`, which are converted to `extracted/data/lightmaps/` but are not linked from the MTL files. Two, `load_sw` and `n_repsold`, are model names. One, `h_f_lo01headtest`, exists only as a TXI file in `textures.bif`. The other 29 names occur in no archive. Of all 37, only `n_repsold` also occurs in a RIM archive.

The PNG files keep their alpha channel, but the MTL files do not declare it as transparency. In KotOR the alpha channel of some textures appears to hold other data, such as an environment map mask, rather than transparency. This has not been verified.

# Images: TGA in the BIF archives

Resource type 3 is a TGA image, the uncompressed image format defined by Truevision. The BIF archives hold 5,601 of them. They matter because 5,586 of them are lightmaps, the precomputed lighting that the game blends over area textures. The converter is `kotor/tga.go`, called by the `tga` stage of `cmd/extract`. It writes 8-bit PNG files with the same channels as the TGA.

The images are in three groups. The table below lists each group, its archives and its output directory.

| Archives                                               | Images | Layout                            | Output                      |
| ------------------------------------------------------ | ------ | --------------------------------- | --------------------------- |
| `lightmaps.bif`, `lightmaps2.bif` to `lightmaps13.bif` | 5,586  | 32-bit BGRA, sizes 2x2 to 512x512 | `extracted/data/lightmaps/` |
| `textures.bif`                                         | 14     | 8-bit grayscale, 256x256          | `extracted/data/textures/`  |
| `legacy.bif`                                           | 1      | 24-bit BGR, 256x256 (`chrome1`)   | `extracted/data/legacy/`    |

All 5,601 images pass the parser's check: the header, the optional image ID, the pixels and the optional trailer account for every byte of the file. No resource name occurs twice within a group, so each output directory is flat.

## TGA header

A TGA file begins with an 18-byte header. The table below gives each field and the values seen in all 5,601 images.

| Offset | Type  | Meaning                             | Evidence                                                                                                       | Confidence |
| ------ | ----- | ----------------------------------- | -------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u8    | Length of the image ID that follows | The value is 0 in all 5,601 images.                                                                            | high       |
| 0x01   | u8    | Color map type                      | The value is 0 (no color map) in all 5,601 images.                                                             | high       |
| 0x02   | u8    | Image type                          | 2 (uncompressed true color) in 5,587 images and 3 (uncompressed grayscale) in the 14 images of `textures.bif`. | high       |
| 0x03   | u8[5] | Color map specification             | All bytes are 0 in all 5,601 images.                                                                           | high       |
| 0x08   | u16   | X origin                            | The value is 0 in all 5,601 images.                                                                            | high       |
| 0x0A   | u16   | Y origin                            | The value is 0 in all 5,601 images.                                                                            | high       |
| 0x0C   | u16   | Width in pixels                     | The pixel data size matches width x height x bytes per pixel in all 5,601 images.                              | high       |
| 0x0E   | u16   | Height in pixels                    | As above. Every image is square, and every side is a power of 2.                                               | high       |
| 0x10   | u8    | Bits per pixel                      | 32 for lightmaps, 8 for `textures.bif` and 24 for `legacy.bif`.                                                | high       |
| 0x11   | u8    | Image descriptor                    | See below.                                                                                                     | high       |

The image descriptor holds the number of alpha bits in its low 4 bits and the pixel origin in bits 4 and 5. Bits 4 and 5 are 0 in all 5,601 images. This means the first stored row is the bottom row of the image, and pixels run left to right. ImageMagick reports every image as bottom-left oriented. Its decoding with `-auto-orient` and ffmpeg's decoding both match the converter's output, in which the rows are reversed to put the top row first. The alpha bit count is 0 in 5,474 lightmaps and 8 in the other 112 lightmaps and in the 14 grayscale images. The game does not appear to rely on it: all 5,474 lightmaps that declare 0 alpha bits still store 4 bytes per pixel.

True color pixels are stored in the order blue, green, red and, at 32 bits per pixel, alpha. ImageMagick's decoding of all 5,601 source files equals the converter's RGBA output, which confirms this order.

## Trailer

Some images end with the optional TGA 2.0 trailer. The table below lists the three cases.

| Bytes after the pixels | Contents                                 | Images                                                      |
| ---------------------- | ---------------------------------------- | ----------------------------------------------------------- |
| 0                      | None                                     | 5,474 lightmaps                                             |
| 26                     | Footer only                              | 5 lightmaps, 14 images of `textures.bif`, 1 of `legacy.bif` |
| 521                    | 495-byte extension area, then the footer | 107 lightmaps                                               |

The footer is 26 bytes long. It holds a u32 offset of the extension area, a u32 offset of the developer area and the signature "TRUEVISION-XFILE." followed by a NUL byte. The developer area offset is 0 in all 127 files with a footer. The extension area offset is 0 when there is no extension area. When there is one, the offset equals the end of the pixel data. The first field of the extension area holds its size. That field holds 494 in all 107 files, but 495 bytes are present, which is the size the TGA 2.0 standard specifies. The cause of this difference is not known. The converter does not read the rest of the extension area.

The 112 lightmaps with a trailer are the same 112 that declare 8 alpha bits. All of them belong to the areas `m09zz` and `m17mg`.

## Lightmaps

Lightmaps are named after an area model, followed by "\_lm" and a number, for example `m01aa_01a_lm0`. Each lightmap has a TXI file of the same name in the same archive, resource type 2022. TXI is plain text with one option per line, as in the TPC trailer above. The TXI files of the 5,474 lightmaps without a trailer all read "islightmap 1", "compresstexture 0", "mipmap 0" and "downsamplemax 0". The TXI files of the other 112, which are the lightmaps with a trailer, lack the "islightmap 1" line. The effect of this difference in the game is unknown. The converter writes each TXI file beside its PNG file.

The alpha channel is 255 everywhere in 3,221 lightmaps. In the other 2,365, some pixels have lower alpha values, down to 0. The meaning of the lightmap alpha channel is UNKNOWN. The converter keeps it.

How a mesh selects its lightmap and which texture coordinates it uses are not documented. Area room models are converted (see the model section), but the converter does not attach the lightmaps to them.

## Other resources in the same archives

`textures.bif` also holds 28 TXI files. Fourteen belong to its grayscale images and contain "isbumpmap 1", which suggests the images are bump maps. The other 14, such as `distortiontex` and `grassinfo`, have no image in that archive. The converter writes all 28.

The lightmap archives also hold 112 room visibility lists of type 3001. They are described in the next section.

# Room layouts: LYT

A LYT file is the room layout of one area. It places the room models of the area, the track and obstacle models of the minigames, and the door hooks, the places where doors stand. It matters because the room models are stored without their position: the layout is what assembles them into an area. Each of the 124 files is one resource of type 3000 in `data/layouts.bif`, named after its area, such as `m01aa`. The parser is `ParseLyt` in `kotor/lyt.go`.

The resource type is named LYT after the string ".lyt" in `swkotor.exe`. It stands next to the strings "trackcount", "roomcount" and "%s %f%f%f". Near them are "beginlayout", "donelayout", "%\*s%d", "%[^\r\n]" and "%s%s%d%f%f%f%f%f%f%f". These are scan formats of the C function `sscanf`, and they match the line forms below: a room or track line has a name and three floats, a count line has a keyword and an integer, and a door hook line has two names, an integer and seven floats. The strings "obstaclecount" and "doorhookcount" do not occur in the executable, so how the game finds those two sections is UNKNOWN.

## Text layout

A LYT file is ASCII text with CRLF line breaks, between 171 and 5,129 bytes long. The first line, "#MAXLAYOUT ASCII", and the misspelled keyword "filedependancy" suggest that the files were exported from 3ds Max. All 124 files have the lines below in the same order. The table gives each line form, its canonical indentation and its fields.

| Line            | Indent   | Form                                                       | Evidence                                                                | Confidence |
| --------------- | -------- | ---------------------------------------------------------- | ----------------------------------------------------------------------- | ---------- |
| Signature       | none     | `#MAXLAYOUT ASCII`                                         | First line of all 124 files.                                            | high       |
| File dependency | none     | `filedependancy` and the name of a 3ds Max scene           | Second line of all files. The name ends in ".max" in all files.         | high       |
| Begin           | none     | `beginlayout`                                              | Third line of all files. The string is in `swkotor.exe`.                | high       |
| Room count      | 3 spaces | `roomcount` and a decimal count                            | All files. Followed by exactly that many room lines.                    | high       |
| Room            | 6 spaces | Model name and the position x, y and z as floats           | 1,401 lines. Matches the scan format "%s %f%f%f".                       | high       |
| Track count     | 3 spaces | `trackcount` and a decimal count                           | All files. Followed by exactly that many track lines.                   | high       |
| Track           | 6 spaces | Model name and position, as in a room line                 | 173 lines in 12 files.                                                  | high       |
| Obstacle count  | 3 spaces | `obstaclecount` and a decimal count                        | All files. Followed by exactly that many obstacle lines.                | high       |
| Obstacle        | 6 spaces | Model name and position, as in a room line                 | 95 lines in 11 files.                                                   | high       |
| Door hook count | 3 spaces | `doorhookcount` and a decimal count                        | All files. Followed by exactly that many door hook lines.               | high       |
| Door hook       | 6 spaces | Room name, door name, an integer, position and orientation | 897 lines in 103 files. Matches the scan format "%s%s%d%f%f%f%f%f%f%f". | high       |
| Done            | none     | `donelayout`                                               | Last line of all files. The string is in `swkotor.exe`.                 | high       |

Fields are separated by single spaces. Every count equals the number of entry lines that follow it, in all 496 sections of the 124 files. Numbers are written with at most six significant digits and always with a decimal point, such as "0.0" and "-1.31576", or in exponent form with three exponent digits, such as "1.26441e-007". This is the form of the C format "%g" in Microsoft's runtime library, with ".0" added to whole numbers. The game reads them as 32-bit floats.

The fields of a door hook line are given in the table below, in line order.

| Field      | Type     | Meaning                                               | Evidence                                                                                                                     | Confidence |
| ---------- | -------- | ----------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------- | ---------- |
| Room       | name     | Room model that holds the door                        | Outside the `stunt_` areas, all 838 hooks name a room of the same layout.                                                    | high       |
| Door       | name     | Name of a node in the room model, such as `Door_02`   | Of the 893 hooks whose room has a model, 889 name a node of that model. All names are "Door*" or "door*" and a number.       | high       |
| Unknown    | integer  | UNKNOWN                                               | The value is 0 in all 897 lines.                                                                                             | low        |
| x, y, z    | 3 floats | Position of the door in area coordinates              | Module door instances stand at these positions (see below).                                                                  | high       |
| w, x, y, z | 4 floats | Orientation of the door as a unit quaternion, w first | All 897 quaternions have length 1. In 885 the second and third value are 0, which leaves a rotation about the vertical axis. | high       |

The positions are in the coordinates of the area, with z pointing up. Room models are built around their own origin, while the rooms stand far from the origin of the area. In half of the 1,162 room models with geometry, no vertex lies farther than 24.7 units from the model origin along x or y. The median room line places its room 155.6 units from the area origin horizontally. The room line therefore gives the translation that moves the room into the area. No line holds a rotation of a room, a track or an obstacle. Of the 1,401 room lines, 505 have a z other than 0.

## Comparison with the module door instances

The door hooks agree with the door instances of the modules. The GIT files of the modules (see "Modules") list each door with its position "X", "Y", "Z" and its "Bearing", a rotation about the vertical axis in radians. For each of the 716 doors of the GIT files whose area has a layout, the nearest door hook of that layout was found.

- 673 doors stand within 0.01 units of a door hook, 38 within 0.5 units, and 5 farther away.
- For 664 of the 673 close pairs, the bearing equals the rotation 2·atan2(z, w) of the quaternion, read with w first. For the other 9, it differs by half a turn.

This confirms both the position fields and the order of the quaternion. Whether the game uses the door hooks or the GIT positions to place doors is UNKNOWN.

## Model names

Every room, track and obstacle name, apart from the placeholder below, names a model in `data/models.bif`, ignoring case. This holds for all 1,267 such rooms, all 173 tracks and all 95 obstacles. Names are written in mixed case, such as `M01aa_08c`. Track models are named like `m03mg_MGT02` and obstacle models like `m03mg_MGO01`. They belong to the areas `m03mg`, `m17mg`, `m26mg` and `m45mg`, which are probably the swoop races, and to `m12ab` to `m12ah` and `mgf_ebonhawk`. The track `MGF_turretcam` of `mgf_ebonhawk` suggests that the latter are the turret minigame. In the seven areas `m12ab` to `m12ah`, all 49 track lines place their track at the origin of the area.

The 13 `stunt_` areas are copies of other areas, probably for cutscenes (see "Module and area names"). Their file dependency names the original scene, such as `M01aa.max` for `stunt_endbridge`, while the other 111 files name their own area. In 11 of them, 134 room lines have the name `****` in place of a room. They keep the position of the original room, which suggests that the room is left out. Their door hooks still name the original rooms: 59 hooks name a room that the layout does not list.

## Irregularities

The parser accepts the following irregularities. It reports them, and it fails on any other deviation from the line forms above.

- In the six areas `m12ac` to `m12ah`, the single room line is indented by a tab. The scan formats skip any white space, so this has no effect.
- In `m12aa`, the 9 bytes "M12aa_01m" follow the line break of the closing line, without a line break of their own. They name a room of the same layout. The game probably stops reading at the closing line.

## Conversion and verification

The `bifs` stage of `cmd/extract` writes each file to `extracted/data/layouts/<area>.lyt.json`. The JSON object has these members:

- "filedependancy", the scene name as written;
- "rooms", "tracks" and "obstacles", with one object per line in file order, each with "model" and "position", an array of x, y and z;
- "doorhooks", with one object per line, each with "room", "door", "unknown", "position" and "orientation", an array of w, x, y and z;
- "trailer", the text after the closing line, which is empty except in `m12aa`.

The counts are the lengths of the arrays. Indentation is not kept. Numbers are written with the fewest digits that read back as the same 64-bit float as the text.

The parser passes for all 124 files: it consumes every byte, finds only printable ASCII, tabs and CRLF line breaks, and finds every count equal to the number of entries.

The `bifs` stage of `cmd/verify` reads each JSON file with jq, reports in `temp/reports/jq_bifs.txt`, and builds the lines of the layout again, with the counts taken from the array lengths. It compares them with the source lines field by field: names as text and numbers by value, because jq writes numbers in its own form. White space is not compared. The trailer must equal the bytes after the closing line. All 124 files match. Two tests were reported as failures: one position changed in `m01aa.lyt.json`, and the trailer of `m12aa.lyt.json` removed.

# Room visibility lists: VIS

A VIS file lists, for each room of an area, the other rooms that can be seen from it. The game probably uses it to decide which room models to draw. Each of the 112 files is one resource of type 3001 in `data/lightmaps*.bif`, named after its area, such as `m01aa`. The archives hold between 3 (`lightmaps12.bif`) and 31 (`lightmaps13.bif`) of them. The parser is `ParseVis` in `kotor/vis.go`.

The resource type is named VIS after two strings in `swkotor.exe`: the extension ".vis" and the path pattern "%s/%s.VIS". The strings " %s" and "%s %d" stand next to them. These match the two line forms below, so the code that reads or writes the file is probably nearby.

## Text layout

A VIS file is ASCII text with CRLF line breaks. The files are between 11 and 9,853 bytes long. There are two kinds of lines, described in the table below.

| Line              | Form                                           | Meaning                                                 | Evidence                                                                           | Confidence |
| ----------------- | ---------------------------------------------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------- | ---------- |
| Room line         | Name, a space and a decimal count, at column 0 | Starts the entry of one room of the area                | Every one of the 1,250 unindented lines has this form, except one without a count. | high       |
| Visible-room line | Two spaces and a name                          | A room visible from the room of the preceding room line | All 8,905 indented lines have this form. None comes before the first room line.    | high       |

The count is meant to give the number of visible-room lines that follow. It does so for 1,246 of the 1,250 rooms. The parser takes the list of visible rooms from the indentation, not from the count, because the count is wrong in four rooms. Three of them list one room fewer than the count (`m38ab_08`, `m38ab_06`) or one more (`m39aa_15`). The fourth, `m17aa_09w` in `m17aa`, has no count and no visible rooms. If the game read a fixed number of lines from the count, the next room line would be taken as a visible room. Whether the game reads by count or by indentation is UNKNOWN.

The files contain a few irregularities in spacing and line breaks. The parser accepts all of them:

- 13 files, such as `m01aa`, lack a line break after the last line.
- In `m36aa`, the first room line has two spaces before its count.
- In `m17aa` and `m22aa`, one visible-room line each has a space after the name.

Each file holds between 1 and 54 rooms, and each room lists between 0 and 44 visible rooms. Of the 1,250 rooms, 65 list no visible room. Room names are at most 16 characters long and are written in mixed case, such as `M01aa_10` beside `m01aa_09`. Case is probably not significant, because resource references are not case-sensitive.

## Relation to room layouts

The room names agree with the room layouts (LYT, type 3000) of `data/layouts.bif`, described in the previous section, which list each area's rooms with their positions. This is the main evidence that the file is about rooms. The comparison below ignores case.

- 111 of the 112 areas have a layout of the same name. Only `m47ac` has none. Thirteen layouts, such as `m09zz` and `m12ac`, have no VIS file.
- In these 111 areas, 1,190 of the 1,248 room lines name a room of the layout. The other 58 are in 14 areas, such as `m02ae`, `m25aa` and `m40ab`, and are probably rooms that were renamed or removed after the file was made. In `m41az`, six room lines name rooms of `m41ad` instead.
- Of the 8,905 visible-room lines, 371 name no room of the layout. Most belong to the same 14 areas. Two are typing errors: `m17aa11b` for `m17aa_11b` in `m17aa`, and `m38aa_07` in `m39aa`.
- Of the 1,221 layout rooms, 31 have no room line. Fifteen of them are the placeholder `****` of the `stunt_` areas.
- Visibility is mostly symmetric. Where both rooms of a listed pair have room lines, the reverse pair is also listed in 8,211 of 8,877 cases.
- Ten rooms list themselves, and six lists name a room twice.

The visible rooms are not simply the rooms nearby. Using the layout positions, the nearest other room is listed in 950 of 1,161 cases, and the mean distance to a listed room (89.3 units) is only slightly shorter than to an unlisted one (95.3 units). This agrees with a visibility list computed from lines of sight, but the method that produced it is UNKNOWN.

## Conversion and verification

The `tga` stage of `cmd/extract` writes each file to `extracted/data/lightmaps/<area>.vis.json`, beside the lightmaps. The JSON object has "rooms", an array with one object per room line in file order. Each room has "room", the name as written; "count", the count as written, or null when the line has none; and "visible", the names of the visible-room lines in file order. Extra spaces and a missing final line break are not kept.

The parser passes for all 112 files: it consumes every byte, finds only printable ASCII and CRLF line breaks, and finds no line of another form. Its report lists the irregularities of each file.

The `tga` stage of `cmd/verify` reads each JSON file with jq, reports in `temp/reports/jq_vis_lightmaps.txt`, builds the text again, and compares it with the source in canonical form. The canonical form has a CRLF after every line, a single space before each count and no space after a name. All 112 files match. A test with one count changed in `m01aa.vis.json` was reported as a failure.

# User interface layouts: GUI

A GUI file (resource type 2047) describes one screen or panel of the user interface. It lists the screen's controls, such as buttons, labels and list boxes, with their position, size, textures, font and text. The game contains 91 GUI resources. 84 are in `data/gui.bif`, which holds nothing else, and 7 are in `patch.erf`. No other archive holds one: `chitin.key` lists type 2047 only for `gui.bif`, the 246 RIM files hold none, and the signature "GUI V3.2" occurs in no other file. The reader is `ReadGff` in `kotor/gff.go`. The `gui` stage of `cmd/extract` writes one JSON file per resource of `gui.bif` to `extracted/data/gui/`, and the `patch` stage writes those of `patch.erf` to `extracted/patch/`.

A GUI file is a GFF file of file type "GUI " (see "Generic file format: GFF"). All 91 files parse under the strict rules of the reader. In addition, every byte of their three data regions is referenced, and every label is used. In the 84 `gui.bif` files, every struct has between 4 and 14 fields, and nine field types occur: 0, 4, 5, 8, 10, 11, 14, 15 and 17. The only list (type 15) is CONTROLS, the only u32 (type 4) is STRREF, and the only vector (type 17) is COLOR.

The JSON output is verified with jq, which parses all 91 JSON files without error. This shows only that the files are valid JSON. No independent check currently compares the JSON values with the binary fields.

The counts in the sections below were measured on the 84 `gui.bif` files and do not include `patch.erf`. The seven `patch.erf` files use the same fields, and they parse with the same strict checks.

## Resources in patch.erf

Five of the seven `patch.erf` resources have the same name as a `gui.bif` resource but different content. These are `abilities`, `character`, `equip`, `inventory` and `mainmenu`. The game probably uses the `patch.erf` version, because the archive's name suggests an update, but this has not been confirmed. The other two, `mipc212x10` and `tooltip12x10`, exist only in `patch.erf`. Both are 1280x1024 panels, a resolution that no `gui.bif` file has.

The changes are small. `abilities`, `character`, `equip` and `inventory` each gain two buttons, BTN_CHARLEFT and BTN_CHARRIGHT, which are 28x28 pixels at the same positions in all four files. The names suggest buttons that step between party members. The patched `mainmenu` has the same 13 controls, with BTN_WARP and BTN_EXIT in swapped order, and its panel is 800x600 instead of 640x480.

## GUI layout content

The root struct describes the panel itself. It has the fields CONTROLTYPE (always 2), TAG, EXTENT, BORDER, COLOR, ALPHA and CONTROLS. CONTROLS is a list of control structs. No control has its own CONTROLS list. Nesting is instead expressed by Obj_ParentID and Obj_Parent. In all 208 controls where Obj_ParentID is not -1, it equals the ID of another control in the same list, and Obj_Parent equals that control's TAG. A value of -1 appears to mean that the parent is the panel itself.

EXTENT is a struct with LEFT, TOP, WIDTH and HEIGHT in pixels. The root EXTENT of full-screen panels matches the resolution that the file name encodes. For example, the `8x6` files are 800x600, the `10x7` files are 1024x768, and the `16x12` files are 1600x1200. The exceptions are `mainmenu10x7`, `mainmenu12x9` and `mainmenu16x12`, which are all 800x600. The reason is UNKNOWN. Forty-nine screens without a suffix are 640x480.

The control kind is given by CONTROLTYPE. The correspondence below comes from the prefixes of the controls' TAG names and from where each kind appears. It has not been confirmed in the running game.

| CONTROLTYPE | Count | Probable kind       | Evidence                                                                            | Confidence |
| ----------- | ----- | ------------------- | ----------------------------------------------------------------------------------- | ---------- |
| 2           | 84    | Panel               | Occurs only on the root struct of every file.                                       | high       |
| 4           | 1,155 | Label               | 1,033 tags begin with "LBL\_".                                                      | medium     |
| 5           | 77    | UNKNOWN             | Tags begin with "LBH\_", "LBL\_" or "LBLH\_". The kind is UNKNOWN.                  | low        |
| 6           | 711   | Button              | 589 tags begin with "BTN\_".                                                        | medium     |
| 7           | 73    | Toggle or check box | Tags begin with "TB\_", "CB\_" or "BTN\_".                                          | low        |
| 8           | 7     | Slider              | Tags begin with "SLD\_" or "SLI\_".                                                 | medium     |
| 9           | 81    | Scroll bar          | Occurs only in the 81 SCROLLBAR structs.                                            | high       |
| 10          | 75    | Progress bar        | All 75 tags begin with "PB\_".                                                      | medium     |
| 11          | 81    | List box            | 78 tags begin with "LB\_". Only these controls have SCROLLBAR or PROTOITEM structs. | medium     |

All 81 list boxes have a SCROLLBAR struct. Eighty of them also have a PROTOITEM struct, which is probably the template control for one row. The exception is LB_DESC in `upgrade.gui`. The SCROLLBAR holds DIR and THUMB structs that describe its arrow and thumb images.

BORDER, HILIGHT, SELECTED and HILIGHTSELECTED describe a frame and its fill. Each holds the resrefs CORNER, EDGE and FILL, together with FILLSTYLE, DIMENSION, INNEROFFSET, COLOR and PULSING. TEXT is a struct with TEXT (a string), STRREF, FONT, ALIGNMENT, COLOR and PULSING. The text of a control comes either from the TEXT string or from STRREF, a string number. The largest STRREF value, 49,116, is below the 49,265 entries of the string table (see "String table: dialog.tlk").

Resrefs name textures and fonts. All 2,016 FONT values name a known resource. Of the CORNER, EDGE, FILL and IMAGE values that are not empty, all name a texture in the texture packs or a resource in `chitin.key`, except nine FILL names, for example `po_mzaalbar` and `georgem`. None of the nine occurs in a RIM archive either.

The meanings of FILLSTYLE (0, 1 or 2), DIMENSION, INNEROFFSET, PULSING, DRAWSTYLE and FLIPSTYLE are UNKNOWN. ALIGNMENT takes only the values 9, 10, 12, 17, 18, 20 and 34. These values fit a bit field in which 1, 2 and 4 select the horizontal position and 8, 16 and 32 select the vertical position. This is a hypothesis that has not been tested. The meaning of COLOR (-1, -1, -1) is UNKNOWN. It may mean that no color is set.

# Patch archive: patch.erf

`patch.erf` is an ERF archive (see "ERF archive") in the game folder. It holds 97 resources that replace or add to resources stored elsewhere. Its header holds the build date day 47 of 2004, which is later than that of any other archive of the installation, so it is probably an update made after the game's release. Only the archives of the saved game, which the game writes when it saves, are later. Whether the game prefers these copies to the copies of the same name in other archives has not been observed. The `patch` stage of `cmd/extract` converts every resource to `extracted/patch/`, and the `patch` stage of `cmd/verify` checks the output.

The table below lists the contents by resource type.

| Type                   | Resources | Relation to other archives                                 | Output                                      |
| ---------------------- | --------- | ---------------------------------------------------------- | ------------------------------------------- |
| GUI layout (2047)      | 7         | 5 have the name of a `gui.bif` layout, and 2 are new.      | `<resref>.json`                             |
| TPC texture (3007)     | 84        | 75 have the name of a texture pack texture, and 9 are new. | `<resref>.png`, and `<resref>.txi` for 9    |
| 2DA table (2017)       | 4         | All 4 have the name of a `2da.bif` table.                  | `<resref>.2da.csv`                          |
| MDL model (2002)       | 1         | Has the name of an `items.bif` model.                      | `w_lghtsbr_008.obj` and `w_lghtsbr_008.mtl` |
| MDX vertex data (3008) | 1         | Byte-for-byte identical to the `items.bif` copy.           | Used for the model, not written on its own  |

The GUI layouts are described in "Resources in patch.erf" in the GUI section. Every resource of the other types has a different content from its namesake, except the MDX resource. The differences are described below.

## Textures

All 84 textures parse completely with the TPC parser: the header, the pixel data and the TXI trailer account for every byte. They form three groups.

**Female head textures.** 75 textures are named `pfha01` to `pfhc05`, three series of five, each with the variants suffixed d, d1, d2 and d3. The name probably means "player female head". `rims/chargen.rim` holds 15 head models with the same base names, whose materials use these textures. `patch.erf` replaces all 75 `pfh` textures and none of the 76 male `pmh` textures.

Each patch copy is 256x256 DXT1 with 9 mipmap levels, the same format as the `swpc_tex_tpa` copy, and is 43,832 bytes long. Each `swpc_tex_tpa` copy is 9 bytes longer because it ends with the TXI trailer "clamp 3" and CRLF. The patch copies have no trailer. The reason for removing it is UNKNOWN.

In 60 of the 75 textures, the patch copy is otherwise byte-for-byte identical to the `swpc_tex_tpa` copy. The other 15 are `pfhc02`, `pfhc03` and `pfhc04` with their four variants each. In these, the top mipmap level differs only in the last column of 4x4 blocks, that is, in the rightmost 4 pixels of the image. The column differs in all 64 block rows of `pfhc02` and `pfhc03` and in 48 block rows of `pfhc04`, all in the top 212 image rows. Every lower mipmap level also differs in some bytes, which fits mipmaps that were generated again from the edited image. In the `swpc_tex_tpa` copies, this column holds stray colors that do not match the neighboring skin. In the patch copies, it holds smooth skin tones. The edit is probably a fix for a visible seam on the head model, but this has not been checked in the game.

The patch copies are full size, whereas the `swpc_tex_tpb` and `swpc_tex_tpc` copies are 128x128 and 64x64. If the game prefers the patch copies, these heads would therefore appear at full resolution at every texture quality setting. This is a deduction that has not been tested.

**Backgrounds for 1280x1024.** Seven textures are named `1280x1024` followed by `back`, `comp0`, `comp1`, `load`, `map`, `pazaak` or `store`. Each is 1024x1024 DXT1 with one mipmap level and the TXI trailer "mipmap 0", a space and CRLF. `swpc_tex_gui.erf` holds the same seven suffixes with the prefixes `800x600`, `1024x768`, `1280x960` and `1600x1200`, but not with `1280x1024`. Those 28 textures are DXT5. Why the patch uses DXT1 is UNKNOWN.

The executable `swkotor.exe` holds the strings "800x600", "1600x1200", "1280x1024", "1280x960" and "1024x768" at file offsets 0x33DF7C to 0x33DFA8. The strings "comp", "map", "pazaak", "store" and "back" follow at 0x33E0B8 to 0x33E0D0. This suggests that the game builds a background name from the screen resolution and a suffix. The patch thus adds the backgrounds for a resolution that the executable names but the texture packs lack. The two new GUI layouts, `mipc212x10` and `tooltip12x10`, belong to the same resolution: their root EXTENT is 1280x1024, and their control tags equal, in order, those of `mipc212x9` and `tooltip12x9` in `gui.bif`.

**Arrow buttons.** `cyclearrow1` and `cyclearrow2` are 64x64 DXT5 textures with one mipmap level, the value 0.2429 in the header float at 0x04, and the TXI trailer "mipmap 0". Both show an arrow that points left, blue in `cyclearrow1` and yellow in `cyclearrow2`. They are the BORDER FILL and HILIGHT FILL of the buttons BTN_CHARLEFT and BTN_CHARRIGHT, which the patch adds to the layouts `abilities`, `character`, `equip` and `inventory`. Both buttons use the same left-pointing images, and no field of these controls marks a mirrored image. How the right-hand button shows an arrow that points right is UNKNOWN.

## Tables

All four tables use the binary form "2DA V2.b" with tab separators and parse with `ParseTwoDa`. Each was compared cell by cell with its `2da.bif` copy, by row label and column label. The table below summarizes the differences.

| Table             | Rows: `2da.bif`, patch | Differences                                                                                                                                                                                                                                                                                      |
| ----------------- | ---------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `bindablekeys`    | 85, 103                | The column `bindable` changes from 0 to 1 for the four arrow keys and from 1 to 0 for KEYBOARD_LEFTALT, KEYBOARD_RIGHTALT and KEYBOARD_PRINTSCREEN. Eighteen rows are added, `key85` to `key102`, for keys such as KEYBOARD_F13, KEYBOARD_MINUS and KEYBOARD_SLASH. Their `keynamestrref` is 0.  |
| `keymap`          | 79, 80                 | A column `eventtype` is added. One row is added, `action265`, named WALKMODIFY, whose strings 49,142 and 49,143 in `dialog.tlk` read "Run / Walk" and describe a key that makes the character walk instead of run. In `AlternateActions` and `AlternateActions2`, `icpcgui` changes from 0 to 1. |
| `skills`          | 8, 8                   | The `description` of row 3, Awareness, changes from string 250 to string 49,138. The new text omits the sentence "Running imparts a -5 penalty to any checks of the skill." and the clause saying results are best while moving slowly.                                                          |
| `weapondischarge` | 64, 64                 | In row 363, `shots` and `hits` change from 1 to 2, `switchmask` changes from empty to "01", and `shot2` changes from empty to 900.                                                                                                                                                               |

The new column `eventtype` of `keymap` holds 0 in 65 rows and 1 in the new row `action265`. It is empty in the 14 rows `action280a` to `action286b`, which name direction pairs such as ActionUp and CameraRotateLeft. Its meaning is UNKNOWN.

In `weapondischarge`, the cells `shot1` to `shot12` appear to hold one value per shot. In 59 of the 61 patched rows that have a `shots` value, exactly that many shot cells are filled, and in 15 of the 17 rows with a `switchmask`, its length equals `shots`. The values range from 0 to 1,367 and may be times in milliseconds. The patch makes row 363 equal in shape to row 233, which has the same `shots`, `hits` and `switchmask`. The meaning of the row labels and the columns is otherwise UNKNOWN.

All new string numbers, 49,138, 49,142 and 49,143, lie within the 49,265 entries of `dialog.tlk`, so the string table already contains the patched text.

## Model

`w_lghtsbr_008` is probably a lightsaber, as its name and its texture names `w_lghtsbr_001` and `w_lsabreturq01` suggest. The MDL resource is 52,323 bytes long in both `patch.erf` and `items.bif`, and the two copies differ in 826 bytes. The MDX resource is identical in both.

Both copies pass the model converter. They have the same 7 nodes, 5 meshes, 1,006 vertices and 251 faces, and the same texture names. In the two OBJ files, every face, normal and texture coordinate is identical. Vertex positions differ by at most 0.000004. The vertex data is identical, so these small differences must come from the node transforms that the converter applies.

Most differing bytes lie in unused bytes or in regions that the MDL section does not decode. For example, the model name at 0x08 of the model header is "w_Lghtsbr_008" followed by a NUL byte. In the rest of the 32-byte field, the bytes from 0x18 to 0x27 are not all zero in the `items.bif` copy, whereas the patch copy holds zeros from the NUL byte to the end of the field. In 177 of the 423 differing 4-byte words of the model data, the patch copy holds 0 where the `items.bif` copy holds another value. This suggests that the patch copy was written by a tool that clears unused memory, but the purpose of the other changes is UNKNOWN. The `items.bif` copy is not converted, because the models of `items.bif` are not converted.

## Conversion and verification

The `patch` stage of `cmd/extract` passes every resource to the converter that the BIF and RIM stages use. Textures become 8-bit RGBA PNG files, as in the texture packs, and a TXI file is written when the trailer holds text. Tables become CSV files, layouts become JSON files, and the model becomes an OBJ file with an MTL file. The MTL file refers to the PNG files in `extracted/TexturePacks/swpc_tex_tpa/` by relative paths, and both textures exist there. All 97 resources pass: 84 textures, 7 layouts, 4 tables, the model, and the MDX resource, whose report line only confirms that a model of the same name exists.

The `patch` stage of `cmd/verify` checks the output with independent tools. All checks pass:

- ImageMagick decodes each of the 84 PNG files to exactly the pixels of the decoded TPC texture, and pngcheck accepts each of them.
- Each of the 9 TXI files equals the texture's trailer.
- Go's encoding/csv reader reads each of the 4 CSV files, and the values build again into the source table byte for byte.
- jq parses each of the 7 JSON files.
- assimp loads the OBJ file with the same face count as the file.

A test with a deliberately wrong PNG file and a changed TXI file made the pixel check and the TXI check fail, as expected.

# Music: streammusic/\*.wav

The game's music is in `streammusic/`, which holds 119 files. All of them have the extension `.wav`, but none can be played as stored. They use two layouts, each of which hides a standard audio file behind extra bytes. The parser is `UnwrapAudio` in `kotor/audio.go`. All 119 files pass its check: walking the MP3 frames or WAV chunks of each file ends exactly at the end of the file.

## Layout A: MP3 behind a fake WAV header

The 68 numbered tracks, such as `01b.wav`, begin with a 58-byte WAV header that does not describe the data. An MP3 stream follows it.

| Offset | Type    | Meaning                                          | Evidence                                           | Confidence |
| ------ | ------- | ------------------------------------------------ | -------------------------------------------------- | ---------- |
| 0x00   | char[4] | "RIFF"                                           | True in all 68 files.                              | high       |
| 0x04   | u32     | RIFF size, always 50                             | True in all 68 files.                              | high       |
| 0x14   | bytes   | Format block claiming mono, 22,050 Hz, 8-bit PCM | True in all 68 files. The claim is false.          | high       |
| 0x36   | u32     | Data size, always 0                              | True in all 68 files.                              | high       |
| 0x3A   | bytes   | MP3 stream: MPEG-1 Layer III at 44,100 Hz        | The frames end exactly at the end of all 68 files. | high       |

In 51 of these files the MP3 stream begins with an ID3v2 tag and ends with a 128-byte ID3v1 tag. In the other 17 it begins directly with an MP3 frame.

## Layout B: WAV behind an MP3 stub

The 50 ambient files, whose names begin with `al_`, and `mus_loadscreen.wav` begin with 470 bytes of MP3 data. These bytes form three MPEG-2 Layer III frames at 22,050 Hz and contain the encoder string "LAME3.93". A complete WAV file follows at offset 0x1D6 (470). The stub is byte-identical in all 51 of these files and in all 970 files of `streamsounds/`. ffmpeg decodes it to 1,728 samples of digital silence (every sample is 0). The purpose of the stub is not known. One plausible reason is that it lets a player that probes for MP3 open the file, but this is unverified.

In 47 of the 51 embedded WAV files the audio is IMA ADPCM, a 4-bit compressed format, in stereo at 44,100 Hz. The other four contain uncompressed 16-bit PCM. Five files carry LIST or smpl metadata chunks after the audio. The chunks of every file end exactly at the end of the file.

## IMA ADPCM encoding

The ADPCM data follows the standard WAV variant of IMA ADPCM. The decoder is `kotor/adpcm.go`.

The audio is split into blocks of 2,048 bytes. Each block holds 2,041 samples per channel. A block begins with a 4-byte header for each channel: an s16 starting sample, a u8 step index, and a zero byte. The rest of the block alternates between channels in groups of 4 bytes, which hold 8 samples. Within each byte the low 4 bits come first. The "fact" chunk stores the total number of samples per channel.

The decoder was checked as follows. Each block header stores the sample the encoder expected next. In all 47 files the difference between that sample and the last decoded sample of the previous block is about the same as the difference between any two neighbouring samples. When the order of the 4-bit values is swapped as a test, this difference grows 8 to 14 times.

## Conversion and verification

The `music` stage of `cmd/extract` writes the 68 Layout A files to `extracted/streammusic/` as MP3 files, with the fake header removed. It writes the 51 Layout B files as WAV files with 16-bit PCM audio, with the stub removed and ADPCM decoded. ffprobe opens all 119 outputs without error. It reports all 51 WAV outputs as 16-bit stereo PCM, 50 at 44,100 Hz and one at 22,050 Hz.

# Sound effects and voices: streamsounds/\*.wav

The folder `streamsounds/` holds 970 files with the extension `.wav`. They are combat voice lines, creature sounds, ambient loops and a few music stingers that the game streams from disk instead of loading from the BIF archives. Every file uses Layout B from the music section: the same 470-byte MP3 stub, followed by a complete WAV file at offset 0x1D6 (470). No file uses Layout A, and none uses ADPCM. The converter `cmd/extract` (stage `sounds`) strips the stub and writes the embedded WAV unchanged to `extracted/streamsounds/`.

## Embedded WAV

The following table lists the fields of the embedded WAV file. Offsets count from the "RIFF" tag at file offset 0x1D6.

| Offset | Type    | Meaning                      | Evidence                                                               | Confidence |
| ------ | ------- | ---------------------------- | ---------------------------------------------------------------------- | ---------- |
| 0x00   | char[4] | "RIFF"                       | True in all 970 files.                                                 | high       |
| 0x04   | u32     | RIFF size: file size - 478   | The chunk walk ends exactly at the end of all 970 files.               | high       |
| 0x08   | char[4] | "WAVE"                       | True in all 970 files.                                                 | high       |
| 0x0C   | chunk   | "fmt " chunk, 16 or 18 bytes | 960 files use 16 bytes. 10 use 18 bytes, where the extra u16 is 0.     | high       |
| 0x14   | u16     | Format tag, always 1 (PCM)   | True in all 970 files.                                                 | high       |
| 0x16   | u16     | Channel count                | 952 files are mono. The 18 `mus_sbat_*` files are stereo.              | high       |
| 0x18   | u32     | Sample rate                  | 966 files use 22,050 Hz. The four `N_DARKJEDIM_*` files use 44,100 Hz. | high       |
| 0x1C   | u32     | Bytes per second             | Equals rate × block align in all 970 files.                            | high       |
| 0x20   | u16     | Block align: 2 × channels    | True in all 970 files.                                                 | high       |
| 0x22   | u16     | Bits per sample, always 16   | True in all 970 files.                                                 | high       |

The chunks after "fmt " vary. The table below counts each chunk sequence.

| Chunk sequence after "fmt " | Files | Notes                                                                                 |
| --------------------------- | ----- | ------------------------------------------------------------------------------------- |
| data                        | 932   | The plain case.                                                                       |
| fact, data                  | 10    | These are the files with an 18-byte "fmt " chunk. "fact" equals the sample count.     |
| data, LIST                  | 25    | LIST INFO metadata written by Sound Forge 6.0, with an engineer name and a 2003 date. |
| data, LIST, smpl            | 1     | `cs_sfgenloop.wav`. The smpl chunk declares no loops.                                 |
| data, smpl                  | 1     | `al_en_emptybar.wav`. The smpl chunk declares no loops.                               |
| data, cue, LIST             | 1     | `al_nt_fountsmal.wav`. One cue point at sample 0, labelled "Loop 1-2".                |

In all 10 files with a "fact" chunk, its value equals the data size divided by 2. Every data size is a whole number of blocks. The engine probably ignores the metadata chunks, because they occur in only 28 files, but this is unverified.

## File names

The names encode what the sound is for. The meanings of the prefixes are inferred from the names alone and are unverified.

| Prefix | Files | Probable meaning                                                               |
| ------ | ----- | ------------------------------------------------------------------------------ |
| P\_    | 419   | Party member voice sets, such as `P_CARTH_*` and `P_PLYERMW_*` (player, male). |
| N\_    | 256   | Non-player character voice sets, such as `N_BITH_*`.                           |
| c\_    | 181   | Creature and droid sounds, such as `c_drdastro_*`.                             |
| al\_   | 69    | Ambient loops, such as `al_nt_ocean`.                                          |
| mus\_  | 18    | Battle music stingers, all named `mus_sbat_<area>`.                            |
| pl\_   | 14    | Placeable loops, such as `pl_lrgfountain`.                                     |
| mgs\_  | 7     | Minigame sounds, such as `mgs_engine_01l` and `mgs_bike_idle`.                 |
| cs\_   | 5     | Cutscene sounds, such as `cs_duelcrowd`.                                       |
| as\_   | 1     | `as_el_alert`. The meaning of this prefix is unknown.                          |

The suffix names the sound set slot that plays the file. Examples are ATK1 (attack), BAT1 (battle cry), HIT1 (hit), DEAD (death), SLCT (selected), CRIT, POIS, LOW, STLH (stealth) and SRCH (search). The section "SSF sound set" derives the meaning of each slot from these suffixes. The prefixes appear in both upper and lower case. No two names collide when case is ignored.

## Verification

All 970 files pass the parser: the chunk walk of each embedded WAV ends exactly at the end of the file. ffprobe opens all 970 outputs. In a check that was run once, ffmpeg decoded every output without error, and each decode yielded exactly the sample count implied by its data chunk. The `sounds` stage of `cmd/verify` runs only ffprobe.

# Voice-over: streamwaves/\*\*/\*.wav

The folder `streamwaves/` holds the spoken dialog of the game: 13,799 files with the extension `.wav`, 552 MB in total. Every file uses Layout A from the music section: a 58-byte fake WAV header followed by an MP3 stream. No file uses Layout B. The converter `cmd/extract` (stage `waves`) strips the header and writes the MP3 stream unchanged to `extracted/streamwaves/`, keeping the folder structure.

## Fake header

The 58-byte header is byte-identical in all 13,799 files and in the 68 Layout A files of `streammusic/`. The following table lists its fields. It describes a WAV file with no audio, so a player that trusts it plays nothing.

| Offset | Type    | Meaning                                          | Evidence                                    | Confidence |
| ------ | ------- | ------------------------------------------------ | ------------------------------------------- | ---------- |
| 0x00   | char[4] | "RIFF"                                           | True in all 13,799 files.                   | high       |
| 0x04   | u32     | RIFF size, always 50. It covers the header only. | True in all 13,799 files.                   | high       |
| 0x08   | char[4] | "WAVE"                                           | True in all 13,799 files.                   | high       |
| 0x0C   | chunk   | "fmt " chunk of 18 bytes                         | True in all 13,799 files.                   | high       |
| 0x14   | u16     | Format tag 1 (PCM)                               | The claim is false: the audio is MP3.       | high       |
| 0x16   | u16     | Channel count 1                                  | True of the MP3 stream.                     | high       |
| 0x18   | u32     | Sample rate 22,050 Hz                            | The claim is false: the MP3 uses 32,000 Hz. | high       |
| 0x1C   | u32     | Bytes per second 22,050                          | Consistent with the false format claim.     | high       |
| 0x20   | u16     | Block align 1                                    | Consistent with the false format claim.     | high       |
| 0x22   | u16     | Bits per sample 8                                | Consistent with the false format claim.     | high       |
| 0x24   | u16     | Extra format size 0                              | True in all 13,799 files.                   | high       |
| 0x26   | chunk   | "fact" chunk of 4 bytes with value 0             | True in all 13,799 files.                   | high       |
| 0x32   | chunk   | "data" chunk header with size 0                  | True in all 13,799 files.                   | high       |
| 0x3A   | bytes   | MP3 stream, which runs to the end of the file    | The frame walk ends exactly at end of file. | high       |

## MP3 stream

Every frame of every file has the same header, 0xFFFB38C4 (ignoring the padding bit): MPEG-1 Layer III, 48 kbit/s, 32,000 Hz, mono, no CRC. A frame is therefore 216 bytes, or 217 when the padding bit is set, and holds 1,152 samples (36 ms). The 13,799 files hold 2,533,955 frames in total, about 25.2 hours of speech.

The first frame of every file is a LAME "Info" frame. It holds no audio and carries the encoder string "LAME3.93". Its frame count equals the number of frames after it in all 13,799 files. Its encoder delay is 576 samples in all files. In all 13,799 outputs, ffmpeg decodes exactly (frames - 1) × 1,152 - 576 - padding samples, so it applies the delay and padding values stored in this frame.

Three files begin the MP3 stream with a 256-byte ID3v2.3 tag and end it with an empty 128-byte ID3v1 tag: `m01aa/c05001/nm01aac05001000_.wav`, `m01aa/tras02/NM01AATRAS02000_.wav` and `m35aa/sith13/NM35AASITH13002_.wav`. The ID3v2 tag holds two PRIV frames named "PeakValue" and "AverageLevel". These names match the volume-levelling frames that Windows Media Player writes, but this is unverified. The converter keeps both tags.

## Folder structure and file names

The folder has 489 files at the top level and 81 subfolders. 77 subfolders are named after modules, such as `m24aa`. The others are `globe`, `stn12`, `stn14` and `stn57`. Each of these holds one subfolder per conversation, such as `m24aa/droi01`, and 568 such conversation folders exist. Some conversation names, such as `bast01`, occur under more than one module.

All 13,310 files in conversation folders are named N, then the module folder name, then the conversation folder name, then three digits, then one more character. In 13,297 files the last character is "\_". In 13 files it is "A" or "B", as in `NM11AAGADO02018A.wav` and `NM11AAGADO02018B.wav`, which appear to be two parts of one line. Dialogs name these files in the field VO_ResRef of their lines (see "Contents of the module files"). Of the 13,799 voice files, 12,700 are named by a VO_ResRef of a dialog in the modules or in `templates.bif`.

The module dialogs hold 34,673 voice references of this form. Each is 16 characters long, the length of a resref: a leading character, a module code of five characters, a conversation code of six characters, three digits, and a final character. The conversation code is four letters and two digits in 29,536 references. The table below describes the parts. The meanings come from the dialogs and are not confirmed in the game.

| Part              | Meaning                                            | Evidence                                                                                                                                                                                                  | Confidence |
| ----------------- | -------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| Leading character | "n" for a spoken line, "\_" for a player reply     | "n" in 17,811 of the 17,812 references of NPC lines (EntryList), and "\_" in all 16,861 references of player replies (ReplyList). No player reply has a file, because the player character is not voiced. | high       |
| Module code       | The module's `Mod_VO_ID`, such as `m24aa`          | Equals the `Mod_VO_ID` of the dialog's module in 30,923 references (see "Module and area names"). Shared lines use `globe`.                                                                               | medium     |
| Conversation code | A name for the conversation, such as `droi01`      | It is not the dialog resref: for example, `unk42_breed02.dlg` names `nm42aabree02001_`.                                                                                                                   | medium     |
| Three digits      | UNKNOWN                                            | They are not the index of the line in its list: they equal it in only 994 of the 34,673 references.                                                                                                       | low        |
| Final character   | "\_", or "A" and "B" for the parts of a split line | See above.                                                                                                                                                                                                | medium     |

Of the 13,799 voice files, 109 are named by no dialog, no entry of `dialog.tlk` and no string constant of a script. They include the eight cutscene lines in the table below, such as `BA02CS001.wav`, and 41 lines of the conversation `m01aa/tras02`. How the game finds them is UNKNOWN.

The following table classifies the 489 top-level files by name.

| Name pattern            | Files | Probable meaning                                                                                              |
| ----------------------- | ----- | ------------------------------------------------------------------------------------------------------------- |
| N_G\<speaker\>\_\<kind> | 421   | Generic lines of non-player characters, such as `N_GDUROS_COML1`. Names of this form occur in `dialog.tlk`.   |
| N_M\<number\>...        | 59    | Dialog lines that follow the conversation-folder scheme but lie at the top level, such as `N_M1ANLEVE98003_`. |
| BA, MA, MI              | 8     | Cutscene lines, such as `BA02CS001` and `MA18CS006A`.                                                         |
| \_m40acdart03999\_      | 1     | A conversation-scheme name with a leading underscore instead of N.                                            |

The prefixes appear in both upper and lower case. No two paths collide when case is ignored. The meanings in the table are inferred from the names and are unverified.

## Verification

All 13,799 files pass the parser in `kotor/audio.go`: the MP3 frame walk of each file ends exactly at the end of the file. ffprobe opens all 13,799 outputs without error. In a check that was run once, ffmpeg decoded every output without error, and each decode yielded the sample count given above. The `waves` stage of `cmd/verify` runs only ffprobe.

# Lip sync: lips/\*.mod

The folder `lips/` holds the mouth animation for the spoken dialog. Each LIP resource is a list of timed mouth shapes for one voice file in `streamwaves/`. The folder has 123 files, 5.7 MB in total. Each file is an ERF archive that holds only LIP resources (resource type 3004). The 123 archives hold 18,206 resources with 12,910 distinct names.

## Archives

The archives use the ERF layout described in the archives section, with the signature "MOD V1.0". All 123 archives pass the same checks as the texture packs. The localized string count and size are 0, both table offsets are 160, every resource ID equals its index, the u16 after the type is 0, the header padding is 0, and the resource data is stored in table order without gaps and ends exactly at the end of the file.

Two header fields differ from the texture packs. The build day is 309 in 110 archives and 145 in 13. The u32 at 0x28 takes 56 different values. One of them, 0x705F7773, occurs in three archives, and its bytes read "sw_p" as ASCII. The field is probably left uninitialized by the tool that wrote the archives, but this is unverified.

117 archive names are a module name followed by `_loc.mod`, such as `danm13_loc.mod`. A RIM archive of the same module name exists in `modules/` for each of them. The other six are `localization.mod`, `global.mod`, `legal.mod`, `mainmenu.mod`, `miniglobal.mod` and `subglobal.mod`. 14 archives are empty: they consist of the 160-byte header only. `localization.mod` is the largest, with 2,596 resources.

An archive holds the LIP resources of the lines spoken in its module. These are mostly lines of that module's conversations, plus shared lines such as generic NPC lines (`n_g*`) and `globe` conversations. For example, `danm14ad_loc.mod` holds 217 resources, all from conversations under `m14ad`. 1,647 names occur in more than one archive. All copies of each name are byte-identical.

## LIP resource

A LIP resource is a 16-byte header followed by keyframes. The following table lists the header fields.

| Offset | Type    | Meaning                     | Evidence                                                                                                      | Confidence |
| ------ | ------- | --------------------------- | ------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "LIP V1.0"        | True in all 18,206 resources.                                                                                 | high       |
| 0x08   | f32     | Length in seconds           | Equals the time of the last keyframe in all 18,206 resources, and is within 50 ms of the voice file duration. | high       |
| 0x0C   | u32     | Keyframe count _n_          | The resource size is exactly 16 + 5 × _n_ in all 18,206 resources.                                            | high       |
| 0x10   | 5 × _n_ | Keyframes, in order of time | See below.                                                                                                    | high       |

Each keyframe is 5 bytes and is not aligned.

| Offset | Type | Meaning              | Evidence                                                                                       | Confidence |
| ------ | ---- | -------------------- | ---------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | f32  | Time in seconds      | Nondecreasing in all 18,206 resources. In 227 of them two neighbouring keyframes share a time. | high       |
| 0x04   | u8   | Mouth shape, 0 to 15 | Only the values 0 to 15 occur. The first and last keyframe of every resource has shape 0.      | medium     |

The resources hold 946,460 keyframes, at most 182 in one resource. The longest resource lasts 25.96 s. The median density is 8.1 keyframes per second, and the range is 2.1 to 17.1.

The following table counts how often each mouth shape occurs.

| Shape | Keyframes | Shape | Keyframes |
| ----- | --------- | ----- | --------- |
| 0     | 120,392   | 8     | 37,681    |
| 1     | 47,407    | 9     | 33,569    |
| 2     | 216,369   | 10    | 90,696    |
| 3     | 29,451    | 11    | 76,339    |
| 4     | 63,227    | 12    | 42,160    |
| 5     | 30,094    | 13    | 19,444    |
| 6     | 73,524    | 14    | 16,609    |
| 7     | 30,595    | 15    | 18,903    |

The sound or mouth pose that each shape stands for is UNKNOWN. Shape 0 is probably the closed or resting mouth, because every resource begins and ends with it. The engine probably blends between neighbouring keyframes, but this is unverified.

## Link to the voice files

The name of every LIP resource equals the name of a voice file in `streamwaves/`, ignoring case and the `.wav` extension. All 18,206 resources have such a file. The LIP length is 13 to 49 ms shorter than the voice file's decoded duration (median 31 ms), which is roughly the length of one MP3 frame (36 ms). Of the 13,799 voice files, 889 have no LIP resource. 98 of them are top-level files, such as the cutscene line `BA02CS001.wav`, and 791 are conversation lines, such as `m01aa/tras02/NM01AATRAS02090_.wav`. Of the 889, 634 are named by the sound of an entry of `dialog.tlk`, such as the generic line `n_genbith_grt`, 199 by a dialog (three by both), and 59 by neither. Why these lines lack lip sync is unknown.

## Conversion and verification

No standard file format exists for lip sync keyframes, so each LIP resource is converted to a CSV (comma-separated values) file. The reader is `ParseLip` in `kotor/lip.go`, and the converter is the `lips` stage of `cmd/extract`. It writes `extracted/lips/<archive>/<resref>.csv`, with one folder per archive, named after the archive without `.mod`. The 14 empty archives produce no folder, so 109 folders are written. A name that occurs in several archives is written once per archive.

The following example shows the first lines of `extracted/lips/danm14ad_loc/nm14adc86901000_.csv`.

```
time_s,shape
0,0
0.183,5
0.292,8
```

The first line is always `time_s,shape`. Each further line is one keyframe: its time in seconds and its mouth shape. Each time is written as the shortest decimal number that reads back as the same f32 value. The header's length field is not written, because the parser requires it to equal the time of the last keyframe. The keyframe count is the number of lines after the first. The CSV file therefore holds every value of the resource.

The parser accepts a resource only if the keyframes end exactly at the end of the resource, every time is at least the previous one, every shape is below 16, and the length equals the last keyframe's time. All 18,206 resources pass.

The `lips` stage of `cmd/verify` reads every CSV file with Go's `encoding/csv` reader. The values it reads are packed again as f32 and u8, and the bytes must equal the keyframe bytes of the source resource. All 18,206 files pass. This reader shares no code with the converter, but it is not an external tool, so the output is not verified by an independent program. A test in which one shape value was altered in one CSV file made that file fail.

# Movies: movies/\*.bik

The game's 61 cutscenes and logo screens are in `movies/`. They are Bink video files, a format by RAD Game Tools that the game plays with `binkw32.dll`. All 61 files begin with the signature "BIKi", which `file` identifies as Bink Video revision i. The parser is `kotor/bink.go`, and the converter is the `movies` stage of `cmd/extract`. All 61 files pass the parser's check: the header size field matches the file size, the frame index ends exactly at the end of the file, the largest-frame field equals the largest frame, and every audio packet fits inside its frame.

The video and audio compression schemes are not documented here. The conversion uses ffmpeg's decoders, `binkvideo` and `binkaudio_dct`, which decode all 61 files.

## Header

The file begins with a 44-byte header. This table describes it.

| Offset | Type    | Meaning                            | Evidence                                                                                             | Confidence |
| ------ | ------- | ---------------------------------- | ---------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[4] | Signature "BIKi"                   | Present in all 61 files.                                                                             | high       |
| 0x04   | u32     | File size minus 8                  | Matches the file size in all 61 files.                                                               | high       |
| 0x08   | u32     | Number of frames (96 to 3,606)     | The frame index has this many entries plus one, and ffprobe counts this many frames in every output. | high       |
| 0x0C   | u32     | Size of the largest frame in bytes | Equals the largest frame in all 61 files.                                                            | high       |
| 0x10   | u32     | UNKNOWN                            | Equals the number of frames in all 61 files. Its purpose is not known.                               | low        |
| 0x14   | u32     | Width in pixels (640)              | Matches `file` and ffprobe in all 61 files.                                                          | high       |
| 0x18   | u32     | Height in pixels (272, 360 or 480) | Matches `file` and ffprobe in all 61 files.                                                          | high       |
| 0x1C   | u32     | Frame rate numerator (2,997)       | Frame count divided by this rate equals the audio duration in all 60 files with audio.               | high       |
| 0x20   | u32     | Frame rate denominator (100)       | As above. The rate is 29.97 frames per second.                                                       | high       |
| 0x24   | u32     | Video flags                        | The value is 0 in all 61 files. The meaning of individual bits is UNKNOWN.                           | low        |
| 0x28   | u32     | Number of audio tracks (0 or 1)    | `legal.bik` has 0 and ffprobe finds no audio in it. The other 60 files have 1.                       | high       |

Three arrays follow the header, each with one entry per audio track. Their offsets below assume one track, which is the case in 60 files. The frame index starts after them, at 44 + 12 × (number of tracks).

| Offset | Type | Meaning                                                    | Evidence                                                                         | Confidence |
| ------ | ---- | ---------------------------------------------------------- | -------------------------------------------------------------------------------- | ---------- |
| 0x2C   | u32  | Largest decoded audio packet in bytes (138,240 or 145,920) | Equals the largest decoded size declared by the track's packets in all 60 files. | high       |
| 0x30   | u16  | Sample rate in Hz (44,100, or 48,000 in `56b.bik`)         | Matches ffprobe in all 60 files.                                                 | high       |
| 0x32   | u16  | Audio flags (0x7000)                                       | See below.                                                                       | low        |
| 0x34   | u32  | Track ID (0)                                               | The value is 0 in all 60 files. That it is an ID is a hypothesis.                | low        |

The audio flags are 0x7000 in all 60 files. ffprobe reports every track as stereo and decodes it with the DCT (discrete cosine transform) variant of Bink audio. Each packet's decoded size matches 16-bit stereo samples, as described below. The value is therefore consistent with 0x1000 selecting DCT audio, 0x2000 stereo and 0x4000 16-bit samples. Every file has the same value, so the files cannot show which bit means what.

## Frame index

The frame index holds one u32 entry per frame plus one final entry. Bit 0 of an entry is set for a keyframe, and the remaining bits give the frame's offset from the start of the file. The final entry is the end of the last frame and equals the file size in all 61 files. Each frame ends where the next begins. The first frame begins immediately after the index in all 61 files.

Only the first frame of each file is marked as a keyframe. Every later frame therefore depends on earlier frames.

## Frame

A frame begins with one audio packet per track, followed by the video data.

| Offset | Type  | Meaning                                             | Evidence                                              | Confidence |
| ------ | ----- | --------------------------------------------------- | ----------------------------------------------------- | ---------- |
| 0x00   | u32   | Audio packet size in bytes, not counting this field | Every packet ends inside its frame in all 61 files.   | high       |
| 0x04   | u32   | Decoded size of the packet in bytes                 | Present when the packet size is 4 or more. See below. | high       |
| 0x08   | bytes | Compressed audio                                    | ffmpeg decodes it. The layout is not documented here. | UNKNOWN    |
| varies | bytes | Compressed video, to the end of the frame           | ffmpeg decodes it. The layout is not documented here. | UNKNOWN    |

The decoded sizes of a track's packets add up to the length of the movie. Dividing the sum by 4 bytes per stereo sample and by the sample rate gives the frame count divided by the frame rate to within 0.0001 seconds, in the 4 files checked by hand: `05_9.bik`, `01a.bik`, `56b.bik` and `07_2.bik`. The verification repeats this check for all 60 files with audio, as described below. ffmpeg's decoder produces about 0.03 seconds more audio than the packets declare. The cause is not known; it may be padding in the last audio block.

## Conversion and verification

The `movies` stage of `cmd/extract` uses ffmpeg to convert each movie to an MP4 file in `extracted/movies/`, with H.264 video and AAC audio. The video uses a constant quality setting (CRF 12), because at CRF 18 frames in 12 movies fell below 40 dB PSNR (peak signal-to-noise ratio, a measure of how closely the output matches the source).

`cmd/verify` checks all 61 outputs with ffprobe and ffmpeg. For each movie, ffprobe must report the frame size, the frame count, the sample rate, the channel count and the audio duration of the Bink file, with the duration within 0.1 seconds. Every output frame must also reach at least 40 dB PSNR against the decoded Bink frame. All 61 outputs pass. The lowest PSNR of any frame is 41.8 dB.

# Saved games: saves/\*/

A saved game is a folder in `saves/` that holds the complete state of a game in progress: the global script variables, the party, the player's inventory and the state of every visited module. The game files contain one saved game, `saves/000002 - Game1/`, made on the first module of the game (`end_m01aa`, "Endar Spire - Command Module") after 1,163 seconds of play. The file `saves/steam_autocloud.vdf` belongs to the Steam client, not to the game, and is not examined.

Only one saved game exists, so most findings below rest on a single sample. Where possible, they are checked against other files: the original module in `modules/`, the table `globalcat.2da`, and fields of other save files that must agree.

Every file except the screenshot is either a GFF file or an ERF archive of GFF files. The readers are `ReadGff` in `kotor/gff.go`, `ReadErf` and `ErfEntries` in `kotor/kotor.go`, and `ReadGlobals` in `kotor/save.go`. All nine GFF files of the saved game parse under the strict rules of `ReadGff`: every field and struct is used once, every byte of the three data regions is used exactly once, and the regions end exactly at the end of the file.

## Files of a saved game

The table below lists the files of the saved game.

| File             | Format                  | Size in bytes | Content                                                        |
| ---------------- | ----------------------- | ------------- | -------------------------------------------------------------- |
| `savenfo.res`    | GFF, file type "NFO "   | 681           | Summary shown in the load menu                                 |
| `GLOBALVARS.res` | GFF, file type "GVT "   | 57,656        | Global script variables                                        |
| `PARTYTABLE.res` | GFF, file type "PT "    | 11,209        | Party, journal, messages and other state shared across modules |
| `SAVEGAME.sav`   | ERF archive, "MOD V1.0" | 585,947       | Party member templates, inventory, factions and saved modules  |
| `Screen.tga`     | TGA, 256x256, 24-bit    | 196,626       | Screenshot shown in the load menu                              |

All four GFF file types have the version "V3.2". The extension ".res" therefore marks a GFF file of any file type.

## Saved game archive: SAVEGAME.sav

`SAVEGAME.sav` is an ERF archive with the signature "MOD V1.0". Its header and tables follow the ERF layout described above: no localized strings, the key table at 160, the resource table directly after it, and the field at 0x28 equal to 0. The resources start directly after the resource table, lie in table order without gaps, and end exactly at the end of the file. The archive holds four resources, listed in the table below.

| Resref      | Type | Extension | Content                                         |
| ----------- | ---- | --------- | ----------------------------------------------- |
| `AVAILNPC0` | 2027 | utc       | Creature template of party member slot 0        |
| `END_M01AA` | 2057 | sav       | Nested "MOD V1.0" archive: the saved module     |
| `INVENTORY` | 0    | res       | GFF file of file type "INV ": the party's items |
| `REPUTE`    | 2038 | fac       | Faction table                                   |

Resource types 0 and 2057 occur nowhere else in the game files. Their extensions come from the pool of extension strings in the `.rdata` section of `swkotor.exe`, which is not encrypted. The pool lists the extensions roughly in descending type order: "sav" stands between "utw" (2058) and "jrl" (2056), and "res" stands last, after "bmp", "mve" and "tga" (types 1 to 3). The confidence of both names is medium. No table that pairs each code with its string was found in the file, because the `.text` section is encrypted by the Steam wrapper (every 64 KB block of it compresses to more than its own size).

The resrefs in the outer archive are uppercase, and those in the nested archive are mixed: `m01aa` and `Module`. The converter writes all resrefs in lowercase, as for the other archives.

### Saved module

The nested archive `END_M01AA` (type 2057) holds one module in the same layout as the outer archive. Its header holds the same date. It holds the three resources of the module's main archive, in the order `m01aa.git`, `m01aa.are` and `module.ifo`. The table below compares them with the same resources in `modules/end_m01aa.rim`.

| Resource     | Original size | Saved size | Original structs | Saved structs | Difference                                                          |
| ------------ | ------------- | ---------- | ---------------- | ------------- | ------------------------------------------------------------------- |
| `m01aa.are`  | 3,621         | 3,621      | 17               | 17            | None: the two files are identical byte for byte.                    |
| `m01aa.git`  | 50,049        | 509,329    | 322              | 7,046         | Each instance holds its full state instead of a template reference. |
| `module.ifo` | 1,623         | 30,923     | 2                | 346           | Adds the game clock, the event queue and the player character.      |

In the original GIT file, `UseTemplates` is 1 and a creature instance holds only its position, orientation and `TemplateResRef`. In the saved GIT file, `UseTemplates` is absent, and each creature instance holds its whole creature record, including `ObjectId`, hit points, classes, skills, effects and equipment. The saved file also adds `VarTable`, `SWVarTable`, `AreaEffectList` and weather and transition fields at the root. The creature list shrinks from 26 to 25 instances: Trask (`end_trask`) is no longer in the area, because Trask has joined the party (see `AVAILNPC0` below).

In the saved IFO file, `Mod_IsSaveGame` is 1 instead of 0, and `Mod_PlayerList` holds the player character, whose first name is a literal substring. `Mod_ID` grows from 16 to 32 bytes. The first 16 bytes equal the original, and the other 16 read "Bad StrRef", a NUL byte and "n.wav". These bytes are probably uninitialized memory.

The saved IFO file holds the game clock in two forms that agree with each other.

- `Mod_PauseDay` is 450,382. The date fields give year 1340, month 6 and day 3. With 12 months of 28 days, 1340 × 336 + 5 × 28 + 2 = 450,382. The day count is therefore the number of days since year 0, month 1, day 1.
- `Mod_PauseTime` is 852,106. With `Mod_MinPerHour` 2, one game hour lasts 120,000 milliseconds. 7 hours × 120,000 + 12 seconds × 1,000 + 106 milliseconds = 852,106, which matches `Mod_StartHour` 7, `Mod_StartMinute` 0, `Mod_StartSecond` 12 and `Mod_StartMiliSec` 106. The time is therefore in real milliseconds since the start of the game day. How `Mod_StartMinute` advances is UNKNOWN, because it is 0.

The journal entry in `PARTYTABLE.res` uses the same clock: `JNL_Date` is 450,382 and `JNL_Time` is 843,530.

### Other resources

`AVAILNPC0` is a creature template (UTC) of the party member in slot 0. Its tag is `end_trask` and its conversation is `end_trask01`. Only slot 0 has a template, and only the first of the nine entries of `PT_AVAIL_NPCS` in `PARTYTABLE.res` has `PT_NPC_AVAIL` 1. The digit in the resref is therefore probably the slot number.

`INVENTORY` (type 0, file type "INV ") holds one list, `ItemList`, with four items, such as a short sword and two medical kits (stack size 2).

`REPUTE` is a faction table (FAC). It lists 21 factions in `FactionList`, from "Player" to "Sacrifice", and 405 relationships in `RepList`. The faction tables in the module support archives list 5 factions and 20 or 0 relationships. The rule that decides which of the 441 faction pairs are stored is UNKNOWN.

## GFF field types in saved games

The saved game is the only source of two GFF field types: type 1, a signed 8-bit integer, and type 6, a 64-bit integer stored in the field data. Its negative values of type 3 show that type 3 is signed. All three types are described in the field type table under "Generic file format: GFF".

## Global variables: GLOBALVARS.res

`GLOBALVARS.res` stores the values of the global script variables, which scripts read and write with routines such as `GetGlobalBoolean` and `SetGlobalNumber`. The table `globalcat.2da` declares them: 809 booleans, 369 numbers, 5 locations and 2 strings. The file holds exactly these names, with the same counts. The names are in the same order as in `globalcat.2da` only for the strings; the order of the others is UNKNOWN.

The root struct holds eight fields. Four lists hold the names, and four fields hold the values.

| Label         | Type        | Content                                                                         | Evidence                                                                                               | Confidence |
| ------------- | ----------- | ------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ | ---------- |
| `CatBoolean`  | list (15)   | One struct per boolean with a string field `Name`                               | 809 items, the same names as the booleans of `globalcat.2da`.                                          | high       |
| `CatNumber`   | list (15)   | One struct per number with a string field `Name`                                | 369 items, the same names as the numbers of `globalcat.2da`.                                           | high       |
| `CatLocation` | list (15)   | One struct per location with a string field `Name`                              | 5 items, the same names as the locations of `globalcat.2da`.                                           | high       |
| `CatString`   | list (15)   | One struct per string with a string field `Name`                                | 2 items, the same names as the strings of `globalcat.2da`.                                             | high       |
| `ValBoolean`  | binary (13) | One bit per boolean, in the order of `CatBoolean`                               | 102 bytes, which is 809 bits rounded up to whole bytes. Every byte is 0.                               | medium     |
| `ValNumber`   | binary (13) | One signed byte per number, in the order of `CatNumber`                         | 369 bytes. `nwscript.nss` limits global numbers to -128 to +127. The values are plausible (see below). | high       |
| `ValLocation` | binary (13) | The location values                                                             | 2,400 bytes, all 0. The layout is UNKNOWN.                                                             | low        |
| `ValString`   | list (15)   | One struct per string with a string field `String`, in the order of `CatString` | 2 items, both empty.                                                                                   | medium     |

Four numbers are not 0. `K_CURRENT_PLANET` is 5, `END_TRASK_DLG` is 11, and `END_CARTH_DLG` and `End_TraskTalk` are 1. All four belong to the Endar Spire, where the game was saved. This supports one byte per number in the order of `CatNumber`.

The order of the bits within a byte of `ValBoolean` is UNKNOWN. Every boolean is false, so this save cannot tell the two orders apart, and the engine code that packs them is encrypted. `ReadGlobals` takes the high bit of each byte first and rejects a set bit beyond the last boolean. A save in which a known boolean is true would settle the order.

The layout of `ValLocation` is UNKNOWN. A location in NWScript holds an area, a position and an orientation. The 2,400 bytes are 480 bytes per location, or 100 entries of 24 bytes (six f32 values), but no reading can be tested while every byte is 0.

## Party table: PARTYTABLE.res

`PARTYTABLE.res` (file type "PT ") holds the state that is shared across modules. The table below lists its root fields. The meanings follow from the labels and the values; those marked with an asterisk are confirmed by another file.

| Label                          | Type      | Value in the save | Probable meaning                                                                            |
| ------------------------------ | --------- | ----------------- | ------------------------------------------------------------------------------------------- |
| `PT_GOLD`                      | u32       | 0                 | Credits                                                                                     |
| `PT_XP_POOL`                   | s32       | 150               | Experience points                                                                           |
| `PT_PLAYEDSECONDS`             | u32       | 1,163             | Time played in seconds\*. Equals `TIMEPLAYED` in `savenfo.res`.                             |
| `PT_CONTROLLED_NP`             | s32       | -1                | Party member under the player's control; -1 for the player character                        |
| `PT_SOLOMODE`, `PT_CHEAT_USED` | u8        | 0                 | Flags                                                                                       |
| `PT_NUM_MEMBERS`, `PT_MEMBERS` | u8, list  | 1                 | Party members besides the player\*. One member with `PT_MEMBER_ID` 0, matching `AVAILNPC0`. |
| `PT_AVAIL_NPCS`                | list      | 9 items           | One entry per party member slot with `PT_NPC_AVAIL` and `PT_NPC_SELECT`                     |
| `PT_AISTATE`, `PT_FOLLOWSTATE` | s32       | 0                 | Party behaviour settings                                                                    |
| `GlxyMap`                      | struct    |                   | Galaxy map: 16 points, a planet mask of 0 and no selected point                             |
| `PT_PAZAAKCARDS`               | list      | 19 items          | Count per pazaak card                                                                       |
| `PT_PAZSIDELIST`               | list      | 10 items          | Pazaak side deck, -1 for an empty slot                                                      |
| `PT_TUT_WND_SHOWN`             | binary    | 6 bytes           | Tutorial windows already shown, probably one bit each. The layout is UNKNOWN.               |
| `PT_LAST_GUI_PNL`              | s32       | 7                 | Last menu panel opened                                                                      |
| `PT_FB_MSG_LIST`               | list      | 10 items          | Feedback messages, with text, type and colour                                               |
| `PT_DLG_MSG_LIST`              | list      | 34 items          | Dialog history, with speaker and text                                                       |
| `PT_COST_MULT_LIS`             | list      | 92 items          | Price multipliers, all 1.0                                                                  |
| `JNL_SortOrder`, `JNL_Entries` | s32, list | 1 entry           | Journal: plot `end_attack` at state 20, with the date and time of the entry\*               |

## Save summary: savenfo.res

`savenfo.res` (file type "NFO ") is a single struct with 16 fields. `AREANAME` is the area's display name, `LASTMODULE` is the module resref in uppercase, `TIMEPLAYED` is the time played in seconds, and `SAVEGAMENAME` is the name typed by the player. `PORTRAIT0` and `PORTRAIT1` are the portraits of the party (`po_pmhc4` for the player and `po_ptrask` for Trask). `CHEATUSED`, `GAMEPLAYHINT` and `STORYHINT` are bytes. The six strings `LIVE1` to `LIVE6` and the byte `LIVECONTENT` are empty or 0; they probably refer to downloadable content of the Xbox version, but this has not been confirmed.

## Screenshot: Screen.tga

`Screen.tga` is an uncompressed TGA image (image type 2) of 256x256 pixels with 24 bits per pixel, no alpha channel and the origin at the bottom left. It has no image ID, colour map or trailer: its size is exactly the 18-byte header plus 196,608 bytes of pixels. The existing TGA reader decodes it.

## Conversion and verification

The `saves` stage of `cmd/extract` converts every saved game folder in `saves/` to `extracted/saves/<save>/`. It writes the following files.

- Each GFF file becomes `<name>.res.json` in the JSON form described under "Generic file format: GFF". 64-bit integers are written exactly.
- `GLOBALVARS.res` also becomes `GLOBALVARS.variables.json`, which pairs each variable name with its value: `booleans` (JSON booleans), `numbers` (integers), `strings`, `locations` (the names only) and `location_data` (`ValLocation` as hexadecimal, because its layout is unknown).
- `Screen.tga` becomes `Screen.png`, a 24-bit PNG with the top row first.
- `SAVEGAME.sav` becomes the folder `SAVEGAME/`. Each GFF resource becomes `<resref>.<ext>.json`, and the saved module becomes the subfolder `SAVEGAME/end_m01aa/`.

The stage writes 13 report lines to `temp/reports/saves.txt`, and all 13 pass: both archives have the expected layout, all nine GFF files parse, the variables decode, and the screenshot decodes.

The `saves` stage of `cmd/verify` checks the outputs with independent tools. jq parses all nine GFF JSON files. ImageMagick's decoding of `Screen.png` equals its decoding of `Screen.tga`, and pngcheck accepts the PNG. The decoded variables, read with jq, are packed again into `ValBoolean` and `ValNumber`, and these bytes, the names and the strings must equal the fields of the source file. All four checks pass. A test in which one number in `GLOBALVARS.variables.json` was changed made the last check fail. This check confirms only that the JSON file holds every bit of the source; it cannot confirm the bit order of `ValBoolean`, because the same order is used to pack the bits again.
