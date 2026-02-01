package migrations

import "database/sql"

var migrations = []string{
	// Migration 1: Create my_node table
	`CREATE TABLE IF NOT EXISTS my_node (
		myNodeNum INTEGER PRIMARY KEY,
		model TEXT,
		firmwareVersion TEXT,
		couldUpdate INTEGER DEFAULT 0,
		shouldUpdate INTEGER DEFAULT 0,
		currentPacketId INTEGER NOT NULL,
		messageTimeoutMsec INTEGER NOT NULL,
		minAppVersion INTEGER NOT NULL,
		maxChannels INTEGER NOT NULL,
		hasWifi INTEGER DEFAULT 0,
		deviceId TEXT DEFAULT 'unknown'
	)`,

	// Migration 2: Create nodes table
	`CREATE TABLE IF NOT EXISTS nodes (
		num INTEGER PRIMARY KEY,
		user BLOB,
		long_name TEXT,
		short_name TEXT,
		position BLOB,
		latitude REAL DEFAULT 0.0,
		longitude REAL DEFAULT 0.0,
		snr REAL DEFAULT 3.4028235e+38,
		rssi INTEGER DEFAULT 2147483647,
		last_heard INTEGER DEFAULT 0,
		device_metrics BLOB,
		channel INTEGER DEFAULT 0,
		via_mqtt INTEGER DEFAULT 0,
		hops_away INTEGER DEFAULT -1,
		is_favorite INTEGER DEFAULT 0,
		is_ignored INTEGER DEFAULT 0,
		is_muted INTEGER DEFAULT 0,
		environment_metrics BLOB,
		power_metrics BLOB,
		public_key BLOB,
		notes TEXT DEFAULT ''
	)`,

	// Migration 3: Create nodes indices
	`CREATE INDEX IF NOT EXISTS idx_nodes_last_heard ON nodes(last_heard)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_short_name ON nodes(short_name)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_long_name ON nodes(long_name)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_hops_away ON nodes(hops_away)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_is_favorite ON nodes(is_favorite)`,
	`CREATE INDEX IF NOT EXISTS idx_nodes_last_heard_favorite ON nodes(last_heard, is_favorite)`,

	// Migration 4: Create packet table
	`CREATE TABLE IF NOT EXISTS packet (
		uuid INTEGER PRIMARY KEY AUTOINCREMENT,
		myNodeNum INTEGER DEFAULT 0,
		port_num INTEGER NOT NULL,
		contact_key TEXT NOT NULL,
		received_time INTEGER NOT NULL,
		read INTEGER DEFAULT 1,
		data TEXT NOT NULL,
		packet_id INTEGER DEFAULT 0,
		routing_error INTEGER DEFAULT -1,
		snr REAL DEFAULT 0,
		rssi INTEGER DEFAULT 0,
		hopsAway INTEGER DEFAULT -1,
		sfpp_hash BLOB,
		filtered INTEGER DEFAULT 0
	)`,

	// Migration 5: Create packet indices
	`CREATE INDEX IF NOT EXISTS idx_packet_myNodeNum ON packet(myNodeNum)`,
	`CREATE INDEX IF NOT EXISTS idx_packet_port_num ON packet(port_num)`,
	`CREATE INDEX IF NOT EXISTS idx_packet_contact_key ON packet(contact_key)`,
	`CREATE INDEX IF NOT EXISTS idx_packet_contact_key_port_time ON packet(contact_key, port_num, received_time)`,
	`CREATE INDEX IF NOT EXISTS idx_packet_packet_id ON packet(packet_id)`,

	// Migration 6: Create reactions table
	`CREATE TABLE IF NOT EXISTS reactions (
		myNodeNum INTEGER DEFAULT 0,
		reply_id INTEGER NOT NULL,
		user_id TEXT NOT NULL,
		emoji TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		snr REAL DEFAULT 0,
		rssi INTEGER DEFAULT 0,
		hopsAway INTEGER DEFAULT -1,
		packet_id INTEGER DEFAULT 0,
		status INTEGER DEFAULT 0,
		routing_error INTEGER DEFAULT 0,
		PRIMARY KEY (myNodeNum, reply_id, user_id, emoji)
	)`,

	// Migration 7: Create reactions indices
	`CREATE INDEX IF NOT EXISTS idx_reactions_reply_id ON reactions(reply_id)`,
	`CREATE INDEX IF NOT EXISTS idx_reactions_packet_id ON reactions(packet_id)`,

	// Migration 8: Create contact_settings table
	`CREATE TABLE IF NOT EXISTS contact_settings (
		contact_key TEXT PRIMARY KEY,
		muteUntil INTEGER DEFAULT 0,
		last_read_message_uuid INTEGER,
		last_read_message_timestamp INTEGER,
		filtering_disabled INTEGER DEFAULT 0
	)`,

	// Migration 9: Create log table
	`CREATE TABLE IF NOT EXISTS log (
		uuid TEXT PRIMARY KEY,
		type TEXT NOT NULL,
		received_date INTEGER NOT NULL,
		message TEXT NOT NULL,
		from_num INTEGER DEFAULT 0,
		port_num INTEGER DEFAULT 0,
		from_radio BLOB DEFAULT X''
	)`,

	// Migration 10: Create log indices
	`CREATE INDEX IF NOT EXISTS idx_log_from_num ON log(from_num)`,
	`CREATE INDEX IF NOT EXISTS idx_log_port_num ON log(port_num)`,

	// Migration 11: Create quick_chat table
	`CREATE TABLE IF NOT EXISTS quick_chat (
		uuid INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT DEFAULT '',
		message TEXT DEFAULT '',
		mode INTEGER DEFAULT 1,
		position INTEGER NOT NULL
	)`,

	// Migration 12: Create metadata table
	`CREATE TABLE IF NOT EXISTS metadata (
		num INTEGER PRIMARY KEY,
		proto BLOB NOT NULL,
		timestamp INTEGER DEFAULT (strftime('%s','now') * 1000)
	)`,
	`CREATE INDEX IF NOT EXISTS idx_metadata_num ON metadata(num)`,

	// Migration 13: Create channels table
	`CREATE TABLE IF NOT EXISTS channels (
		idx INTEGER PRIMARY KEY,
		myNodeNum INTEGER NOT NULL,
		role INTEGER DEFAULT 0,
		settings BLOB,
		UNIQUE(idx, myNodeNum)
	)`,

	// Migration 14: Create schema_version table
	`CREATE TABLE IF NOT EXISTS schema_version (
		version INTEGER PRIMARY KEY
	)`,

	// Migration 15: Create telemetry table for historical data
	`CREATE TABLE IF NOT EXISTS telemetry (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		node_num INTEGER NOT NULL,
		telemetry_type TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		via_mqtt INTEGER DEFAULT 0,
		battery_level INTEGER,
		voltage REAL,
		channel_utilization REAL,
		air_util_tx REAL,
		uptime_seconds INTEGER,
		temperature REAL,
		relative_humidity REAL,
		barometric_pressure REAL,
		gas_resistance REAL,
		iaq INTEGER,
		distance REAL,
		lux REAL,
		uv_lux REAL,
		wind_speed REAL,
		wind_direction INTEGER,
		rainfall REAL,
		ch1_voltage REAL,
		ch1_current REAL,
		ch2_voltage REAL,
		ch2_current REAL,
		ch3_voltage REAL,
		ch3_current REAL,
		pm10 INTEGER,
		pm25 INTEGER,
		pm100 INTEGER,
		co2 INTEGER,
		raw_data BLOB
	)`,

	// Migration 16: Create telemetry indices
	`CREATE INDEX IF NOT EXISTS idx_telemetry_node_num ON telemetry(node_num)`,
	`CREATE INDEX IF NOT EXISTS idx_telemetry_type ON telemetry(telemetry_type)`,
	`CREATE INDEX IF NOT EXISTS idx_telemetry_timestamp ON telemetry(timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_telemetry_node_type_time ON telemetry(node_num, telemetry_type, timestamp)`,

	// Migration 17: Create waypoints table
	`CREATE TABLE IF NOT EXISTS waypoints (
		id INTEGER PRIMARY KEY,
		latitude_i INTEGER NOT NULL,
		longitude_i INTEGER NOT NULL,
		expire INTEGER DEFAULT 0,
		locked_to INTEGER DEFAULT 0,
		name TEXT NOT NULL,
		description TEXT DEFAULT '',
		icon INTEGER DEFAULT 0,
		from_node INTEGER DEFAULT 0,
		via_mqtt INTEGER DEFAULT 0,
		created_at INTEGER DEFAULT (strftime('%s','now')),
		updated_at INTEGER DEFAULT (strftime('%s','now'))
	)`,

	// Migration 18: Create waypoints indices
	`CREATE INDEX IF NOT EXISTS idx_waypoints_name ON waypoints(name)`,
	`CREATE INDEX IF NOT EXISTS idx_waypoints_from_node ON waypoints(from_node)`,
	`CREATE INDEX IF NOT EXISTS idx_waypoints_expire ON waypoints(expire)`,

	// Migration 19: Create traceroutes table
	`CREATE TABLE IF NOT EXISTS traceroutes (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		from_node INTEGER NOT NULL,
		to_node INTEGER NOT NULL,
		route TEXT NOT NULL,
		route_back TEXT DEFAULT '',
		snr_towards TEXT DEFAULT '',
		snr_back TEXT DEFAULT '',
		hop_count INTEGER DEFAULT 0,
		timestamp INTEGER NOT NULL,
		success INTEGER DEFAULT 1
	)`,

	// Migration 20: Create traceroutes indices
	`CREATE INDEX IF NOT EXISTS idx_traceroutes_from_node ON traceroutes(from_node)`,
	`CREATE INDEX IF NOT EXISTS idx_traceroutes_to_node ON traceroutes(to_node)`,
	`CREATE INDEX IF NOT EXISTS idx_traceroutes_timestamp ON traceroutes(timestamp)`,
	`CREATE INDEX IF NOT EXISTS idx_traceroutes_from_to ON traceroutes(from_node, to_node)`,
}

func Run(db *sql.DB) error {
	// Get current version
	var currentVersion int
	row := db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_version")
	if err := row.Scan(&currentVersion); err != nil {
		// Table might not exist yet, that's ok
		currentVersion = 0
	}

	// Run migrations
	for i, migration := range migrations {
		version := i + 1
		if version <= currentVersion {
			continue
		}

		if _, err := db.Exec(migration); err != nil {
			return err
		}
	}

	// Update version
	if len(migrations) > currentVersion {
		_, err := db.Exec("INSERT OR REPLACE INTO schema_version (version) VALUES (?)", len(migrations))
		return err
	}

	return nil
}
