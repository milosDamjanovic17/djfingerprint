-- Migration 003: Create song_features table
-- Depends on songs table from Migration 001
-- All feature columns are nullable — populated only for Spotify-linked tracks

CREATE TABLE IF NOT EXISTS song_features (
    id                  SERIAL          PRIMARY KEY,
    song_id             INTEGER         NOT NULL,
    spotify_track_id    VARCHAR(100),
    energy              FLOAT,
    danceability        FLOAT,
    valence             FLOAT,
    acousticness        FLOAT,
    instrumentalness    FLOAT,
    liveness            FLOAT,
    speechiness         FLOAT,
    loudness            FLOAT,
    tempo               FLOAT,
    musical_key         INTEGER,
    created_at          TIMESTAMP       NOT NULL DEFAULT NOW(),

    CONSTRAINT fk_song_features_song
        FOREIGN KEY (song_id)
        REFERENCES songs(id)
        ON DELETE CASCADE,

    CONSTRAINT song_features_song_id_unique
        UNIQUE (song_id),

    CONSTRAINT song_features_energy_range
        CHECK (energy IS NULL OR (energy >= 0.0 AND energy <= 1.0)),

    CONSTRAINT song_features_danceability_range
        CHECK (danceability IS NULL OR (danceability >= 0.0 AND danceability <= 1.0)),

    CONSTRAINT song_features_valence_range
        CHECK (valence IS NULL OR (valence >= 0.0 AND valence <= 1.0)),

    CONSTRAINT song_features_musical_key_range
        CHECK (musical_key IS NULL OR (musical_key >= 0 AND musical_key <= 11))
);

CREATE INDEX IF NOT EXISTS idx_song_features_song_id
    ON song_features(song_id);

COMMENT ON TABLE  song_features                 IS 'Spotify audio features for tracks with a known Spotify ID';
COMMENT ON COLUMN song_features.valence         IS 'Musical positiveness: 0.0 (sad/negative) to 1.0 (happy/positive)';
COMMENT ON COLUMN song_features.musical_key     IS 'Spotify Pitch Class: 0=C 1=C# 2=D 3=D# 4=E 5=F 6=F# 7=G 8=G# 9=A 10=A# 11=B';
COMMENT ON COLUMN song_features.tempo           IS 'Estimated tempo in BPM';
COMMENT ON COLUMN song_features.loudness        IS 'Overall loudness in dB, typically between -60 and 0';
COMMENT ON COLUMN song_features.anchor_time     IS 'DO NOT ADD — anchor_time lives in fingerprints table not here';