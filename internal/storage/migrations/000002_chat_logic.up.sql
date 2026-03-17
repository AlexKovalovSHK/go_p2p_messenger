-- Migration: 000002_chat_logic
-- Adds is_incoming, status, last_seen and aligns with 02_DES_chat_logic.md

ALTER TABLE messages ADD COLUMN is_incoming BOOLEAN DEFAULT 1;
ALTER TABLE messages ADD COLUMN status TEXT DEFAULT 'sent';
ALTER TABLE messages ADD COLUMN chat_id TEXT;
ALTER TABLE messages ADD COLUMN timestamp INTEGER;

-- Copy data from old columns to new ones for backward compatibility
UPDATE messages SET chat_id = conversation_id, timestamp = sent_at;

ALTER TABLE contacts ADD COLUMN last_seen INTEGER DEFAULT 0;
ALTER TABLE contacts ADD COLUMN is_trusted BOOLEAN DEFAULT 0;

-- Create indexes for performance
CREATE INDEX IF NOT EXISTS idx_messages_chat_time ON messages(chat_id, timestamp);
