-- Migration 002: Create fingerprints table
-- Depends on songs table from Migration 001

CREATE TABLE IF NOT EXISTS fingerprints (
    id            BIGSERIAL       PRIMARY KEY,
    hash          INTEGER         NOT NULL,
    song_id       INTEGER         NOT NULL,
    anchor_time   INTEGER         NOT NULL,

    CONSTRAINT fk_fingerprints_song
        FOREIGN KEY (song_id)
        REFERENCES songs(id)
        ON DELETE CASCADE
);

-- ============================================================
-- CRITICAL INDEX — do not drop or disable
-- Every single match query runs through this index
-- ============================================================
CREATE INDEX IF NOT EXISTS idx_fingerprints_hash
    ON fingerprints(hash);

CREATE INDEX IF NOT EXISTS idx_fingerprints_song_id
    ON fingerprints(song_id);

COMMENT ON TABLE  fingerprints             IS 'Audio fingerprint hashes for all indexed songs';
COMMENT ON COLUMN fingerprints.hash        IS '32-bit encoded hash: anchor_freq(9bit) | target_freq(9bit) | delta_time(14bit)';
COMMENT ON COLUMN fingerprints.song_id     IS 'Reference to the parent song';
COMMENT ON COLUMN fingerprints.anchor_time IS 'Time offset of the anchor peak in FFT frame units — NOT milliseconds';