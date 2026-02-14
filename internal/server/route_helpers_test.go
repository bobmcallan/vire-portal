package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRouteByMethod_MatchingMethod(t *testing.T) {
	called := false
	routes := MethodRouter{
		"GET": func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		},
	}

	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	RouteByMethod(w, req, routes)

	if !called {
		t.Error("expected GET handler to be called")
	}
}

func TestRouteByMethod_NoMatchingMethod(t *testing.T) {
	routes := MethodRouter{
		"GET": func(w http.ResponseWriter, r *http.Request) {
			t.Error("GET handler should not be called")
		},
	}

	req := httptest.NewRequest("POST", "/test", nil)
	w := httptest.NewRecorder()

	RouteByMethod(w, req, routes)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("expected status 405, got %d", w.Code)
	}
}

func TestRouteResourceCollection_GET(t *testing.T) {
	getCalled := false
	list := func(w http.ResponseWriter, r *http.Request) {
		getCalled = true
	}

	req := httptest.NewRequest("GET", "/items", nil)
	w := httptest.NewRecorder()

	RouteResourceCollection(w, req, list, nil)

	if !getCalled {
		t.Error("expected list handler to be called for GET")
	}
}

func TestRouteResourceCollection_POST(t *testing.T) {
	postCalled := false
	create := func(w http.ResponseWriter, r *http.Request) {
		postCalled = true
	}

	req := httptest.NewRequest("POST", "/items", nil)
	w := httptest.NewRecorder()

	RouteResourceCollection(w, req, nil, create)

	if !postCalled {
		t.Error("expected create handler to be called for POST")
	}
}

func TestRouteResourceItem_GET(t *testing.T) {
	getCalled := false
	get := func(w http.ResponseWriter, r *http.Request) {
		getCalled = true
	}

	req := httptest.NewRequest("GET", "/items/1", nil)
	w := httptest.NewRecorder()

	RouteResourceItem(w, req, get, nil, nil)

	if !getCalled {
		t.Error("expected get handler to be called")
	}
}

func TestRouteResourceItem_DELETE(t *testing.T) {
	delCalled := false
	del := func(w http.ResponseWriter, r *http.Request) {
		delCalled = true
	}

	req := httptest.NewRequest("DELETE", "/items/1", nil)
	w := httptest.NewRecorder()

	RouteResourceItem(w, req, nil, nil, del)

	if !delCalled {
		t.Error("expected delete handler to be called")
	}
}
