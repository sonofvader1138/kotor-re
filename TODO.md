# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Remaining `patch.erf` resources.** The archive holds 84 TPC textures, 4 2DA tables and 1 MDL model that are neither documented nor converted. Only its 7 GUI layouts are converted.
2. **Room visibility (VIS, type 3001).** The 112 resources in `data/lightmaps*.bif` are ASCII text that lists, for each room, other rooms. The format is not documented and the files are not converted.
3. **Room layouts (LYT, type 3000).** The 124 resources in `data/layouts.bif` are ASCII text beginning with "#MAXLAYOUT ASCII" that place rooms by coordinates. The format is not documented and the files are not converted.
4. **Saved game.** `saves/000002 - Game1/` holds `SAVEGAME.sav` (an ERF archive with signature "MOD V1.0"), three GFF files (`GLOBALVARS.res`, `PARTYTABLE.res`, `savenfo.res`) and `Screen.tga`. The containers can be read with existing code. Their content, especially the packing of variables in `GLOBALVARS.res`, is not examined. Only one save exists, so more saves are needed for comparison.
5. **Walkmeshes (BWM, types 2016, 2052 and 2053).** The 1,554 resources in `data/models.bif` begin with "BWM V1.0". The binary layout has not been examined, and nothing is converted.
6. **Bink compressed audio and video.** The movies are converted with ffmpeg, but the layout of the compressed audio and video data inside each frame is UNKNOWN.

Individual fields marked UNKNOWN in `FILE_FORMATS.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
