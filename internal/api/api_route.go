package api

import (
	"net/http"

	"gitbub.com/wbuntu/gin-template/internal/api/cluster"
	"gitbub.com/wbuntu/gin-template/internal/api/tools"
	"gitbub.com/wbuntu/gin-template/internal/model"
)

var clusterRoute = []model.Route{
	// cluster
	{Method: http.MethodPost, Path: "/clusters", Ctrl: new(cluster.CreateClusterCtrl)},
	{Method: http.MethodDelete, Path: "/clusters/:clusterId", Ctrl: new(cluster.DeleteClusterCtrl)},
	{Method: http.MethodGet, Path: "/clusters/:clusterId", Ctrl: new(cluster.GetClusterCtrl)},
	{Method: http.MethodGet, Path: "/clusters", Ctrl: new(cluster.ListClusterCtrl)},
}

var utilsRoute = []model.Route{
	// utils
	{Method: http.MethodPost, Path: "/tools/check-cidr", Ctrl: new(tools.CheckCIDRCtrl)},
}
