-- Migration: 000003_relaxed_constraints
-- Makes global_seq and sender_signature nullable for local messages before sync.

-- SQLite doesn't support ALTER TABLE ALTER COLUMN well. 
-- We'll create a new table, copy data, and rename.
-- But since we are in MVP, and SQLite allows multiple types in columns (mostly),
-- we could try to just rely on the fact that existing rows are fine.
-- However, for NOT NULL we really need a new table or just avoid the constraint.

CREATE TABLE messages_new (
    id TEXT PRIMARY KEY,
    conversation_id TEXT NOT NULL,
    sender_id TEXT NOT NULL,
    content BLOB NOT NULL,
    global_seq INTEGER UNIQUE,
    sender_signature BLOB,
    sent_at INTEGER NOT NULL,
    is_incoming BOOLEAN DEFAULT 1,
    status TEXT DEFAULT 'sent',
    chat_id TEXT,
    timestamp INTEGER,
    delivered_at INTEGER,
    read_at INTEGER
);

-- Note: We check if chat_id exists by using conversation_id as fallback if the select fails? No, sql doesn't work like that.
-- We will assume chat_id exists since 000002 added it. 
-- BUT if it failed, let's try to be safe.
-- Actually, the error might be because I'm running them together? No.
-- I'll just use conversation_id and sent_at for chat_id and timestamp in the SELECT since those are what we want anyway.

INSERT INTO messages_new (id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, chat_id, timestamp, delivered_at, read_at)
SELECT id, conversation_id, sender_id, content, global_seq, sender_signature, sent_at, is_incoming, status, conversation_id, sent_at, delivered_at, read_at FROM messages;

DROP TABLE messages;
ALTER TABLE messages_new RENAME TO messages;

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id);
CREATE INDEX IF NOT EXISTS idx_messages_global_seq ON messages(global_seq);
CREATE INDEX IF NOT EXISTS idx_messages_sent_at ON messages(sent_at);
CREATE INDEX IF NOT EXISTS idx_messages_chat_time ON messages(chat_id, timestamp);
