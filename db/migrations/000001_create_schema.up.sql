CREATE TABLE buckets (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    name TEXT NOT NULL
);

CREATE UNIQUE INDEX idx_buckets_name ON buckets(name);
CREATE INDEX idx_buckets_deleted_at ON buckets(deleted_at);

CREATE TABLE object_contents (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    sha256 TEXT NOT NULL,
    size INTEGER NOT NULL,
    path TEXT NOT NULL,
    ref_count INTEGER NOT NULL DEFAULT 0
);

CREATE UNIQUE INDEX idx_object_contents_sha256 ON object_contents(sha256);
CREATE INDEX idx_object_contents_deleted_at ON object_contents(deleted_at);

CREATE TABLE objects (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    created_at DATETIME,
    updated_at DATETIME,
    deleted_at DATETIME,
    key TEXT NOT NULL,
    bucket_id INTEGER NOT NULL,
    content_id INTEGER NOT NULL,
    CONSTRAINT fk_object_contents_objects FOREIGN KEY (content_id) REFERENCES object_contents(id) ON UPDATE CASCADE ON DELETE RESTRICT,
    CONSTRAINT fk_buckets_objects FOREIGN KEY (bucket_id) REFERENCES buckets(id) ON UPDATE CASCADE ON DELETE CASCADE
);

CREATE INDEX idx_objects_content_id ON objects(content_id);
CREATE UNIQUE INDEX idx_bucket_key ON objects(key, bucket_id);
CREATE INDEX idx_objects_deleted_at ON objects(deleted_at);
