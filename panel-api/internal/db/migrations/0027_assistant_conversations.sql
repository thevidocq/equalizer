-- The assistant's conversations, private to the person who had them: no
-- other user, administrator or platform operator reads them. Each is about
-- one site. They are dropped after the tenant's retention (days, Settings →
-- AI, 30 by default), counted from the last message.

CREATE TABLE assistant_conversations (
    id         text PRIMARY KEY,
    user_id    bigint NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    app_id     text NOT NULL REFERENCES apps (id) ON DELETE CASCADE,
    title      text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assistant_conversations_user ON assistant_conversations (user_id, updated_at DESC);

CREATE TABLE assistant_messages (
    id              bigserial PRIMARY KEY,
    conversation_id text NOT NULL REFERENCES assistant_conversations (id) ON DELETE CASCADE,
    role            text NOT NULL CHECK (role IN ('user', 'assistant')),
    text            text NOT NULL,
    cards           jsonb NOT NULL DEFAULT '[]',
    proposals       jsonb NOT NULL DEFAULT '[]',
    created_at      timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX assistant_messages_conversation ON assistant_messages (conversation_id, id);

ALTER TABLE tenants ADD COLUMN assistant_keep_days int NOT NULL DEFAULT 30
    CHECK (assistant_keep_days BETWEEN 1 AND 365);
