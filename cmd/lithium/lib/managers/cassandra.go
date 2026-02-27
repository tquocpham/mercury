package managers

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

// Client wraps a Cassandra session and provides message persistence.
type CassandraClient struct {
	session *gocql.Session
}

// NewClient connects to Cassandra, creates the keyspace/table if needed,
// and returns a ready-to-use Client.
func NewCassandraClient(hosts ...string) (*CassandraClient, error) {
	cluster := gocql.NewCluster(hosts...)
	cluster.Consistency = gocql.Quorum
	cluster.Timeout = 5 * time.Second

	// Connect without a keyspace first so we can create it.
	cluster.Keyspace = ""
	session, err := cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("cassandra: connect: %w", err)
	}

	if err := session.Query(`
		CREATE KEYSPACE IF NOT EXISTS lithium
		WITH replication = {'class': 'SimpleStrategy', 'replication_factor': 1}
	`).Exec(); err != nil {
		session.Close()
		return nil, fmt.Errorf("cassandra: create keyspace: %w", err)
	}

	if err := session.Query(`
		CREATE TABLE IF NOT EXISTS lithium.messages (
			conversation_id text,
			created_at      timestamp,
			message_id      uuid,
			body            text,
			user            text,
			PRIMARY KEY (conversation_id, created_at, message_id)
		) WITH CLUSTERING ORDER BY (created_at DESC)
	`).Exec(); err != nil {
		session.Close()
		return nil, fmt.Errorf("cassandra: create table: %w", err)
	}

	session.Close()

	// Reconnect scoped to the keyspace.
	cluster.Keyspace = "lithium"
	session, err = cluster.CreateSession()
	if err != nil {
		return nil, fmt.Errorf("cassandra: connect to keyspace: %w", err)
	}

	return &CassandraClient{session: session}, nil
}

// SaveMessage persists a message to Cassandra.
func (c *CassandraClient) SaveMessage(conversationID, messageID, user, body string, createdAt time.Time) error {
	msgUUID, err := gocql.ParseUUID(messageID)
	if err != nil {
		return fmt.Errorf("cassandra: invalid message_id %q: %w", messageID, err)
	}

	return c.session.Query(`
		INSERT INTO messages (conversation_id, created_at, message_id, body, user)
		VALUES (?, ?, ?, ?, ?)
	`, conversationID, createdAt, msgUUID, body, user).Exec()
}

// Message is a row from the messages table.
type Message struct {
	ConversationID string    `json:"conversation_id"`
	CreatedAt      time.Time `json:"created_at"`
	MessageID      string    `json:"message_id"`
	Body           string    `json:"body"`
	User           string    `json:"user"`
}

type GetMessagesResponse struct {
	Messages []Message
	Next     []byte
}

func (c *CassandraClient) RefreshMessages(conversationID string, messageID string) (*GetMessagesResponse, error) {
	query := c.session.Query(`
		SELECT conversation_id, created_at, message_id, body, user
		FROM messages
		WHERE conversation_id = ?
	`, conversationID)

	iter := query.Iter()

	var m Message
	var msgUUID gocql.UUID

	resp := &GetMessagesResponse{}

	for iter.Scan(&m.ConversationID, &m.CreatedAt, &msgUUID, &m.Body, &m.User) {
		m.MessageID = msgUUID.String()
		if m.MessageID == messageID {
			break
		}
		resp.Messages = append(resp.Messages, m)
	}

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("cassandra: query messages: %w", err)
	}

	return resp, nil
}

// GetMessages returns all messages for a conversation, newest first.
func (c *CassandraClient) GetMessages(conversationID string, pageSize int, nextToken []byte) (*GetMessagesResponse, error) {
	query := c.session.Query(`
		SELECT conversation_id, created_at, message_id, body, user
		FROM messages
		WHERE conversation_id = ?
	`, conversationID).PageSize(pageSize)

	if nextToken != nil {
		query = query.PageState(nextToken)
	}

	iter := query.Iter()

	var m Message
	var msgUUID gocql.UUID

	resp := &GetMessagesResponse{}

	for i := 0; i < iter.NumRows(); i++ {
		if iter.Scan(&m.ConversationID, &m.CreatedAt, &msgUUID, &m.Body, &m.User) {
			m.MessageID = msgUUID.String()
			resp.Messages = append(resp.Messages, m)
			// if message id == mmessage id in method parameter break;
		}
	}

	// Get the token for the next page
	nextPageState := iter.PageState()
	resp.Next = nextPageState

	if err := iter.Close(); err != nil {
		return nil, fmt.Errorf("cassandra: query messages: %w", err)
	}

	return resp, nil
}

// Close shuts down the Cassandra session.
func (c *CassandraClient) Close() {
	c.session.Close()
}
