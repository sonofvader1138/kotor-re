# File formats of Star Wars: Knights of the Old Republic

This document describes the game's file formats as determined from the files in `swkotor`. Each field lists the evidence for it and a confidence level. Fields that have not been verified are marked UNKNOWN.

All integers and floats are little-endian. Offsets are in hexadecimal and count from the start of the structure they belong to, unless a section states otherwise.

# Archives: chitin.key and data/\*.bif

Most game resources are stored in 26 BIF archives in `data/`. A BIF archive holds resource data but no names. The file `chitin.key` (the KEY file) lists every resource, gives it a name, and records which BIF archive holds it. Both files are needed to extract a named resource. The reader is `scripts/kotor.js`.

A resource name is called a resref. It is at most 16 characters long. A resource is identified by its resref together with a numeric resource type.

## KEY file

The KEY file begins with a 64-byte header.

| Offset | Type    | Meaning                                | Evidence                                               | Confidence |
| ------ | ------- | -------------------------------------- | ------------------------------------------------------ | ---------- |
| 0x00   | char[8] | Signature "KEY V1 "                    | Present in the only KEY file.                          | high       |
| 0x08   | u32     | Number of BIF archives (26)            | Equals the number of entries in the BIF table.         | high       |
| 0x0C   | u32     | Number of resources (25,836)           | Equals the total number of entries in all 26 archives. | high       |
| 0x10   | u32     | Offset of the BIF table (64)           | The table parses consistently from this offset.        | high       |
| 0x14   | u32     | Offset of the resource table (847)     | The table ends exactly at the end of the file.         | high       |
| 0x18   | u32     | Build year minus 1900 (103, i.e. 2003) | The value is plausible. It has not been confirmed.     | low        |
| 0x1C   | u32     | Build day of the year (309)            | The value is plausible. It has not been confirmed.     | low        |
| 0x20   | u8[32]  | Reserved                               | All bytes are zero.                                    | medium     |

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

The archives contain 29 resource types. Only two are identified: 2002 is an MDL model and 3008 is its MDX vertex data. The others are UNKNOWN. The number of resources of each type is listed below.

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

# 3D models: MDL and MDX

Each 3D model is stored as two resources with the same resref. The MDL resource (type 2002) holds the model's structure: a tree of nodes, some of which carry meshes. The MDX resource (type 3008) holds the vertex data for those meshes. The converter is `scripts/mdl2obj.js`. It writes Wavefront OBJ files.

The models examined here are in `data/models.bif`, which contains 2,445 MDL resources and 2,445 MDX resources. Models may also exist in the RIM, MOD and ERF files, which have not been examined.

All 2,445 models pass the converter's checks. Every read stays inside the file, every face index refers to an existing vertex, and the vertex positions stored in the MDL match those stored in the MDX for all 11,248,282 vertices. Of these models, 368 contain no meshes.

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

| Bit  | Meaning                                                 | Evidence                                            | Confidence |
| ---- | ------------------------------------------------------- | --------------------------------------------------- | ---------- |
| 0x01 | Set on every node                                       | Set on all 107,605 nodes. Its meaning is not known. | medium     |
| 0x20 | The node has a mesh header directly after the node      | Meshes parse correctly in all 2,445 models.         | high       |
| 0x40 | The mesh is skinned, that is, deformed by several bones | Set only on the three body meshes of c_bantha.      | low        |

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
| 0x144  | u32      | Offset of this mesh's vertex records in the MDX file                               | MDX positions match MDL positions for every vertex.                                                     | high       |
| 0x148  | u32      | Offset of the vertex positions in the MDL file. Each position is three f32 values. | MDX positions match MDL positions for every vertex.                                                     | high       |

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

## Unidentified resources in models.bif

The archive also contains resources of types 2016 (1,202), 2052 (156) and 2053 (196). Their formats are UNKNOWN. The string "BWM V1.0" appears in one of these resources and may mark a walkmesh, but this has not been examined.

# Music: streammusic/\*.wav

The game's music is in `streammusic/`, which holds 119 files. All of them have the extension `.wav`, but none can be played as stored. They use two layouts, each of which hides a standard audio file behind extra bytes. The converter is `scripts/kotor_audio.js`. All 119 files pass its check: walking the MP3 frames or WAV chunks of each file ends exactly at the end of the file.

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

The 50 ambient files, whose names begin with `al_`, and `mus_loadscreen.wav` begin with 470 bytes of MP3 data. These bytes form three MPEG-2 Layer III frames at 22,050 Hz and contain the encoder string "LAME3.93". A complete WAV file follows at offset 0x1D6 (470). The purpose of the stub is not known.

In 47 of the 51 embedded WAV files the audio is IMA ADPCM, a 4-bit compressed format, in stereo at 44,100 Hz. The other four contain uncompressed 16-bit PCM. Five files carry LIST or smpl metadata chunks after the audio. The chunks of every file end exactly at the end of the file.

## IMA ADPCM encoding

The ADPCM data follows the standard WAV variant of IMA ADPCM. The decoder is `scripts/adpcm2pcm.js`.

The audio is split into blocks of 2,048 bytes. Each block holds 2,041 samples per channel. A block begins with a 4-byte header for each channel: an s16 starting sample, a u8 step index, and a zero byte. The rest of the block alternates between channels in groups of 4 bytes, which hold 8 samples. Within each byte the low 4 bits come first. The "fact" chunk stores the total number of samples per channel.

The decoder was checked as follows. Each block header stores the sample the encoder expected next. In all 47 files the difference between that sample and the last decoded sample of the previous block is about the same as the difference between any two neighbouring samples. When the order of the 4-bit values is swapped as a test, this difference grows 8 to 14 times.
