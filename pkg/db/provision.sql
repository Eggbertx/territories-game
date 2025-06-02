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
	territory CHAR(2) NOT NULL UNIQUE,
	nation_id INTEGER NOT NULL,
	army_size INTEGER NOT NULL CHECK(army_size > 0),

	CONSTRAINT holdings_nations_id_fk
		FOREIGN KEY(nation_id)
		REFERENCES nations(id)
		ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS battles (
	id INTEGER PRIMARY KEY AUTOINCREMENT NOT NULL,
	attacker_id INTEGER NOT NULL,
	defender_id INTEGER NOT NULL,
	attacker_size_start INTEGER NOT NULL CHECK(attacker_size_start > 0),
	defender_size_start INTEGER NOT NULL CHECK(defender_size_start > 0),
	attacker_size_end INTEGER NOT NULL,
	defender_size_end INTEGER NOT NULL,
	attacker_wins BOOLEAN NOT NULL,

	CONSTRAINT battles_attacker_id_fk
		FOREIGN KEY(attacker_id)
		REFERENCES nations(id)
		ON DELETE CASCADE,

	CONSTRAINT battles_defender_id_fk
		FOREIGN KEY(defender_id)
		REFERENCES nations(id)
		ON DELETE CASCADE
);

CREATE VIEW IF NOT EXISTS v_nation_holdings
	AS SELECT holdings.id as id, nations.id as nation_id, country_name, color, territory, army_size, player
	FROM holdings left join nations on nation_id = nations.id;