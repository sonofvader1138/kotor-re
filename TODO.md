# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Saved games need more samples.** The only saved game, `saves/000002 - Game1/`, is documented and converted, but every value in its `GLOBALVARS.res` is 0. The Xbox code and the PC code now give the bit order of `ValBoolean` and the size of `ValLocation`, but no PC file tests them. A save in which a known global boolean is true and a global location is set would confirm both for the PC build. `ReadGlobals` should then decode `ValLocation` into positions and facings instead of writing hexadecimal. The rule that decides which faction pairs `repute.fac` stores is also UNKNOWN.

2. **More of the engine code can be read.** The PC code can be decrypted with Steamless and disassembled like the Xbox code. The routines traced so far are listed with their addresses in `README.md`. The cube face order and the float at 0x04 of the TPC header, the GUI field ALIGNMENT, the localized strings of ERF archives and the resource type names are settled. The SSF slot reader and the events that play each SSF slot, the GUI border fields, the mapping of LIP values to `talk`, the mesh header fields 0xC8 to 0xE4, the use of the TPC alpha mean and the effect of the GUI fields DRAWSTYLE and FLIPSTYLE are now settled as well. No named field is left that the code is known to settle. Other fields marked UNKNOWN may still be settled by reading the routines that use them. The counter values at 0xC8 were written by the original tool, so the engine code probably cannot settle them.

3. **Nine Xbox music files do not decode completely.** ffmpeg reports errors in a few blocks of nine WMA files of `dump/xbox/streammusic`, such as `04.wma` and `mus_bat_shadow.wma`, and decodes about 95 to 99 percent of each. The converter writes no output for them. Another WMA decoder would show whether the files or ffmpeg are at fault.

4. **The Xbox system library files are not converted.** `media/dlcustom.xpr` and the two `.xbx` save images are XPR resource bundles, and `media/Wave.xwb` and `media/Sound.xsb` are XACT wave and sound banks. They belong to Microsoft's Xbox development kit, not to the game, and no installed tool decodes them. Converting them needs a decision on whether such formats are in scope. `dsstdfx.bin` was not examined.

5. **Some Xbox duplicates have no known priority.** `dor_ukn01` and `dor_ukn02` occur as textures in two BIF archives, 50 music tracks exist as both WAV and WMA, and the 32 conversations of `players.erf` differ from those of `templates.bif` with the same names. The converter writes every copy and links models to one texture copy. Which copy the game uses is UNKNOWN.

6. **The Xbox code that tests bit 3 of the texture encoding was not found.** The images settle the swizzled order, but the routine that selects it is unknown.

Individual fields marked UNKNOWN in `README.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
