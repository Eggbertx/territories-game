# territories-game
A package and sample program that manages a Risk-style territorial conquest game, storing player nations and holdings in a SQLite database. It is still a work in progress.

# Goal
This project is not meant to handle things like turn order, player management, etc. It only is meant to handle things like battle resolution, army management and migration, and map exporting based on the game events. A consuming application is expected to handle the rest of the game logic.

# Usage
When run, territories-referee will load config.json (see config.example.json for an example), connect to the SQLite database, and do the given action, if valid.

Action | Subject       | Predicate    | Description
-------|---------------|--------------|-------------
join   | nation name   | territory ID | Initialize a nation with an army at a territory. A random color will be assigned to the nation. The territory must not already have an army, and the nation name must not already exist.
color  | color         | *N/A*        | Set the color of a nation. *color* can be any valid CSS color (e.g. "red", "#ff0000", "rgb(255, 0, 0)", etc.)
move   | territoryID:n | territoryID  | Move n armies from from the territory referenced in the subject to the territory referenced in the predicate. The territories must be neighboring, the subject territory must have at least n armies owned by the player, and the predicate territory must be empty. Not yet implemented.
attack | territoryID   | territoryID  | Attack from from the territory referenced in the subject to the territory referenced in the predicate. The territories must be neighboring and the attacked territory must have an army that is not the attacking player's. Not yet implemented.
raise  | territoryID   | *N/A*        | Add one unit to an army in a territory. The territory must have an army that is the player's. Not yet implemented.

# Battle calculation
As the battle resolution is not yet implemented, this is subject to change. The current plan is to use a dice roll system.
An attack would use the formula $x > (b-a)*2+10$ where:
- $a$ is the attacking force (the number of armies in the attacking territory),
- $b$ is the defending force (the number of armies in the defending territory),
- $x$ is the result of a die roll (a random integer between 1 and 20).

The following rules apply to the value of $x$:
- 19-20 is always a victory.
- 13 or higher would be a victory if defending forces outnumber attacking forces by at least 1.
- 11 or higher would be a victory if forces are equivalent.
- 9 or higher would be a victory if attacking forces outnumber defending forces by 1.
- 1 is always a defeat and always results in the loss of at least 1 attacking army.

Losses will be determined by the formula $y = floor(0.5x+a-b-5)$ where $a$, $b$, and $x$ are the same as the previous formula, and $y$ is the number of armies destroyed. Positive results indicate defending armies destroyed while negative results indicate attacking armies destroyed. If the attack was a success, at least one of the defending armies will be destroyed. Any defending armies that survive a successful attack may optionally retreat to allied territory with the moveaction if there is available space.
