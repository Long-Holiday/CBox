package store

const schema = `
CREATE TABLE IF NOT EXISTS images (
    id TEXT PRIMARY KEY,
    manifest_json TEXT NOT NULL,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS image_tags (
    tag TEXT PRIMARY KEY,
    image_id TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    FOREIGN KEY (image_id) REFERENCES images(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS volumes (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    source TEXT NOT NULL,
    mode TEXT NOT NULL,
    immutable INTEGER NOT NULL DEFAULT 0,
    manifest_hash TEXT,
    created_at DATETIME NOT NULL
);

CREATE TABLE IF NOT EXISTS containers (
    id TEXT PRIMARY KEY,
    name TEXT UNIQUE NOT NULL,
    image_id TEXT NOT NULL,
    state TEXT NOT NULL,
    desired_state TEXT NOT NULL,
    command_json TEXT NOT NULL,
    env_json TEXT,
    workdir TEXT NOT NULL DEFAULT '',
    resource_json TEXT,
    restart_policy TEXT NOT NULL,
    resume_command_json TEXT,
    secrets_json TEXT,
    runtime_id TEXT,
    exit_code INTEGER,
    created_at DATETIME NOT NULL,
    started_at DATETIME,
    finished_at DATETIME
);

CREATE TABLE IF NOT EXISTS container_mounts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    container_id TEXT NOT NULL,
    volume_id TEXT NOT NULL,
    source TEXT NOT NULL,
    target TEXT NOT NULL,
    mode TEXT NOT NULL,
    FOREIGN KEY (container_id) REFERENCES containers(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS runtimes (
    id TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    session TEXT NOT NULL,
    profile TEXT NOT NULL,
    requested_gpu TEXT NOT NULL,
    actual_gpu TEXT NOT NULL,
    state TEXT NOT NULL,
    created_at DATETIME NOT NULL,
    last_seen DATETIME NOT NULL,
    cache_json TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS contexts (
    name TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    profile TEXT NOT NULL,
    auto_schedule INTEGER NOT NULL DEFAULT 0,
    is_current INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS events (
    id TEXT PRIMARY KEY,
    object_type TEXT NOT NULL,
    object_id TEXT NOT NULL,
    type TEXT NOT NULL,
    timestamp DATETIME NOT NULL,
    payload TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_containers_state ON containers(state);
CREATE INDEX IF NOT EXISTS idx_containers_runtime_id ON containers(runtime_id);
CREATE INDEX IF NOT EXISTS idx_runtimes_state ON runtimes(state);
CREATE INDEX IF NOT EXISTS idx_events_object ON events(object_type, object_id);
`

func (db *DB) Migrate() error {
	_, err := db.Exec(schema)
	return err
}
