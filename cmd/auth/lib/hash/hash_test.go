package hash

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPasswordHash(suite *testing.T) {
	tests := []struct {
		name           string
		setPassword    string
		signinPassword string
		expectMatch    bool
	}{
		{
			name:           "verifies_password",
			setPassword:    "password",
			signinPassword: "password",
			expectMatch:    true,
		}, {
			name:           "verifies_password",
			setPassword:    "password",
			signinPassword: "wrong,",
			expectMatch:    false,
		},
	}
	for _, test := range tests {
		suite.Run(test.name, func(t *testing.T) {
			salt, err := GenerateSalt(64)
			assert.NoError(t, err)

			hashed, err := Hash(test.setPassword, salt)
			assert.NoError(t, err)

			matched := CheckPasswordHash(test.signinPassword, salt, hashed)
			assert.Equal(t, test.expectMatch, matched)
		})
	}
}
