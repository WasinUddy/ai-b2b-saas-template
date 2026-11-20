package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/getkin/kin-openapi/openapi2"
	"github.com/getkin/kin-openapi/openapi2conv"
	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"github.com/gofiber/fiber/v2"
	"gopkg.in/yaml.v3"

	"github.com/wasinuddy/ai-b2b-saas-template/internal/model"
)

type fakeHealth struct {
	err error
}

func (f fakeHealth) Ready(context.Context) (model.HealthStatus, error) {
	if f.err != nil {
		return model.HealthStatus{}, f.err
	}
	return model.HealthStatus{Status: "ok"}, nil
}

func TestRoutesMatchSpec(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)
	app := newContractApp(fakeHealth{})
	seen := map[string]bool{}
	for _, route := range app.GetRoutes(true) {
		if route.Method == http.MethodHead || route.Path == "" {
			continue
		}
		key := route.Method + " " + route.Path
		seen[key] = true
		item := doc.Paths.Value(route.Path)
		if item == nil || item.GetOperation(route.Method) == nil {
			t.Errorf("route %s is not in the OpenAPI spec", key)
		}
	}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			key := method + " " + path
			if !seen[key] {
				t.Errorf("spec operation %s is not registered", key)
			}
		}
	}
}

func TestContractResponses(t *testing.T) {
	t.Parallel()

	doc := loadSpec(t)
	cases := []struct {
		name   string
		ready  error
		method string
		target string
		status int
	}{
		{name: "live", method: http.MethodGet, target: "/healthz", status: http.StatusOK},
		{name: "ready", method: http.MethodGet, target: "/readyz", status: http.StatusOK},
		{name: "not ready", ready: errors.New("database down"), method: http.MethodGet, target: "/readyz", status: http.StatusServiceUnavailable},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			app := newContractApp(fakeHealth{err: tc.ready})
			assertContract(t, doc, app, httptest.NewRequest(tc.method, tc.target, nil), tc.status)
		})
	}
}

func assertContract(t *testing.T, doc *openapi3.T, app *fiber.App, req *http.Request, want int) {
	t.Helper()
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatalf("router: %v", err)
	}
	res, err := app.Test(req)
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	body := readAll(t, res)
	if res.StatusCode != want {
		t.Fatalf("status = %d, want %d, body %s", res.StatusCode, want, body)
	}
	route, params, findErr := router.FindRoute(req)
	if findErr != nil {
		t.Fatalf("FindRoute(%s): %v", req.URL.Path, findErr)
	}
	if res.Header.Get(fiber.HeaderContentType) == "" {
		res.Header.Set(fiber.HeaderContentType, fiber.MIMEApplicationJSON)
	}
	validateErr := openapi3filter.ValidateResponse(context.Background(), &openapi3filter.ResponseValidationInput{
		RequestValidationInput: &openapi3filter.RequestValidationInput{
			Request:    req,
			PathParams: params,
			Route:      route,
		},
		Status: res.StatusCode,
		Header: res.Header,
		Body:   io.NopCloser(bytes.NewReader(body)),
	})
	if validateErr != nil {
		t.Fatalf("response does not match the spec: %v\nbody: %s", validateErr, body)
	}
}

func TestUnknownRoute(t *testing.T) {
	t.Parallel()

	app := newContractApp(fakeHealth{})
	res, err := app.Test(httptest.NewRequest(http.MethodGet, "/missing", nil))
	if err != nil {
		t.Fatalf("Test() error = %v", err)
	}
	body := readAll(t, res)
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body %s", res.StatusCode, body)
	}
	var payload ErrorResponse
	if decodeErr := json.Unmarshal(body, &payload); decodeErr != nil {
		t.Fatalf("decode: %v", decodeErr)
	}
	if payload.Error.Code != "NOT_FOUND" || payload.Error.Message != "not found" {
		t.Fatalf("body = %#v", payload.Error)
	}
}

func newContractApp(health fakeHealth) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		ErrorHandler:          NewErrorHandler(discardLog()),
	})
	Register(app, Dependencies{
		Health: &HealthHandler{health: health},
		Log:    discardLog(),
	})
	return app
}

func loadSpec(t *testing.T) *openapi3.T {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	raw, readErr := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "api", "swagger.yaml"))
	if readErr != nil {
		t.Fatalf("read spec: %v", readErr)
	}
	var node any
	if parseErr := yaml.Unmarshal(raw, &node); parseErr != nil {
		t.Fatalf("parse spec: %v", parseErr)
	}
	encoded, encodeErr := json.Marshal(yamlJSON(node))
	if encodeErr != nil {
		t.Fatalf("encode spec: %v", encodeErr)
	}
	var doc2 openapi2.T
	if decodeErr := json.Unmarshal(encoded, &doc2); decodeErr != nil {
		t.Fatalf("decode swagger: %v", decodeErr)
	}
	doc, convertErr := openapi2conv.ToV3(&doc2)
	if convertErr != nil {
		t.Fatalf("convert spec: %v", convertErr)
	}
	// The template does not publish a host. Match paths only.
	doc.Servers = nil
	return doc
}

func yamlJSON(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, child := range typed {
			out[key] = yamlJSON(child)
		}
		return out
	case []any:
		for i, child := range typed {
			typed[i] = yamlJSON(child)
		}
		return typed
	default:
		return value
	}
}
