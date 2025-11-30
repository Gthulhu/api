package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAngron(t *testing.T) {
	password := "your-password-here"

	hash, err := CreateArgon2Hash(password)
	require.NoError(t, err)

	t.Log(string(hash))

	ok, err := ComparePasswordAndHash(password, hash)
	require.NoError(t, err)
	assert.True(t, ok, "Password should match the hash")

	wrongPassword := "wrong_password"
	ok, err = ComparePasswordAndHash(wrongPassword, hash)
	require.NoError(t, err)
	assert.False(t, ok, "Wrong password should not match the hash")
}
