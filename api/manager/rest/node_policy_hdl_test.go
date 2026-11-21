package rest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Gthulhu/api/manager/domain"
	"github.com/Gthulhu/api/manager/service"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagerNodePolicyPreviewInvalidRegexReturnsWarning(t *testing.T) {
	for _, pattern := range []string{"", "("} {
		t.Run("pattern="+pattern, func(t *testing.T) {
			h := &Handler{Svc: &service.Service{}}
			req := httptest.NewRequest(http.MethodPost, "/api/v1/node-scheduling-policies/preview",
				strings.NewReader(`{"policy":{"nodeNames":["node-a"],"commandRegex":`+`"`+pattern+`"}}`))
			req = req.WithContext(h.SetClaimsInContext(req.Context(), domain.Claims{}))
			w := httptest.NewRecorder()
			h.PreviewNodeSchedulingPolicy(w, req)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			var result SuccessResponse[PreviewNodeSchedulingPolicyResponse]
			require.NoError(t, json.NewDecoder(w.Body).Decode(&result))
			require.True(t, result.Success)
			require.NotNil(t, result.Data.Policy)
			assert.Equal(t, pattern, result.Data.Policy.CommandRegex)
			assert.Equal(t, []string{"node-a"}, result.Data.Policy.NodeNames)
			assert.Empty(t, result.Data.MatchedTasks)
			require.Len(t, result.Data.Warnings, 1)
			assert.Equal(t, "invalidRegex", result.Data.Warnings[0].Code)
		})
	}
}

func TestManagerNodePolicyPreviewRouteRequiresAuthentication(t *testing.T) {
	h := &Handler{}
	e := echo.New()
	h.SetupRoutes(e)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/node-scheduling-policies/preview", strings.NewReader(`{}`))
	w := httptest.NewRecorder()
	e.ServeHTTP(w, req)
	assert.Equal(t, http.StatusUnauthorized, w.Code, w.Body.String())
}
