//go:build unit

package repository

// [mok] Fork-only test (see FORK.md): invite codes are 4 characters long in this fork.
// It fails when an upstream merge puts affiliateCodeLength back to upstream's value.

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

func TestMokGeneratedAffiliateCodesAreFourCharactersLong(t *testing.T) {
	seen := make(map[string]bool)
	for i := 0; i < 500; i++ {
		code, err := generateAffiliateCode()
		require.NoError(t, err)
		require.Len(t, code, 4)
		for _, c := range []byte(code) {
			require.True(t, bytes.IndexByte(affiliateCodeCharset, c) >= 0, "unexpected character %q in %s", c, code)
		}
		seen[code] = true
	}
	require.Greater(t, len(seen), 400, "codes are random")

	// Registration only binds codes within these bounds.
	require.LessOrEqual(t, service.AffiliateCodeMinLength, 4)
	require.GreaterOrEqual(t, service.AffiliateCodeMaxLength, 4)
}
