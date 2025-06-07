# territories-game
A package and sample program that manages a Risk-style territorial conquest game, storing player nations and holdings in a SQLite database. It is still a work in progress.

# Goal
This project is not meant to handle things like turn order, player management, etc. It only is meant to handle things like battle resolution, army management and migration, and map exporting based on the game events. A consuming application is expected to handle the rest of the game logic.

# Usage
When run, territories-referee will load config.json (see config.example.json for an example), connect to the SQLite database, and do the given action, if valid.
In the subject and predicate fields of an action, "territory" can refer to the name, abbreviation, or alias of a territory set in the config file. It must also exist in the configured map file (see [Map details](#map-details) below).

Action | Subject       | Predicate    | Description
-------|---------------|--------------|-------------
join   | nation name   | territory    | Initialize a nation with an army at a territory. A random color will be assigned to the nation. The territory must not already have an army, and the nation names must be unique.
color  | color         | *N/A*        | Set the color of a nation. *color* can be any valid CSS color (e.g. "red", "#ff0000", "rgb(255, 0, 0)", etc.), but must be already used by another army.
move   | territory:n   | territory    | If the subject ends in :n where n is an integer, move n armies from the subject territory to the predicate territory. The territories must be neighboring and the moving territory must have at least n armies that are the player's. If n is not specified, all armies will be moved. The destination territory must not have an army that is not the player's, and the resulting number of armies in the destination territory must not exceed the maximum number of armies allowed per territory.
attack | territory     | territory    | Attack from from the territory referenced in the subject to the territory referenced in the predicate. The territories must be neighboring and the attacked territory must have an army that is not the attacking player's.
raise  | territory     | *N/A*        | Add one unit to an army in a territory. The territory must not be empty, and the army/armies must be the player's.

# Map details
The map is a SVG file that is copied to the configured output directory. The copy is modified to reflect in-game events, and rendered to a PNG file with ffmpeg. [usa-with-territories.svg](./usa-with-territories.svg) is provided for example purposes, but any SVG file with the following requirements can be used:
- Each configured territory must have a corresponding path element with the abbreviation as the value of the id attribute.
- Each configured territory must have a corresponding circle element with an id attribute value of "&lt;id&gt;-armies" with valid cx, cy, and r attributes to indicate the position and size of the armies to be drawn in that territory.
- Currently, it is also required to have a g element with an id attribute value of "nations-list" where the nations will be listed, and a rect element with an id attribute value of "nations-list-bounds" with valid x, y, width, and height attributes to indicate the bounds of the nations list. This may be made optional in the future.

# Combat
Battle calculations are calculated taking into consideration the number of attacking vs defending armies, with some randomness. If all defending armies in the territory are defeated, the territory is no longer claimed, and can be moved into.