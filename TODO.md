# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Walkmeshes (BWM, types 2016, 2052 and 2053).** The 1,554 resources in `data/models.bif` begin with "BWM V1.0". The binary layout has not been examined, and nothing is converted.
2. **Bink compressed audio and video.** The movies are converted with ffmpeg, but the layout of the compressed audio and video data inside each frame is UNKNOWN.
3. **Saved games: more samples.** The only saved game, `saves/000002 - Game1/`, is documented and converted, but two layouts in `GLOBALVARS.res` cannot be settled from it, because every value is 0: the bit order of `ValBoolean` and the layout of `ValLocation`. A save in which a known global boolean is true and a global location is set would settle both. The rule that decides which faction pairs `repute.fac` stores is also UNKNOWN.

Individual fields marked UNKNOWN in `FILE_FORMATS.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
