# Reverse Engineering

## Objective

- You are reverse engineering the game in `~/re/dump/<platform>`, where `<platform>` is `pc` or `xbox`
- You will focus on reverse engineering the file formats

## Tools

The following tools are installed:

- go
  - golangci-lint
- node
- jq
- perl
- python3
- xxd
- file
- binutils
- ffmpeg
- pngcheck
- imagemagick
- assimp-utils
- gltf_validator (Khronos glTF validator)
- xz-utils
- extract-xiso
- steamless

## Rules

- The working directory is `~/re`
- You are forbidden from editing the original game files in `~/re/dump`
- Scratch work goes in the `~/re/temp` directory
- Converted, human-readable (or in otherwise ubiquitous binary format) assets should be stored in the `~/re/extract/<platform>` directory of the build they come from
  - These assets should be primarily organized similarly to how they are organized in the original game files
  - When the original game files don't offer a clear structure, prefer to organize assets by feature and with a flat structure
  - Converted assets must use the most widely supported encoding, not just a common container (e.g. PCM WAV, not ADPCM WAV)
  - Verify each output with an independent tool (e.g. ffprobe, a mesh loader) before storing it in `~/re/extract/<platform>`. If no such tool is available, say so and mark the output unverified
- Reverse engineer only formats specific to the game. Skip standard formats that an installed tool already decodes, such as Bink through ffmpeg, and skip storefront details such as Steam's wrapper.
- Read engine code only when it settles a question about a data format.
- Use the installed tools instead of writing replacements. If a tool fails, report what it needs and wait.
- Gather evidence before hypothesizing: file, xxd, strings, entropy per region. Compare at least 5 samples of each type. If fewer than 5 exist, say so and ask for more.
- Record findings in `~/re/README.md`, which holds all formats in one file. Give each field its offset, type, evidence and confidence. Write UNKNOWN for anything unverified.
- Describe findings for the build (platform, region and revision), not for the particular copy that was dumped.
- A parser is correct only if it consumes each file exactly, keeps offsets inside the file, and yields plausible counts. Report pass/fail per sample.
- After a refactor, rename or port, run the extraction again and compare it with the current `~/re/extract/<platform>`. Compare PNG images by pixels, and report every difference.
- After changing a converter, run its stage again so that `~/re/extract/<platform>` matches the code.
- Write all long-living code in Golang.
- For short-lived scripts, you may use whichever language is most obvious, simple, or easy.

## Writing

- Write documentation, comments, and replies in concise, formal, plain language
  - Cut words that carry no information. Never cut grammar: no sentence fragments, telegraphic notes or shorthand.
  - Lead with what a thing is and why it matters, then give details.
  - Define each term or abbreviation the first time it appears.
  - Give every number its meaning. Write "all 26 archives end exactly at end of file", not "26/26".
  - State uncertainty in a plain sentence, e.g. "The purpose of this field is unknown."
  - Use tables for field layouts and sentences to explain them. Introduce each table with a sentence saying what it describes.
  - Do not use a colon to join two clauses in a sentence, as in "Most of the effort isn't code: it's checking deployed config." Write two sentences or join the clauses with a word such as "because" or "but". This applies to documentation, comments and replies. A colon is still fine when it refers to the colon character itself or is part of a real title, such as "Star Wars: Knights of the Old Republic".
