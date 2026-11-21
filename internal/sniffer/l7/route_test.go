package l7

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNormalizeRoute(t *testing.T) {
	cases := map[string]string{
		"/users/123":          "/users/:id",
		"/users/123/orders/9": "/users/:id/orders/:id",
		"/files/550e8400-e29b-41d4-a716-446655440000": "/files/:id",
		"/tx/9f86d081884c7d65":                        "/tx/:id",
		"/busca?q=senha":                              "/busca",
		"/":                                           "/",
		"/v1/status":                                  "/v1/status",
		"/tokens/aZ3kP9qL2mN8xR4tY7wB1cD5":            "/tokens/:id",
		"/pagina#topo":                                "/pagina",
		"":                                            "/",
		"*":                                           "/",
	}
	for in, want := range cases {
		assert.Equal(t, want, NormalizeRoute(in), in)
	}
	assert.Len(t, NormalizeRoute("/"+strings.Repeat("z", 500)), maxRouteLen)
}
