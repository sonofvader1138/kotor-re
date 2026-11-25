# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Saved games need more samples.** The only saved game, `saves/000002 - Game1/`, is documented and converted, but every value in its `GLOBALVARS.res` is 0. The Xbox code now gives the bit order of `ValBoolean` and the layout of `ValLocation`, but no PC file tests them. A save in which a known global boolean is true and a global location is set would confirm both for the PC build. `ReadGlobals` should then decode `ValLocation` into positions and facings instead of writing hexadecimal. The rule that decides which faction pairs `repute.fac` stores is also UNKNOWN.

2. **The model outputs predate the integer reading of control point smoothing.** The emitter field at 0x10 is now read as an integer, but the glTF files in `extract/pc` were written while it was read as f32, so 40 of them hold 1e-45 where the value is 1. Running the model stage of `cmd/extract` again would correct them.

3. **Models do not inherit the animations of their supermodels.** Most creature, party and player models hold no animations of their own and name a supermodel, such as `S_Female03`, whose animations they probably use. The converter writes only the animations that a model holds itself. Merging them requires matching the nodes of the two models, probably by name, because their node numbers differ. The Xbox code shows that a model stores a pointer to its loaded supermodel at 0x64 and counts references at 0x54, but the routine that looks up an animation was not found. How the game combines a model with its supermodel, and why the node count at 0x2C of the model header exceeds the tree, are therefore still UNKNOWN.

4. **More of the Xbox code can be read.** The routines traced so far are listed with their addresses in `README.md`. Fields that the Xbox code could still settle include the order of the six cube faces of a TPC texture, the float at 0x04 of the TPC header, the slots 28 to 48 of an SSF sound set, the meanings of the GUI fields such as FILLSTYLE and ALIGNMENT, and the mouth shapes of LIP files.

Individual fields marked UNKNOWN in `README.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
