package openapiimport

import (
	"testing"
)

func TestParseSimpleOpenAPI(t *testing.T) {
	doc := `{
		"openapi": "3.0.0",
		"info": {"title": "Test", "version": "1.0"},
		"servers": [{"url": "https://api.example.com/v1"}],
		"paths": {
			"/users": {
				"get": {
					"operationId": "listUsers",
					"summary": "List all users",
					"parameters": [
						{"name": "limit", "in": "query", "example": 10}
					],
					"responses": {"200": {"description": "OK"}}
				},
				"post": {
					"operationId": "createUser",
					"requestBody": {
						"content": {
							"application/json": {
								"example": {"name": "Alice", "age": 30}
							}
						}
					},
					"responses": {"201": {"description": "Created"}}
				}
			},
			"/users/{id}": {
				"get": {
					"operationId": "getUser",
					"parameters": [
						{"name": "id", "in": "path"}
					],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`

	specs, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}

	if len(specs) != 3 {
		t.Fatalf("got %d specs, want 3", len(specs))
	}

	// Find listUsers.
	var listUsers, createUser, getUser *struct {
		Name   string
		Method string
		URL    string
	}
	for i := range specs {
		s := specs[i]
		switch s.Name {
		case "listUsers":
			listUsers = &struct {
				Name   string
				Method string
				URL    string
			}{s.Name, s.Method, s.URL}
		case "createUser":
			createUser = &struct {
				Name   string
				Method string
				URL    string
			}{s.Name, s.Method, s.URL}
		case "getUser":
			getUser = &struct {
				Name   string
				Method string
				URL    string
			}{s.Name, s.Method, s.URL}
		}
	}

	if listUsers == nil || listUsers.Method != "GET" || listUsers.URL != "https://api.example.com/v1/users" {
		t.Errorf("listUsers incorrect: %+v", listUsers)
	}
	if createUser == nil || createUser.Method != "POST" {
		t.Errorf("createUser incorrect: %+v", createUser)
	}
	if getUser == nil || getUser.URL != "https://api.example.com/v1/users/${ID}" {
		t.Errorf("getUser URL = %q, expected path interpolation", getUser.URL)
	}
}

func TestParseQueryParameter(t *testing.T) {
	doc := `{
		"openapi": "3.0.0",
		"paths": {
			"/search": {
				"get": {
					"operationId": "search",
					"parameters": [
						{"name": "q", "in": "query", "example": "hello"}
					],
					"responses": {"200": {"description": "OK"}}
				}
			}
		}
	}`

	specs, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Query["q"] != "hello" {
		t.Errorf("query q = %q", specs[0].Query["q"])
	}
}

func TestParseRequestBodyFromSchema(t *testing.T) {
	doc := `{
		"openapi": "3.0.0",
		"paths": {
			"/posts": {
				"post": {
					"operationId": "createPost",
					"requestBody": {
						"content": {
							"application/json": {
								"schema": {
									"type": "object",
									"properties": {
										"title": {"type": "string"},
										"views": {"type": "integer"}
									}
								}
							}
						}
					},
					"responses": {"201": {"description": "Created"}}
				}
			}
		}
	}`

	specs, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if specs[0].Body == nil || specs[0].Body.Type != "json" {
		t.Fatal("expected JSON body generated from schema")
	}
	body := string(specs[0].Body.Content)
	if body == "" {
		t.Error("body content is empty")
	}
}

func TestParseNotAnOpenAPIDoc(t *testing.T) {
	_, err := Parse([]byte(`{"foo":"bar"}`))
	if err == nil {
		t.Error("expected error for non-OpenAPI doc")
	}
}

func TestParseStatusAssertionFromResponse(t *testing.T) {
	doc := `{
		"openapi": "3.0.0",
		"paths": {
			"/created": {
				"post": {
					"operationId": "create",
					"responses": {"201": {"description": "Created"}}
				}
			}
		}
	}`

	specs, err := Parse([]byte(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(specs[0].Assertions) != 1 || specs[0].Assertions[0].Equals != 201 {
		t.Errorf("expected status 201 assertion, got %+v", specs[0].Assertions)
	}
}
