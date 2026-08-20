package main

import (
	"context"
	"net/http"
	"regexp"
	"strings"
)

// route 单条路由规则
type route struct {
	method  string
	re      *regexp.Regexp
	handler http.HandlerFunc
	params  []string
}

// Router 极简路由（支持 {param} 路径参数），无第三方依赖
type Router struct {
	routes []*route
}

func NewRouter() *Router { return &Router{} }

func (r *Router) Handle(method, pattern string, h http.HandlerFunc) {
	parts := strings.Split(pattern, "/")
	var b strings.Builder
	b.WriteString("^")
	var params []string
	for _, p := range parts {
		if p == "" {
			continue
		}
		if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
			params = append(params, strings.TrimSuffix(strings.TrimPrefix(p, "{"), "}"))
			b.WriteString("/([^/]+)")
		} else {
			b.WriteString("/" + regexp.QuoteMeta(p))
		}
	}
	b.WriteString("/?$")
	r.routes = append(r.routes, &route{method, regexp.MustCompile(b.String()), h, params})
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	for _, rt := range r.routes {
		if rt.method != req.Method {
			continue
		}
		m := rt.re.FindStringSubmatch(req.URL.Path)
		if m == nil {
			continue
		}
		ctx := req.Context()
		for i, p := range rt.params {
			ctx = context.WithValue(ctx, ctxKey(p), m[i+1])
		}
		rt.handler(w, req.WithContext(ctx))
		return
	}
	http.NotFound(w, req)
}

// Param 取路径参数
func Param(req *http.Request, name string) string {
	v, _ := req.Context().Value(ctxKey(name)).(string)
	return v
}
