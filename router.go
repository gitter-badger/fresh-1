package fresh

import (
	"log"
	"net/http"
	"reflect"
	"runtime"
	"strings"
	"path/filepath"
)

// Handler struct
type (
	Handler interface {
		After(...HandlerFunc) Handler
		Before(...HandlerFunc) Handler
	}
	handler struct {
		method string
		ctrl   HandlerFunc
		before []HandlerFunc
		after  []HandlerFunc
	}
)

// Route struct
type route struct {
	path     	string
	handlers 	[]*handler
	parent   	 *route
	children 	[]*route
	after    	[]HandlerFunc
	before   	[]HandlerFunc
	parameter	bool
}

// Router struct
type router struct {
	route *route
	context *context
}

// Resource struct
type (
	Resource interface {
		After(...HandlerFunc) Resource
		Before(...HandlerFunc) Resource
	}
	resource struct {
		methods []string
		rest    []Handler
	}
)


// isURLParameter check if given string is a param
func isURLParameter(value string) bool {
	if strings.HasPrefix(value, ":"){
		return true
	}
	return false
}

// TODO remove
func getFuncName(f interface{}) string {
	path := runtime.FuncForPC(reflect.ValueOf(f).Pointer()).Name()
	name := strings.Split(path, "/")
	return name[len(name)-1]
}

// Print the list of routes
func (r *router) printRoutes() {
	var tree func(routes []*route, parentPath string) error
	tree = func(routes []*route, parentPath string) error {
		for _, route := range routes {
			separator := ""
			if strings.HasSuffix(parentPath, "/") == false {
				separator = "/"
			}
			currentPath := parentPath + separator + route.path

			for _, handler := range route.handlers {
				log.Println(
					handler.method,
					currentPath,
					getFuncName(handler.ctrl),
					len(handler.before),
					len(handler.after),
				)
			}
			tree(route.children, currentPath)
		}
		return nil
	}
	tree([] *route{r.route}, "")
}

// Run a middleware
func (h *handler) middleware(c Context, handlers ...HandlerFunc) error {
	for _, f := range handlers {
		if f != nil {
			if err := f(c); err != nil {
				return err
			}
		}
	}
	return nil
}

// Add handlers to a route
func (r *route) add(method string, controller HandlerFunc, middleware ...HandlerFunc) Handler {
	// If already exist an entry for the method change related handler
	for _, h := range r.handlers {
		if h.method == method {
			h.ctrl = controller
			return h
		}
	}
	h := handler{method: method, ctrl: controller}
	r.handlers = append(r.handlers, &h)
	return &h
}

func (r* route) getHandler(method string) *handler {
	for _, h := range r.handlers {
		if h.method == method {
			return h
		}
	}
	return nil
}


// Register a route with its handlers
func (r *router) register(method string, path string, group *route, handler HandlerFunc) Handler {



	//if group != nil {
	//	// route middleware after group middleware
	//	path = group.fullPath + path
	//	response := r.scan(nil, method, path, handler)
	//	if group.after != nil {
	//		response.After(group.after...)
	//	}
	//	if group.before != nil {
	//		response.Before(group.before...)
	//	}
	//	return response
	//}
	//if group != nil {
		// route middleware after group middleware
	//	path = filepath.Join(group.path, path)
	//}

	// TODO: manage groups routes
	splittedPath := strings.Split(strings.Trim(path, "/"), "/")
	new := r.scanTree(r.route, splittedPath, true)
	new.add(method, handler)
	return nil
}

// Process a request
func (r *router) process(handler *handler, response http.ResponseWriter, request *http.Request) (err error){
	context := r.context
	context.init(request, response)
	if err = handler.middleware(context, handler.before...); err != nil {
		return err
	}
	// route controller
	err = handler.ctrl(context)
	if err != nil {
		return err
	}
	// after middleware
	if err = handler.middleware(context, handler.after...); err != nil {
		return err
	}
	// write response
	context.response.write()
	return
}

// After middleware for a single route
func (h *handler) After(middleware ...HandlerFunc) Handler {
	if middleware != nil {
		h.after = append(h.after, middleware...)
	}
	return h
}

// Before middleware for a single route
func (h *handler) Before(middleware ...HandlerFunc) Handler {
	if middleware != nil {
		h.before = append(h.before, middleware...)
	}
	return h
}

// After middleware for a resource
func (r *resource) After(middleware ...HandlerFunc) Resource {
	if middleware != nil {
		for _, route := range r.rest {
			route.After(middleware...)
		}
	}
	return r
}

// Before middleware for a resource
func (r *resource) Before(middleware ...HandlerFunc) Resource {
	if middleware != nil {
		for _, route := range r.rest {
			route.Before(middleware...)
		}
	}
	return r
}

// Router main function. Find the matching route and call registered handlers.
func (r *router) ServeHTTP(response http.ResponseWriter, request *http.Request) {
	r.context.parameters = make(map[string] string)
	splittedPath := strings.Split(strings.Trim(request.URL.Path, "/"), "/")
	if route := r.scanTree(r.route,  splittedPath, false); route != nil {
		if routeHandler := route.getHandler(request.Method); routeHandler != nil {
			r.process(routeHandler, response, request)
		} else {
			response.WriteHeader(http.StatusNotFound)
		}
	} else {
		response.WriteHeader(http.StatusNotFound)
	}
}

// Return the route and create branch if not exist
func (r *router) scanTree(parent *route, path []string, save bool) *route{
	if len(path) > 0 {
		for _,route := range parent.children {
			if route.path == path[0] {
				return r.scanTree(route, path[1:], save)
			}
			if !save && route.parameter {
				r.context.parameters[route.path[1:]] = path[0]
				return r.scanTree(route, path[1:], save)
			}
		}
		if !save {
			if parent.children[len(parent.children) - 1].parameter {
				return parent.children[len(parent.children) - 1]
			} else {
				return nil
			}
		}
		new := &route{path:path[0], parent:parent}
		switch {
		case isURLParameter(path[0]):
			new.parameter = true
			parent.children = append(parent.children, new)
		default:
			parent.children = append([] *route{new}, parent.children...)
		}
		return r.scanTree(new,path[1:], save)
	}
	return parent
}