# territories-game
A package and sample program that manages a Risk-style territorial conquest game, storing player nations and holdings in a SQLite database. It is still a work in progress.

# Goal
This project is not meant to handle things like turn order, player management, etc. It only is meant to handle things like battle resolution, army management and migration, and map exporting based on the game events. A consuming application is expected to handle the rest of the game logic.

# Building
Projects using territories-game must be built with the sqlite_math_functions tag, as it uses some SQLite math functions that are not included by default.

To build the sample program, run `go build -tags sqlite_math_functions ./cmd/territories-referee`.

To run tests, run `go test -tags sqlite_math_functions ./pkg/...`.

**Important:** The `sqlite_math_functions` tag is required if doTurnManagement is set to true (doTurnManagement determines whether turn management is handled automatically by the package)

# Usage
When run, territories-referee will load config.json (see config.example.json for an example), connect to the SQLite database, and do the given action, if registered. The following actions are built-in:
- `join` - Join a player to the game, initializing a nation with an army at a territory.
- `color` - Set the color of a nation.
- `move` - Move armies from one territory to another.
- `attack` - Attack a territory from another territory.
- `raise` - Add one unit to an army in a territory.

## `join` action arguments
Argument      | Description
--------------|------------
`user`        | The name of the player joining the game. This must be unique.
`nation name` | The name of the nation being created. This must be unique.
`territory`   | The territory where the nation will start. This must not already have an army, and must be a valid territory in the map and configuration file. This can be an abbreviation (e.g., "DC), a full name (e.g., "District of Columbia"), or an alias (e.g., "Washington DC") as defined in the configuration file.


## `color` action arguments
Argument | Description
---------|------------
`user`   | The name of the player whose nation color is being set. This must match a nation name in the database.
`color`  | The color to set for the nation. This can be any valid CSS color (e.g., "red", "#ff0000", "rgb(255, 0, 0)", etc.), but must not be already used by another army in the game.

## `move` action arguments
Argument           | Description
-------------------|------------
`user`             | The name of the player moving armies. This must match a nation name in the database.
`armies`           | The number of armies to move. If not specified, all armies will be moved.
`source territory` | The territory from which armies are being moved. This must be a valid territory in the map and configuration file, and must have at least the specified number of armies that are the player's.
`destination`      | The territory to move the armies to. This must be a valid territory in the map and configuration file that neighbors the source territory, and must not have an army that is not the player's. The resulting number of armies in the destination territory must not exceed the maximum number of armies allowed per territory.

## `raise` action arguments
Argument           | Description
-------------------|------------
`user`             | The name of the player raising armies. This must match a nation name in the database.
`territory`        | The territory where the army is being raised. This must be a valid territory in the map and configuration file, and must not be empty. The army/armies in the territory must be the player's, and the number of armies in the territory must not exceed the maximum number of armies allowed per territory after the raise.

## `attack` action arguments
Argument      | Description
--------------|------------
`user`        | The name of the player attacking. This must match a nation name in the database.
`attacking`   | The territory whose armies are attacking. This must be a valid territory in the map and configuration file, and must have at least one army that is the player's.
`destination` | The territory being attacked. This must be a valid territory in the map and configuration file that neighbors the source territory, and must have an army that is not the player's. If the attack is successful, the defending armies will be reduced, and if all defending armies are defeated, the territory will no longer be claimed.

# Combat
Battle calculations are calculated taking into consideration the number of attacking vs defending armies, with some randomness. If all defending armies in the territory are defeated, the territory is no longer claimed, and can be moved into.

# Map details
The map is a SVG file that is copied to the configured output directory. The copy is modified to reflect in-game events, and rendered to a PNG file with ffmpeg. [usa-with-territories.svg](./usa-with-territories.svg) is provided for example purposes, but any SVG file with the following requirements can be used:
- Each configured territory must have a corresponding path element with the abbreviation as the value of the id attribute.
- Each configured territory must have a corresponding circle element with an id attribute value of "&lt;id&gt;-armies" with valid cx, cy, and r attributes to indicate the position and size of the armies to be drawn in that territory.
- Currently, it is also required to have a g element with an id attribute value of "nations-list" where the nations will be listed, and a rect element with an id attribute value of "nations-list-bounds" with valid x, y, width, and height attributes to indicate the bounds of the nations list. This may be made optional in the future.