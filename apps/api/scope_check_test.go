package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestCheckMunicipalityScope_Bypass_Standalone(t *testing.T) {
	// Setup
	gin.SetMode(gin.TestMode)
	app := &App{}

	// Case 1: Operator with NO municipality (simulating the vulnerability)
	// Theoretically this user should NOT be able to access a report from Amsterdam
	// But current logic might allow it if checkMunicipalityScope returns nil when session.Municipality is nil
	t.Run("MunicipalityOperator_WithNilMunicipality_ShouldBeDenied", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		// Simulate authenticated session
		// Role is municipality_operator, but Municipality is nil (the bug state)
		session := OperatorSession{
			Email:        "bad_actor@operator.com",
			Role:         "municipality_operator",
			Municipality: nil,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"

		// The function under test
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		// ASSERTION: We EXPECT this to fail (return error)
		// If it returns nil, the vulnerability exists
		if err == nil {
			t.Fatal("Vulnerability confirmed: municipality_operator with nil municipality bypassed scope check!")
		}

		// We expect a 403 Forbidden error
		apiErr, ok := err.(*apiError)
		assert.True(t, ok, "Expected apiError")
		assert.Equal(t, http.StatusForbidden, apiErr.Status)
	})

	// Case 2: Happy path - Admin should bypass
	t.Run("Admin_ShouldBypass", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		session := OperatorSession{
			Email:        "admin@zwerffiets.org",
			Role:         "admin",
			Municipality: nil, // Admins don't need a municipality
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		assert.NoError(t, err, "Admin should be allowed to access any municipality")
	})

	// Case 3: Happy path - Matching Municipality
	t.Run("MatchingMunicipality_ShouldAllow", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		muni := "Amsterdam"
		session := OperatorSession{
			Email:        "amsterdam@operator.com",
			Role:         "municipality_operator",
			Municipality: &muni,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)
		assert.NoError(t, err)
	})

	// Case 4: Mismatching Municipality
	t.Run("MismatchingMunicipality_ShouldDeny", func(t *testing.T) {
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)

		muni := "Rotterdam"
		session := OperatorSession{
			Email:        "rotterdam@operator.com",
			Role:         "municipality_operator",
			Municipality: &muni,
		}
		c.Set("operatorSession", session)

		reportMunicipality := "Amsterdam"
		err := app.checkMunicipalityScope(c, &reportMunicipality)

		assert.Error(t, err)
		apiErr, ok := err.(*apiError)
		assert.True(t, ok)
		assert.Equal(t, http.StatusForbidden, apiErr.Status)
	})
}
