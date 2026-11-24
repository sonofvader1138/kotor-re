# TODO

This file lists the resources that still need to be reversed, documented or converted. Items are ordered from easiest to hardest, as estimated from their structure and from the readers that already exist.

1. **Saved games need more samples.** The only saved game, `saves/000002 - Game1/`, is documented and converted, but every value in its `GLOBALVARS.res` is 0. As a result, the bit order of `ValBoolean` and the layout of `ValLocation` cannot be settled from it. A save in which a known global boolean is true and a global location is set would settle both. The rule that decides which faction pairs `repute.fac` stores is also UNKNOWN.

2. **Models do not inherit the animations of their supermodels.** Most creature, party and player models hold no animations of their own and name a supermodel, such as `S_Female03`, whose animations they probably use. The converter writes only the animations that a model holds itself. Merging them requires matching the nodes of the two models, probably by name, because their node numbers differ. How the game combines a model with its supermodel, and why the node count at 0x2C of the model header exceeds the tree, are UNKNOWN.

3. **The controller and emitter names are not confirmed by the game.** The code of `swkotor.exe` is encrypted, so the names of the controller types, the emitter flag bits and the light fields come from other descriptions of the format and are checked only against value ranges. The type 256, named `numcontrolpts`, holds fractions, which contradicts its name.

Individual fields marked UNKNOWN in `README.md` are not listed here. Most of them are constant or never occur in the game files, so the files alone cannot settle them.
