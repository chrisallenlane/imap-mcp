package client

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew(t *testing.T) {
	c := New("https://api.example.com")

	if c.BaseURL != "https://api.example.com" {
		t.Errorf(
			"Expected BaseURL to be https://api.example.com, got %s",
			c.BaseURL,
		)
	}

	if c.HTTPClient == nil {
		t.Error("Expected HTTPClient to be set")
	}
}

func TestGet(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "GET" {
				t.Errorf("Expected GET, got %s", r.Method)
			}
			if r.URL.Path != "/test" {
				t.Errorf("Expected /test, got %s", r.URL.Path)
			}

			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"data": "success"}`))
		}),
	)
	defer server.Close()

	c := NewWithHTTPClient(server.URL, server.Client())

	resp, err := c.Get(context.Background(), "/test")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestPost(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "POST" {
				t.Errorf("Expected POST, got %s", r.Method)
			}

			contentType := r.Header.Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf(
					"Expected Content-Type application/json, got %s",
					contentType,
				)
			}

			w.WriteHeader(http.StatusCreated)
			w.Write([]byte(`{"id": 1}`))
		}),
	)
	defer server.Close()

	c := NewWithHTTPClient(server.URL, server.Client())

	resp, err := c.Post(
		context.Background(),
		"/test",
		map[string]string{"key": "value"},
	)
	if err != nil {
		t.Fatalf("Post failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("Expected 201, got %d", resp.StatusCode)
	}
}

func TestPut(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "PUT" {
				t.Errorf("Expected PUT, got %s", r.Method)
			}

			w.WriteHeader(http.StatusOK)
		}),
	)
	defer server.Close()

	c := NewWithHTTPClient(server.URL, server.Client())

	resp, err := c.Put(
		context.Background(),
		"/test/1",
		map[string]string{"key": "updated"},
	)
	if err != nil {
		t.Fatalf("Put failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200, got %d", resp.StatusCode)
	}
}

func TestDelete(t *testing.T) {
	server := httptest.NewServer(
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != "DELETE" {
				t.Errorf("Expected DELETE, got %s", r.Method)
			}

			w.WriteHeader(http.StatusNoContent)
		}),
	)
	defer server.Close()

	c := NewWithHTTPClient(server.URL, server.Client())

	resp, err := c.Delete(context.Background(), "/test/1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("Expected 204, got %d", resp.StatusCode)
	}
}
