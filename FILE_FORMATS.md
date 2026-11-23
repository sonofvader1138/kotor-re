# File formats of Star Wars: Knights of the Old Republic

This document describes the game's file formats as determined from the files in `swkotor`. Each field lists the evidence for it and a confidence level. Fields that have not been verified are marked UNKNOWN.

All integers and floats are little-endian. Offsets are in hexadecimal and count from the start of the structure they belong to, unless a section states otherwise.

# Archives: chitin.key, data/\*.bif and ERF

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

The archives contain 29 resource types. Six are described in this document: 2002 is an MDL model, 3008 is its MDX vertex data, 2047 is a GUI layout, 3 is a TGA image, 2022 is a TXI text file of texture options, and 3001 is a text file that probably lists visible rooms. Type 3007 (TPC texture) does not occur in the BIF archives; it occurs in the ERF texture packs. The number of resources of each type is listed below.

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

Many of the other types begin with a signature of four characters followed by a version. The table below lists the signatures found at the start of the resources of each type. The formats behind these signatures have not been examined, except the "V3.2" container, which is described in the GUI section.

| Signature | Types and counts                                                                                                                                                                                               |
| --------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| "V3.2"    | 2015 BIC (1), 2024 BTI (1), 2025 UTI (557), 2026 BTC (3), 2027 UTC (205), 2029 DLG (32), 2030 ITP (16), 2032 UTT (21), 2040 UTE (65), 2042 UTD (50), 2044 UTP (317), 2047 GUI (84), 2056 JRL (1), 2058 UTW (9) |
| BWM V1.0  | 2016 (1,202), 2052 (156), 2053 (196)                                                                                                                                                                           |
| NCS V1.0  | 2010 (1,784)                                                                                                                                                                                                   |
| SSF V1.1  | 2060 (106)                                                                                                                                                                                                     |
| LTR V1.0  | 2036 (3)                                                                                                                                                                                                       |

In the "V3.2" row, the four characters before the version differ by type and are given beside each type number. Every resource of these types carries its signature. Types 4, 2009, 2017 and 3000 carry no such signature, and their formats are UNKNOWN.

## ERF archive

An ERF archive is a self-contained archive: unlike a BIF archive, it stores the name of each resource itself. The four texture packs in `TexturePacks/` are ERF archives. The reader is `ReadErf` in `kotor/kotor.go`. `patch.erf` is also an ERF archive, and `ReadErf` reads its tables without error. It holds 97 resources: 84 TPC textures, 7 GUI layouts, 4 resources of type 2017, and the MDL and MDX resources of one model. Only its GUI resources have been examined. The 123 files in `lips/` are ERF archives with the signature "MOD V1.0" instead of "ERF V1.0" (see the lip sync section). The files in `modules/` and `rims/` are not ERF archives (see the RIM section below).

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

The build date fields agree between the KEY file and the archives built with it, and `patch.erf`, which was presumably built later, holds a later date. This supports the reading as a date, but it has not been confirmed.

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

The tpb and tpc packs are likely the lower texture quality settings. Only tpa and gui are converted, because tpb and tpc hold no additional images.

## RIM archive

A RIM archive is a self-contained archive like ERF, with a simpler header and a single table. All 246 files in `modules/` and `rims/` begin with "RIM V1.0". They hold 22,511 resources in total. The layout below was established only to search them for GUI resources. There is no reader in `kotor/kotor.go` yet. Apart from their signatures (see below), the resources have not been examined.

A RIM archive begins with a 120-byte header.

| Offset | Type    | Meaning                      | Evidence                                                                                                 | Confidence |
| ------ | ------- | ---------------------------- | -------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | char[8] | Signature "RIM V1.0"         | Present in all 246 files.                                                                                | high       |
| 0x08   | u32     | UNKNOWN, always 0            | The value is 0 in all 246 files.                                                                         | low        |
| 0x0C   | u32     | Number of resources          | The table parses consistently in all 246 files. Four files have 0 resources.                             | high       |
| 0x10   | u32     | Offset of the resource table | The value is 120 in all 246 files.                                                                       | high       |
| 0x14   | u32     | UNKNOWN flag                 | The value is 1 in the six files whose names end in "dx", such as `globaldx.rim`, and 0 in the other 240. | low        |
| 0x18   | u8[96]  | Padding                      | All bytes are 0 in all 246 files.                                                                        | medium     |

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
| 4    | 375    | none      | 2035 | 635   | UTS V3.2            |
| 2002 | 239    | none      | 2038 | 20    | FAC V3.2            |
| 2010 | 11,355 | NCS V1.0  | 2040 | 68    | UTE V3.2            |
| 2012 | 117    | ARE V3.2  | 2042 | 525   | UTD V3.2            |
| 2014 | 117    | IFO V3.2  | 2044 | 1,467 | UTP V3.2            |
| 2017 | 306    | none      | 2051 | 38    | UTM V3.2            |
| 2023 | 117    | GIT V3.2  | 2056 | 3     | JRL V3.2            |
| 2025 | 498    | UTI V3.2  | 2058 | 2,049 | UTW V3.2            |
| 2027 | 1,753  | UTC V3.2  | 2060 | 210   | SSF V1.1            |
| 2029 | 1,135  | DLG V3.2  | 3003 | 132   | PTH V3.2            |
| 2032 | 1,100  | UTT V3.2  | 3008 | 252   | none, 153 are empty |

Types 2012, 2014 and 2023 each occur 117 times, which equals the number of `_loc.mod` archives in `lips/` and suggests one area description per module. This has not been verified.

# 3D models: MDL and MDX

Each 3D model is stored as two resources with the same resref. The MDL resource (type 2002) holds the model's structure: a tree of nodes, some of which carry meshes. The MDX resource (type 3008) holds the vertex data for those meshes. The converter is `cmd/mdl2obj`. It writes Wavefront OBJ files.

The models examined here are in `data/models.bif`, which contains 2,445 MDL resources and 2,445 MDX resources, including area room models such as `m01aa_01a`. Of the converted models, 1,329 have names of that form. The BIF archives hold 387 more models, which have not been examined or converted: 305 in `items.bif`, 63 in `player.bif` and 19 in `party.bif`. The RIM archives hold another 239 MDL and 252 MDX resources, which have not been examined either. `patch.erf` holds one model, `w_lghtsbr_008`, which has not been examined. The MOD archives in `lips/` hold only LIP resources, and the texture packs hold no models.

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

| Offset | Type     | Meaning                                             | Evidence                                                                                                 | Confidence |
| ------ | -------- | --------------------------------------------------- | -------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u32      | Constant 0x00413670                                 | Identical in all 2,445 models. It is likely an address in the game executable. Its purpose is not known. | medium     |
| 0x04   | u32      | Constant 0x00405520                                 | Identical in all 2,445 models. It is likely an address in the game executable. Its purpose is not known. | medium     |
| 0x08   | char[32] | Model name                                          | Equals the resref in 2,444 of 2,445 models.                                                              | high       |
| 0x28   | u32      | Offset of the root node                             | The node tree parses from this offset in all 2,445 models.                                               | high       |
| 0x2C   | u32      | Number of nodes                                     | Equals the number of nodes in the tree in 2,187 of 2,445 models. See below.                              | medium     |
| 0x68   | f32[6]   | Bounding box: minimum x, y, z, then maximum x, y, z | Plausible values in one model (plc_sithflag).                                                            | low        |
| 0x88   | char[32] | Name of the supermodel, or "NULL"                   | Contains model names such as S_Female02.                                                                 | high       |
| 0xA8   | u32      | Offset of the root node (repeated)                  | Seen in one model (plc_sithflag).                                                                        | low        |
| 0xB0   | u32      | Size of the MDX file (repeated)                     | Seen in one model (plc_sithflag).                                                                        | low        |
| 0xB8   | u32      | Offset of the name table                            | All node names resolve in all 2,445 models.                                                              | high       |
| 0xC0   | u32      | Number of entries in the name table                 | All node names resolve in all 2,445 models.                                                              | high       |

A supermodel is another model whose nodes this model appears to inherit. In 258 models the node count does not match the number of nodes in the tree. Those models likely count the nodes of their supermodel as well. This has not been verified.

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

Bits 0x02, 0x04 and 0x10 never occur together with 0x20, so these nodes carry no mesh. Bits 0x100 and 0x200 occur only together with 0x20. The meanings of bits 0x02, 0x04, 0x10, 0x100 and 0x200 are UNKNOWN. The converter treats all nodes with 0x20 alike, and it does not read any additional data that the other bits may announce.

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

An MDX file has no header. It is an array of vertex records, each built from f32 values. The mesh header in the MDL file gives the record size and the position of each component within a record. Some MDX files are empty, for example those of v_polymorf_imp and m12ah_mgt04. Those models appear to have no meshes.

## Walkmeshes in models.bif

The archive also contains resources of types 2016 (1,202), 2052 (156) and 2053 (196). All 1,554 begin with the signature "BWM V1.0". They are probably walkmeshes, the surfaces that define where characters can walk, but their format has not been examined. An empty MDX resource has the offset of the resource that follows it. For 32 of the 368 empty MDX resources, such as that of `m03mg_01b`, the next resource is a walkmesh, so a reader that looks at the bytes at that offset finds "BWM V1.0".

# Textures: TPC

A TPC file (resource type 3007) is a texture. It holds pixel data for one image, or for several sub-images, each with its own mipmap chain. A mipmap chain is a series of copies of an image, each half the width and height of the previous one. The file ends with an optional TXI trailer. TXI (texture information) is plain ASCII text that sets rendering options. The converter is `cmd/tpc2png`. It writes the top mipmap level as an 8-bit RGBA PNG.

All 11,452 TPC files in the four texture packs parse completely. In each file, the header, the pixel data and an ASCII trailer together account for every byte. The converted PNG files are verified with two independent tools. ImageMagick decodes each PNG to exactly the pixels of the decoded TPC texture, and pngcheck accepts each file. This holds for all 3,294 textures of tpa and all 1,570 textures of gui.

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

## Conversion to model materials

Each converted model has an MTL (material library) file beside its OBJ file. The MTL file defines one material per texture name, and each material points at the PNG in `assets/TexturePacks/swpc_tex_tpa/`, or in `swpc_tex_gui/` if the name is only found there. Texture names are compared without regard to case. Of the 1,401 distinct texture names that rendered meshes refer to, 1,364 are found. The converter lists the remaining 37 in its summary and marks them in the MTL files. Eight of the 37 are resources of other types in the BIF archives. Five, `m17mg_01c_a0001l` to `m17mg_01c_a0001p`, are TGA images in `lightmaps6.bif`, which are converted to `assets/data/lightmaps/` but are not linked from the MTL files. Two, `load_sw` and `n_repsold`, are model names. One, `h_f_lo01headtest`, exists only as a TXI file in `textures.bif`. The other 29 names occur in no archive. Of all 37, only `n_repsold` also occurs in a RIM archive.

The PNG files keep their alpha channel, but the MTL files do not declare it as transparency. In KotOR the alpha channel of some textures appears to hold other data, such as an environment map mask, rather than transparency. This has not been verified.

# Images: TGA in the BIF archives

Resource type 3 is a TGA image, the uncompressed image format defined by Truevision. The BIF archives hold 5,601 of them. They matter because 5,586 of them are lightmaps, the precomputed lighting that the game blends over area textures. The converter is `kotor/tga.go`, called by the `tga` stage of `cmd/convert`. It writes 8-bit PNG files with the same channels as the TGA.

The images are in three groups. The table below lists each group, its archives and its output directory.

| Archives                                               | Images | Layout                            | Output                   |
| ------------------------------------------------------ | ------ | --------------------------------- | ------------------------ |
| `lightmaps.bif`, `lightmaps2.bif` to `lightmaps13.bif` | 5,586  | 32-bit BGRA, sizes 2x2 to 512x512 | `assets/data/lightmaps/` |
| `textures.bif`                                         | 14     | 8-bit grayscale, 256x256          | `assets/data/textures/`  |
| `legacy.bif`                                           | 1      | 24-bit BGR, 256x256 (`chrome1`)   | `assets/data/legacy/`    |

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

The lightmap archives also hold 112 resources of type 3001, one per area, named after the area, such as `m01aa`. They are ASCII text with CRLF line breaks. An unindented line holds a room name and a count, and indented lines naming other rooms follow it. In 107 of the 112 files, the number of indented lines always equals the count. The other 5, such as `m38ab`, contain a count that does not match or a doubled space. The text may be a room visibility list, but this has not been verified. The converter skips them.

# User interface layouts: GUI

A GUI file (resource type 2047) describes one screen or panel of the user interface. It lists the screen's controls, such as buttons, labels and list boxes, with their position, size, textures, font and text. The game contains 91 GUI resources. 84 are in `data/gui.bif`, which holds nothing else, and 7 are in `patch.erf`. No other archive holds one: `chitin.key` lists type 2047 only for `gui.bif`, the 246 RIM files hold none, and the signature "GUI V3.2" occurs in no other file. The reader is `ReadGff` in `kotor/gff.go`, and the converter is the `gui` stage of `cmd/convert`. It writes one JSON file per resource, to `assets/data/gui/` for `gui.bif` and to `assets/patch/` for `patch.erf`.

## Resources in patch.erf

Five of the seven `patch.erf` resources have the same name as a `gui.bif` resource but different content. These are `abilities`, `character`, `equip`, `inventory` and `mainmenu`. The game probably uses the `patch.erf` version, because the archive's name suggests an update, but this has not been confirmed. The other two, `mipc212x10` and `tooltip12x10`, exist only in `patch.erf`. Both are 1280x1024 panels, a resolution that no `gui.bif` file has.

The changes are small. `abilities`, `character`, `equip` and `inventory` each gain two buttons, BTN_CHARLEFT and BTN_CHARRIGHT, which are 28x28 pixels at the same positions in all four files. The names suggest buttons that step between party members. The patched `mainmenu` has the same 13 controls, with BTN_WARP and BTN_EXIT in swapped order, and its panel is 800x600 instead of 640x480.

The file is a generic tree container. The layout data is not fixed; each value is a field that carries its own type and its own label. The first four bytes of the file name the content type ("GUI "). This suggests that other resource types use the same container with a different content type. That has not been examined.

The parser passes on all 91 files. In every file the six regions chain from offset 56 and the last one ends exactly at the end of the file. Every byte of the field data, field index and list index regions is used exactly once. Every field is used exactly once, and every struct except the root is referenced exactly once. Every label is referenced.

The JSON output is verified with jq, which parses all 91 JSON files without error. This shows only that the files are valid JSON. No independent check currently compares the JSON values with the binary fields.

The JSON does not record the binary field type. Types 0, 4 and 5 all become numbers, and types 10 and 11 both become strings. The type can still be recovered, because in all 91 files each of the 55 combinations of label and JSON value kind occurs with only one binary type. Strings are decoded as Latin-1. The only byte above 0x7F is 0xB1 in six strings, which is "±" in both Latin-1 and Windows-1252, so the choice of code page does not change the output.

The counts in the sections below were measured on the 84 `gui.bif` files and do not include `patch.erf`. The seven `patch.erf` files use the same header, struct, field and label layout, and they parse with the same strict checks.

## Container header

A GUI file begins with a 56-byte header. After the signature, it consists of six pairs of u32 values, and each pair locates one region of the file.

| Offset | Type    | Meaning                                 | Evidence                                                        | Confidence |
| ------ | ------- | --------------------------------------- | --------------------------------------------------------------- | ---------- |
| 0x00   | char[4] | Content type, "GUI "                    | Present in all 84 files.                                        | high       |
| 0x04   | char[4] | Version, "V3.2"                         | Present in all 84 files.                                        | high       |
| 0x08   | u32     | Offset of the struct table (always 56)  | The value is 56 in all 84 files.                                | high       |
| 0x0C   | u32     | Number of structs                       | The next region starts at this offset plus 12 bytes per struct. | high       |
| 0x10   | u32     | Offset of the field table               | Equals the struct table offset plus 12 times the struct count.  | high       |
| 0x14   | u32     | Number of fields                        | The next region starts at this offset plus 12 bytes per field.  | high       |
| 0x18   | u32     | Offset of the label table               | Equals the field table offset plus 12 times the field count.    | high       |
| 0x1C   | u32     | Number of labels                        | The next region starts at this offset plus 16 bytes per label.  | high       |
| 0x20   | u32     | Offset of the field data region         | Equals the label table offset plus 16 times the label count.    | high       |
| 0x24   | u32     | Size of the field data region in bytes  | The next region starts at its end.                              | high       |
| 0x28   | u32     | Offset of the field index region        | Directly follows the field data region.                         | high       |
| 0x2C   | u32     | Size of the field index region in bytes | Equals 4 times the field count in all 84 files.                 | high       |
| 0x30   | u32     | Offset of the list index region         | Directly follows the field index region.                        | high       |
| 0x34   | u32     | Size of the list index region in bytes  | The region ends exactly at the end of all 84 files.             | high       |

## Struct, field and label tables

A struct is a group of fields. Struct 0 is the root of the tree. Each entry of the struct table is 12 bytes.

| Offset | Type | Meaning                                 | Evidence                                                                                                                      | Confidence |
| ------ | ---- | --------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------- | ---------- |
| 0x00   | u32  | Struct type                             | 0xFFFFFFFF on all 84 roots, 14 on 17 structs in `debug.gui`, and 0 on all 10,950 others. The meaning of the value is UNKNOWN. | low        |
| 0x04   | u32  | Byte offset into the field index region | The region holds the struct's field indices at this offset, and all structs together cover it exactly.                        | high       |
| 0x08   | u32  | Number of fields                        | Every struct has between 4 and 14 fields, and the indices it reads are all valid.                                             | high       |

No struct in these files has fewer than 4 fields. The encoding of a struct with one or zero fields is therefore UNKNOWN. The parser assumes that a struct with one field stores the field index directly at 0x04, but no file tests this assumption.

Each entry of the field table is 12 bytes.

| Offset | Type | Meaning                                                    | Evidence                                                       | Confidence |
| ------ | ---- | ---------------------------------------------------------- | -------------------------------------------------------------- | ---------- |
| 0x00   | u32  | Field type (see below)                                     | Nine values occur. Each decodes consistently across all files. | high       |
| 0x04   | u32  | Index into the label table                                 | Every index is valid, and every label is used.                 | high       |
| 0x08   | u32  | The value itself, or an offset or index, depending on type | See the field type table.                                      | high       |

Each entry of the label table is a 16-byte name, padded with NUL bytes, for example "EXTENT". No struct contains the same label twice. A label does not determine the field type: "TEXT" is a string in 1,631 fields and a struct in 2,016 fields.

## Field types

The nine field types that occur are listed below. Types 0, 4, 5 and 8 store their value in the field entry itself. The others store an offset or an index there.

| Type | Count  | Value                                                                                              | Evidence                                                                             | Confidence |
| ---- | ------ | -------------------------------------------------------------------------------------------------- | ------------------------------------------------------------------------------------ | ---------- |
| 0    | 8,002  | u8 stored in the field entry                                                                       | Values are only 0, 1 and 2. The labels are flags such as PULSING and Obj_Locked.     | medium     |
| 4    | 2,016  | u32 stored in the field entry                                                                      | Used only by STRREF. Values range up to 49,116, and 0xFFFFFFFF marks no string.      | medium     |
| 5    | 32,222 | s32 stored in the field entry                                                                      | Coordinates such as LEFT and WIDTH are plausible, and Obj_ParentID uses -1.          | high       |
| 8    | 250    | f32 stored in the field entry                                                                      | ALPHA is always 1.0 and ROTATE is always 0.0.                                        | medium     |
| 10   | 6,235  | String: offset into field data, where a u32 length is followed by that many characters             | Decodes to tags and text such as "LBL_BARKTEXT". The data region is covered exactly. | high       |
| 11   | 12,484 | Resref: offset into field data, where a u8 length (at most 16) is followed by that many characters | Decodes to texture and font names that exist in the game files (see below).          | high       |
| 14   | 8,868  | Index into the struct table                                                                        | Every non-root struct is referenced exactly once by a type 14 or type 15 field.      | high       |
| 15   | 84     | List: offset into the list index region, where a u32 count is followed by that many struct indices | Used only by CONTROLS. The list index region is covered exactly.                     | high       |
| 17   | 5,549  | Vector: offset into field data, where three f32 values are stored                                  | Used only by COLOR. Components lie between 0 and 1, except the value (-1, -1, -1).   | high       |

The type numbers between 0 and 17 that do not occur (1, 2, 3, 6, 7, 9, 12, 13 and 16) are UNKNOWN.

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

BORDER, HILIGHT, SELECTED and HILIGHTSELECTED describe a frame and its fill. Each holds the resrefs CORNER, EDGE and FILL, together with FILLSTYLE, DIMENSION, INNEROFFSET, COLOR and PULSING. TEXT is a struct with TEXT (a string), STRREF, FONT, ALIGNMENT, COLOR and PULSING. The text of a control comes either from the TEXT string or from STRREF, a string number. The largest STRREF value, 49,116, is below 49,265, which is the u32 at offset 0x0C of `dialog.tlk`. That value is probably the string count, but the TLK format has not been examined.

Resrefs name textures and fonts. All 2,016 FONT values name a known resource. Of the CORNER, EDGE, FILL and IMAGE values that are not empty, all name a texture in the texture packs or a resource in `chitin.key`, except nine FILL names, for example `po_mzaalbar` and `georgem`. None of the nine occurs in a RIM archive either.

The meanings of FILLSTYLE (0, 1 or 2), DIMENSION, INNEROFFSET, PULSING, DRAWSTYLE and FLIPSTYLE are UNKNOWN. ALIGNMENT takes only the values 9, 10, 12, 17, 18, 20 and 34. These values fit a bit field in which 1, 2 and 4 select the horizontal position and 8, 16 and 32 select the vertical position. This is a hypothesis that has not been tested. The meaning of COLOR (-1, -1, -1) is UNKNOWN. It may mean that no color is set.

# Music: streammusic/\*.wav

The game's music is in `streammusic/`, which holds 119 files. All of them have the extension `.wav`, but none can be played as stored. They use two layouts, each of which hides a standard audio file behind extra bytes. The converter is `kotor/audio.go`. All 119 files pass its check: walking the MP3 frames or WAV chunks of each file ends exactly at the end of the file.

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

The `music` stage of `cmd/convert` writes the 68 Layout A files to `assets/streammusic/` as MP3 files, with the fake header removed. It writes the 51 Layout B files as WAV files with 16-bit PCM audio, with the stub removed and ADPCM decoded. ffprobe opens all 119 outputs without error. It reports all 51 WAV outputs as 16-bit stereo PCM, 50 at 44,100 Hz and one at 22,050 Hz.

# Sound effects and voices: streamsounds/\*.wav

The folder `streamsounds/` holds 970 files with the extension `.wav`. They are combat voice lines, creature sounds, ambient loops and a few music stingers that the game streams from disk instead of loading from the BIF archives. Every file uses Layout B from the music section: the same 470-byte MP3 stub, followed by a complete WAV file at offset 0x1D6 (470). No file uses Layout A, and none uses ADPCM. The converter `cmd/convert` (stage `sounds`) strips the stub and writes the embedded WAV unchanged to `assets/streamsounds/`.

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

The suffix names the sound-set slot. Examples are ATK1 (attack), BAT1 (battle cry), HIT1 (hit), DEAD (death) and SLCT (selected). Party members have about 29 slots each, including CRIT, POIS, LOW, STLH (stealth) and SRCH (search). The prefixes appear in both upper and lower case. No two names collide when case is ignored.

## Verification

All 970 files pass the parser: the chunk walk of each embedded WAV ends exactly at the end of the file. ffprobe opens all 970 outputs. ffmpeg decodes every output without error, and each decode yields exactly the sample count implied by its data chunk.

# Voice-over: streamwaves/\*\*/\*.wav

The folder `streamwaves/` holds the spoken dialog of the game: 13,799 files with the extension `.wav`, 552 MB in total. Every file uses Layout A from the music section: a 58-byte fake WAV header followed by an MP3 stream. No file uses Layout B. The converter `cmd/convert` (stage `waves`) strips the header and writes the MP3 stream unchanged to `assets/streamwaves/`, keeping the folder structure.

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

All 13,310 files in conversation folders are named N, then the module folder name, then the conversation folder name, then three digits, then one more character. In 13,297 files the last character is "\_". In 13 files it is "A" or "B", as in `NM11AAGADO02018A.wav` and `NM11AAGADO02018B.wav`, which appear to be two parts of one line. The three digits are probably the index of the dialog node that speaks the line. The name `nm24aadroi01000_` occurs in `modules/kas_m24aa_s.rim`, which suggests that dialog resources refer to these files by name. The meaning of the digits and the format of the dialog resources are unverified.

The following table classifies the 489 top-level files by name.

| Name pattern            | Files | Probable meaning                                                                                              |
| ----------------------- | ----- | ------------------------------------------------------------------------------------------------------------- |
| N_G\<speaker\>\_\<kind> | 421   | Generic lines of non-player characters, such as `N_GDUROS_COML1`. Names of this form occur in `dialog.tlk`.   |
| N_M\<number\>...        | 59    | Dialog lines that follow the conversation-folder scheme but lie at the top level, such as `N_M1ANLEVE98003_`. |
| BA, MA, MI              | 8     | Cutscene lines, such as `BA02CS001` and `MA18CS006A`.                                                         |
| \_m40acdart03999\_      | 1     | A conversation-scheme name with a leading underscore instead of N.                                            |

The prefixes appear in both upper and lower case. No two paths collide when case is ignored. The meanings in the table are inferred from the names and are unverified.

## Verification

All 13,799 files pass the parser in `kotor/audio.go`: the MP3 frame walk of each file ends exactly at the end of the file. ffprobe opens all 13,799 outputs without error. ffmpeg decodes every output without error, and each decode yields the sample count given above.

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

The name of every LIP resource equals the name of a voice file in `streamwaves/`, ignoring case and the `.wav` extension. All 18,206 resources have such a file. The LIP length is 13 to 49 ms shorter than the voice file's decoded duration (median 31 ms), which is roughly the length of one MP3 frame (36 ms). Of the 13,799 voice files, 889 have no LIP resource. 98 of them are top-level files, such as the cutscene line `BA02CS001.wav`, and 791 are conversation lines, such as `m01aa/tras02/NM01AATRAS02090_.wav`. Why these lines lack lip sync is unknown.

# Movies: movies/\*.bik

The game's 61 cutscenes and logo screens are in `movies/`. They are Bink video files, a format by RAD Game Tools that the game plays with `binkw32.dll`. All 61 files begin with the signature "BIKi", which `file` identifies as Bink Video revision i. The parser is `kotor/bink.go`, and the converter is the `movies` stage of `cmd/convert`. All 61 files pass the parser's check: the header size field matches the file size, the frame index ends exactly at the end of the file, the largest-frame field equals the largest frame, and every audio packet fits inside its frame.

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

The `movies` stage of `cmd/convert` uses ffmpeg to convert each movie to an MP4 file in `assets/movies/`, with H.264 video and AAC audio. The video uses a constant quality setting (CRF 12), because at CRF 18 frames in 12 movies fell below 40 dB PSNR (peak signal-to-noise ratio, a measure of how closely the output matches the source).

`cmd/verify` checks all 61 outputs with ffprobe and ffmpeg. For each movie, ffprobe must report the frame size, the frame count, the sample rate, the channel count and the audio duration of the Bink file, with the duration within 0.1 seconds. Every output frame must also reach at least 40 dB PSNR against the decoded Bink frame. All 61 outputs pass. The lowest PSNR of any frame is 41.8 dB.
