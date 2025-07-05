CREATE TABLE IF NOT EXISTS nations (
	id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	country_name VARCHAR(125) NOT NULL,
	player VARCHAR(90) NOT NULL,
	color CHAR(6) NOT NULL,
	CONSTRAINT country_name_length CHECK(LENGTH(country_name) > 0),
	CONSTRAINT player_length CHECK(LENGTH(player) > 0),
	CONSTRAINT unique_country_name UNIQUE(country_name)
	CONSTRAINT unique_player UNIQUE(player)
	CONSTRAINT color_length CHECK(LENGTH(color) = 6 OR LENGTH(color) = 3),
	CONSTRAINT unique_color UNIQUE(color)
);

CREATE TABLE IF NOT EXISTS holdings (
	id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	territory VARCHAR(10) NOT NULL UNIQUE,
	nation_id INTEGER NOT NULL,
	army_size INTEGER NOT NULL CHECK(army_size > 0),

	CONSTRAINT holdings_nations_id_fk
		FOREIGN KEY(nation_id)
		REFERENCES nations(id)
		ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS actions (
	id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	nation_id INTEGER,
	action_type VARCHAR(45) NOT NULL,
	is_new_turn BOOLEAN NOT NULL DEFAULT 0,
	timestamp DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

	CONSTRAINT actions_nation_id_fk
		FOREIGN KEY(nation_id)
		REFERENCES nations(id)
		ON DELETE SET NULL
);

CREATE VIEW IF NOT EXISTS v_nation_holdings
	AS SELECT holdings.id as id, nations.id as nation_id, country_name, color, territory, army_size, player
	FROM holdings left join nations on nation_id = nations.id;

CREATE VIEW IF NOT EXISTS v_actions
	AS SELECT actions.id as id, nations.id as nation_id, country_name, player, action_type, is_new_turn, timestamp
	FROM actions left join nations on nation_id = nations.id;

CREATE VIEW IF NOT EXISTS v_new_turn_actions
	AS SELECT actions.id as id, timestamp
	FROM actions WHERE is_new_turn = 1;