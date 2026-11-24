# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Saved games need more samples.** The only saved game, `saves/000002 - Game1/`, is documented and converted, but every value in its `GLOBALVARS.res` is 0. As a result, the bit order of `ValBoolean` and the layout of `ValLocation` cannot be settled from it. A save in which a known global boolean is true and a global location is set would settle both. The rule that decides which faction pairs `repute.fac` stores is also UNKNOWN.

Individual fields marked UNKNOWN in `README.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
