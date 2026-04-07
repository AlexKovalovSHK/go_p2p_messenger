-- Migration: 000004_relax_contact_pubkey
-- Makes public_key nullable in contacts table to allow adding contacts by PeerID only.

CREATE TABLE contacts_new (
    peer_id TEXT PRIMARY KEY,
    public_key BLOB,
    multiaddr TEXT,
    alias TEXT,
    trusted BOOLEAN NOT NULL DEFAULT 0,
    last_seen INTEGER,
    created_at INTEGER NOT NULL
);

INSERT INTO contacts_new (peer_id, public_key, multiaddr, alias, trusted, last_seen, created_at)
SELECT peer_id, public_key, multiaddr, alias, trusted, last_seen, created_at FROM contacts;

DROP TABLE contacts;
ALTER TABLE contacts_new RENAME TO contacts;
