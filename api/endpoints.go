package api

import "net/http"

type Endpoint struct {
	Pattern string
	Handler http.HandlerFunc
}

func (h *HTTPServer) Endpoints() []Endpoint {
	return []Endpoint{
		{
			Pattern: "/api/v1/system/health",
			Handler: h.SysHealth,
		},
		{
			Pattern: "/api/v1/system/health/latest",
			Handler: h.LatestSysHealth,
		},
	}
}
