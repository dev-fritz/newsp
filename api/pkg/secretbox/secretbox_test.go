package secretbox

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSealOpen(t *testing.T) {
	t.Parallel()
	box, err := New(bytes.Repeat([]byte{7}, KeySize))
	require.NoError(t, err)

	a, err := box.Seal([]byte("sk-secret"))
	require.NoError(t, err)
	b, err := box.Seal([]byte("sk-secret"))
	require.NoError(t, err)
	assert.NotEqual(t, a, b, "nonces are random")
	assert.NotContains(t, string(a), "sk-secret")

	plain, err := box.Open(a)
	require.NoError(t, err)
	assert.Equal(t, "sk-secret", string(plain))

	a[len(a)-1] ^= 1
	_, err = box.Open(a)
	assert.ErrorIs(t, err, ErrDecrypt, "tampering is detected")

	other, _ := New(bytes.Repeat([]byte{8}, KeySize))
	_, err = other.Open(b)
	assert.ErrorIs(t, err, ErrDecrypt, "a different key cannot open it")

	_, err = New([]byte("short"))
	assert.Error(t, err)
}
