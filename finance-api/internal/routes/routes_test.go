package routes_test

import (
	"encoding/json"
	"finance-api/internal/config"
	"finance-api/internal/routes"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// Every registered route is in docs/swagger.json with the same method (run `swag init -g cmd/api/main.go` after
// changing handlers), and every protected route declares BearerAuth.
func TestSwaggerCoversAllRoutes(t *testing.T) {
	raw, err := os.ReadFile("../../docs/swagger.json")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		BasePath string                                `json:"basePath"`
		Paths    map[string]map[string]json.RawMessage `json:"paths"`
	}
	if err := json.Unmarshal(raw, &spec); err != nil {
		t.Fatal(err)
	}

	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes.Register(r, &config.Config{JWTSecret: "x"})

	param := regexp.MustCompile(`:(\w+)`)
	if len(r.Routes()) < 25 {
		t.Fatalf("only %d routes registered", len(r.Routes()))
	}
	for _, route := range r.Routes() {
		path := param.ReplaceAllString(strings.TrimPrefix(route.Path, spec.BasePath), "{$1}")
		op, ok := spec.Paths[path][strings.ToLower(route.Method)]
		if !ok {
			t.Errorf("%s %s is not in swagger.json", route.Method, path)
			continue
		}
		open := strings.HasPrefix(path, "/auth/") || strings.HasPrefix(path, "/webhooks/")
		if !open && !strings.Contains(string(op), "BearerAuth") {
			t.Errorf("%s %s needs a login but declares no BearerAuth", route.Method, path)
		}
	}
}
