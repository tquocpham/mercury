package ids

import (
	"crypto/rand"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
)

// Global entropy source to ensure monotonicity within a single process
var (
	entropy *ulid.MonotonicEntropy
	mutex   sync.Mutex
)

func init() {
	// Seed the entropy source with crypto/rand
	entropy = ulid.Monotonic(rand.Reader, 0)
}

// ValidateOrderID returns true if s is a valid ULID string.
func ValidateOrderID(s string) bool {
	_, err := ulid.ParseStrict(s)
	return err == nil
}

// NewOrderID generates a sortable, unique 26-character string
func NewOrderID() string {
	mutex.Lock()
	defer mutex.Unlock()

	// Generate ULID using the current time and our secure entropy source
	t := time.Now()
	id, err := ulid.New(ulid.Timestamp(t), entropy)
	if err != nil {
		// This only happens if entropy fails (extremely rare with crypto/rand)
		panic(err)
	}

	return id.String()
}
