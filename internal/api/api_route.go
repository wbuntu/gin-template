package api

import (
	"net/http"

	"gitbub.com/wbuntu/gin-template/internal/api/cluster"
	"gitbub.com/wbuntu/gin-template/internal/api/tools"
	"gitbub.com/wbuntu/gin-template/internal/model"
)

func getRoutes() []model.Route {
	routes := []model.Route{}
	routes = append(routes, clusterRoute...)
	routes = append(routes, toolsRoute...)
	return routes
}

var clusterRoute = []model.Route{
	// cluster
	{Method: http.MethodPost, Path: "/clusters", Factory: func() model.Controller { return new(cluster.CreateClusterCtrl) }},
	{Method: http.MethodDelete, Path: "/clusters/:clusterId", Factory: func() model.Controller { return new(cluster.DeleteClusterCtrl) }},
	{Method: http.MethodGet, Path: "/clusters/:clusterId", Factory: func() model.Controller { return new(cluster.GetClusterCtrl) }},
	{Method: http.MethodGet, Path: "/clusters", Factory: func() model.Controller { return new(cluster.ListClusterCtrl) }},
}

var toolsRoute = []model.Route{
	// utils
	{Method: http.MethodPost, Path: "/tools/check-cidr", Factory: func() model.Controller { return new(tools.CheckCIDRCtrl) }},
}
