package managers

import (
	"fmt"
	"time"

	"github.com/gocql/gocql"
)

type CassandraClient interface {
	SaveMessage(conversationID, messageID, user, body string, createdAt time.Time) error
	Close()
}

// Client wraps a Cassandra session and provides message persistence.
type cassandraClient struct {
	session *gocql.Session
}

// NewClient connects to Cassandra, creates the keyspace/table if needed,
// and returns a ready-to-use Client.
func NewCassandraClient(hosts ...string) (CassandraClient, error) {
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

	return &cassandraClient{session: session}, nil
}

// SaveMessage persists a message to Cassandra.
func (c *cassandraClient) SaveMessage(conversationID, messageID, user, body string, createdAt time.Time) error {
	msgUUID, err := gocql.ParseUUID(messageID)
	if err != nil {
		return fmt.Errorf("cassandra: invalid message_id %q: %w", messageID, err)
	}

	return c.session.Query(`
		INSERT INTO messages (conversation_id, created_at, message_id, body, user)
		VALUES (?, ?, ?, ?, ?)
	`, conversationID, createdAt, msgUUID, body, user).Exec()
}

// Close shuts down the Cassandra session.
func (c *cassandraClient) Close() {
	c.session.Close()
}
