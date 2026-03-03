-- Migration 001: Create songs table
-- Run this first — fingerprints and song_features reference songs

CREATE TABLE IF NOT EXISTS songs (
    id            SERIAL          PRIMARY KEY,
    title         VARCHAR(255),
    artist        VARCHAR(255),
    album         VARCHAR(255),
    duration_ms   INTEGER,
    source        VARCHAR(50)     NOT NULL DEFAULT 'manual',
    spotify_id    VARCHAR(100),
    file_hash     VARCHAR(64)     NOT NULL,
    created_at    TIMESTAMP       NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMP       NOT NULL DEFAULT NOW(),

    CONSTRAINT songs_file_hash_unique
        UNIQUE (file_hash),

    CONSTRAINT songs_source_check
        CHECK (source IN ('manual', 'spotify', 'unknown'))
);

CREATE INDEX IF NOT EXISTS idx_songs_artist
    ON songs(artist);

CREATE INDEX IF NOT EXISTS idx_songs_spotify_id
    ON songs(spotify_id)
    WHERE spotify_id IS NOT NULL;

COMMENT ON TABLE  songs                IS 'Indexed music tracks in the DJ library';
COMMENT ON COLUMN songs.file_hash      IS 'SHA256 hash of the original audio file — prevents duplicate uploads. VARCHAR(64) not CHAR(64) to avoid space padding.';
COMMENT ON COLUMN songs.source         IS 'Origin of the track: manual upload or spotify metadata';
COMMENT ON COLUMN songs.duration_ms    IS 'Track duration in milliseconds';
COMMENT ON COLUMN songs.spotify_id     IS 'Spotify track ID, nullable, only set when user provides a Spotify URL';