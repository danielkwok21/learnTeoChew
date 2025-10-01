CREATE TABLE sqlite_sequence(name,seq);
CREATE TABLE cards (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    front TEXT NOT NULL,
    back TEXT NOT NULL,
    ease INTEGER DEFAULT NULL,  -- user rating (1=again, 2=hard, 3=good, 4=easy)
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
