package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"time"
)

// Conversation is one of a person's conversations with the assistant.
type Conversation struct {
	ID        string    `json:"id"`
	App       string    `json:"app"`
	AppName   string    `json:"app_name"`
	Tenant    string    `json:"-"`
	Title     string    `json:"title"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ConversationMessage is one message of a conversation.
type ConversationMessage struct {
	Role      string          `json:"role"`
	Text      string          `json:"text"`
	Cards     json.RawMessage `json:"cards"`
	Proposals json.RawMessage `json:"proposals"`
	CreatedAt time.Time       `json:"created_at"`
}

// keep is the condition of a conversation still within its tenant's retention.
const keep = `c.updated_at >= now() - make_interval(days => (SELECT t.assistant_keep_days FROM apps a JOIN tenants t ON t.id = a.tenant_id WHERE a.id = c.app_id))`

// AssistantKeepDays is how long the tenant keeps conversations.
func (s *Store) AssistantKeepDays(ctx context.Context, tenant string) (int, error) {
	var d int
	err := s.DB.QueryRowContext(ctx, `SELECT assistant_keep_days FROM tenants WHERE id = $1`, tenant).Scan(&d)
	return d, translate(err)
}

// SetAssistantKeepDays changes it and drops what is now older.
func (s *Store) SetAssistantKeepDays(ctx context.Context, tenant string, days int) error {
	if _, err := s.DB.ExecContext(ctx, `UPDATE tenants SET assistant_keep_days = $2 WHERE id = $1`, tenant, days); err != nil {
		return translate(err)
	}
	return s.PurgeConversations(ctx, tenant)
}

// PurgeConversations drops the tenant's conversations past its retention.
func (s *Store) PurgeConversations(ctx context.Context, tenant string) error {
	_, err := s.DB.ExecContext(ctx, `DELETE FROM assistant_conversations c USING apps a
		WHERE a.id = c.app_id AND a.tenant_id = $1 AND NOT (`+keep+`)`, tenant)
	return err
}

// PurgeAssistant drops, for every tenant, the conversations and proposals
// past its retention (hourly).
func (s *Store) PurgeAssistant(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM assistant_conversations c WHERE NOT (`+keep+`)`); err != nil {
		return err
	}
	_, err := s.DB.ExecContext(ctx, `DELETE FROM assistant_proposals p USING apps a, tenants t
		WHERE a.id = p.app_id AND t.id = a.tenant_id AND p.created_at < now() - make_interval(days => t.assistant_keep_days)`)
	return err
}

// Conversations lists a person's conversations, newest first.
func (s *Store) Conversations(ctx context.Context, user int64) ([]Conversation, error) {
	rows, err := s.DB.QueryContext(ctx, `SELECT c.id, c.app_id, a.name, a.tenant_id, c.title, c.updated_at
		FROM assistant_conversations c JOIN apps a ON a.id = c.app_id
		WHERE c.user_id = $1 AND `+keep+` ORDER BY c.updated_at DESC LIMIT 100`, user)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Conversation{}
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.App, &c.AppName, &c.Tenant, &c.Title, &c.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// ConversationOf is a person's conversation; ErrNotFound when it is someone
// else's, gone, or past retention.
func (s *Store) ConversationOf(ctx context.Context, user int64, id string) (Conversation, []ConversationMessage, error) {
	var c Conversation
	err := s.DB.QueryRowContext(ctx, `SELECT c.id, c.app_id, a.name, a.tenant_id, c.title, c.updated_at
		FROM assistant_conversations c JOIN apps a ON a.id = c.app_id
		WHERE c.id = $1 AND c.user_id = $2 AND `+keep, id, user).Scan(&c.ID, &c.App, &c.AppName, &c.Tenant, &c.Title, &c.UpdatedAt)
	if err != nil {
		return c, nil, translate(err)
	}
	rows, err := s.DB.QueryContext(ctx, `SELECT role, text, cards, proposals, created_at FROM assistant_messages
		WHERE conversation_id = $1 ORDER BY id`, id)
	if err != nil {
		return c, nil, err
	}
	defer rows.Close()
	msgs := []ConversationMessage{}
	for rows.Next() {
		var m ConversationMessage
		var cards, props []byte
		if err := rows.Scan(&m.Role, &m.Text, &cards, &props, &m.CreatedAt); err != nil {
			return c, nil, err
		}
		m.Cards, m.Proposals = cards, props
		msgs = append(msgs, m)
	}
	return c, msgs, rows.Err()
}

// StartConversation opens a conversation of a person about a site.
func (s *Store) StartConversation(ctx context.Context, user int64, app, title string) (string, error) {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	id := hex.EncodeToString(b)
	_, err := s.DB.ExecContext(ctx, `INSERT INTO assistant_conversations (id, user_id, app_id, title) VALUES ($1, $2, $3, $4)`,
		id, user, app, title)
	return id, translate(err)
}

// AddExchange records a question and its answer.
func (s *Store) AddExchange(ctx context.Context, id, question, answer string, cards, proposals json.RawMessage) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	// one exchange at a time per conversation: question and answer stay together
	var locked string
	if err := tx.QueryRowContext(ctx, `SELECT id FROM assistant_conversations WHERE id = $1 FOR UPDATE`, id).Scan(&locked); err != nil {
		return translate(err) // deleted meanwhile: ErrNotFound
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO assistant_messages (conversation_id, role, text) VALUES ($1, 'user', $2)`, id, question); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO assistant_messages (conversation_id, role, text, cards, proposals)
		VALUES ($1, 'assistant', $2, $3, $4)`, id, answer, []byte(cards), []byte(proposals)); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE assistant_conversations SET updated_at = now() WHERE id = $1`, id); err != nil {
		return err
	}
	return tx.Commit()
}

// DeleteConversation drops a person's conversation.
func (s *Store) DeleteConversation(ctx context.Context, user int64, id string) error {
	res, err := s.DB.ExecContext(ctx, `DELETE FROM assistant_conversations WHERE id = $1 AND user_id = $2`, id, user)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ProposalDrafts are the draft versions of proposals, by id (none yet: absent).
func (s *Store) ProposalDrafts(ctx context.Context, app string, ids []string) (map[string]int, error) {
	out := map[string]int{}
	if len(ids) == 0 {
		return out, nil
	}
	b, _ := json.Marshal(ids)
	rows, err := s.DB.QueryContext(ctx, `SELECT id, draft_version FROM assistant_proposals
		WHERE app_id = $1 AND id IN (SELECT jsonb_array_elements_text($2::jsonb)) AND draft_version IS NOT NULL`, app, b)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var v int
		if err := rows.Scan(&id, &v); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}
