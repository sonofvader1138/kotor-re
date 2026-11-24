# Reverse Engineering

## Objective

- You are reverse engineering the game in `~/re/swkotor`
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

## Rules

- The working directory is `~/re`
- You are forbidden from editing the original game files in `~/re/swkotor`
- Scratch work goes in the `~/re/temp` directory
- Converted, human-readable (or in otherwise ubiquitous binary format) assets should be stored in the `~/re/extracted` directory
  - These assets should be primarily organized similarly to how they are organized in the original game files
  - When the original game files don't offer a clear structure, prefer to organize assets by feature and with a flat structure
  - Converted assets must use the most widely supported encoding, not just a common container (e.g. PCM WAV, not ADPCM WAV)
  - Verify each output with an independent tool (e.g. ffprobe, a mesh loader) before storing it in `~/re/extracted`. If no such tool is available, say so and mark the output unverified
- Gather evidence before hypothesizing: file, xxd, strings, entropy per region. Compare at least 5 samples of each type.
- Record findings in `~/re/README.md`, which holds all formats in one file. Give each field its offset, type, evidence and confidence. Write UNKNOWN for anything unverified.
- A parser is correct only if it consumes each file exactly, keeps offsets inside the file, and yields plausible counts. Report pass/fail per sample.
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
