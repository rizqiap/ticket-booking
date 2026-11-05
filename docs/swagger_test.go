package docs

import (
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/SeladaKeju/ticket-booking/internal/auth"
	"github.com/SeladaKeju/ticket-booking/internal/event"
	"github.com/SeladaKeju/ticket-booking/internal/tickettype"
	"github.com/gin-gonic/gin"
	"github.com/goccy/go-yaml"
)

func TestSwaggerRoutes(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterRoutes(router)
	for _, tt := range []struct {
		path   string
		status int
		body   string
	}{
		{"/swagger/", http.StatusOK, "SwaggerUIBundle"},
		{"/swagger/index.html", http.StatusMovedPermanently, ""},
		{"/swagger/openapi.yaml", http.StatusOK, "openapi: 3.0.3"},
		{"/swagger/REQUIREMENT.MD", http.StatusNotFound, ""},
	} {
		t.Run(tt.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, tt.path, nil))
			if response.Code != tt.status || !strings.Contains(response.Body.String(), tt.body) {
				t.Fatalf("unexpected response: %d %s", response.Code, response.Body.String())
			}
		})
	}
}

// Every implemented API operation must match a registered route in both directions.
func TestImplementedContractRoutes(t *testing.T) {
	content, err := files.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(content, &spec); err != nil {
		t.Fatal(err)
	}
	implemented := func(path, method string) bool {
		operation, ok := spec.Paths[path][method].(map[string]any)
		return ok && operation["x-implementation-status"] == "implemented"
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	api := router.Group("/api/v1")
	noop := func(c *gin.Context) { c.Next() }
	auth.RegisterRoutes(api, auth.NewHandler(nil), noop)
	event.RegisterRoutes(api, event.NewHandler(nil), noop, noop)
	tickettype.RegisterRoutes(api, tickettype.NewHandler(nil), noop, noop)
	routes := make(map[string]bool)
	for _, route := range router.Routes() {
		path := strings.ReplaceAll(strings.TrimPrefix(route.Path, "/api/v1"), ":id", "{id}")
		method := strings.ToLower(route.Method)
		routes[method+" "+path] = true
		if !implemented(path, method) {
			t.Errorf("registered route is not documented as implemented: %s %s", route.Method, path)
		}
	}
	for path, operations := range spec.Paths {
		for method := range operations {
			if implemented(path, method) && !routes[method+" "+path] {
				t.Errorf("implemented contract operation has no route: %s %s", method, path)
			}
		}
	}
}

// Keep every method/path in the requirement represented in the OpenAPI document.
func TestContractCoverage(t *testing.T) {
	content, err := files.ReadFile("openapi.yaml")
	if err != nil {
		t.Fatal(err)
	}
	var spec struct {
		Paths map[string]map[string]any `yaml:"paths"`
	}
	if err := yaml.Unmarshal(content, &spec); err != nil {
		t.Fatal(err)
	}
	requirement, err := os.ReadFile("REQUIREMENT.MD")
	if err != nil {
		t.Fatal(err)
	}
	operations := regexp.MustCompile("`(GET|POST|PATCH|PUT|DELETE) ([^`]+)`").FindAllStringSubmatch(string(requirement), -1)
	if len(operations) == 0 {
		t.Fatal("no API operations found in requirement")
	}
	for _, operation := range operations {
		if _, ok := spec.Paths[operation[2]][strings.ToLower(operation[1])]; !ok {
			t.Errorf("missing operation: %s %s", operation[1], operation[2])
		}
	}
}
