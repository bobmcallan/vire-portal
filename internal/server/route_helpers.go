package server

import "net/http"

// RouteHandler is a function type for HTTP handlers.
type RouteHandler func(http.ResponseWriter, *http.Request)

// MethodRouter maps HTTP methods to handlers.
type MethodRouter map[string]RouteHandler

// RouteByMethod routes requests based on HTTP method.
func RouteByMethod(w http.ResponseWriter, r *http.Request, routes MethodRouter) {
	handler, ok := routes[r.Method]
	if !ok {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	handler(w, r)
}

// RouteResourceCollection handles standard list + create pattern.
// GET -> list, POST -> create.
func RouteResourceCollection(w http.ResponseWriter, r *http.Request, list, create RouteHandler) {
	routes := make(MethodRouter)
	if list != nil {
		routes["GET"] = list
	}
	if create != nil {
		routes["POST"] = create
	}
	RouteByMethod(w, r, routes)
}

// RouteResourceItem handles standard get + update + delete pattern.
// GET -> get, PUT -> update, DELETE -> delete.
func RouteResourceItem(w http.ResponseWriter, r *http.Request, get, update, del RouteHandler) {
	routes := make(MethodRouter)
	if get != nil {
		routes["GET"] = get
	}
	if update != nil {
		routes["PUT"] = update
	}
	if del != nil {
		routes["DELETE"] = del
	}
	RouteByMethod(w, r, routes)
}
