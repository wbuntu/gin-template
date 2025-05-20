package cluster

import (
	"os"
	"strconv"
	"strings"
	"time"

	"gitbub.com/wbuntu/gin-template/internal/model"
	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
	"gitbub.com/wbuntu/gin-template/internal/pkg/utils"
	"gitbub.com/wbuntu/gin-template/internal/storage"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gorm.io/gorm"
)

type CreateClusterCtrl struct {
	model.BaseController[model.CreateClusterReq, model.CreateClusterResp]
}

// @Summary     创建集群
// @Description 根据配置参数自动创建集群
// @Tags        Cluster
// @Param       CreateClusterReq body     model.CreateClusterReq  true "请求"
// @Response    200              {object} model.CreateClusterResp "响应"
// @Router      /clusters [post]
func (ctrl *CreateClusterCtrl) Serve(g *gin.Context) {
	logger := log.GetLogger(g)
	// 生成集群资源ID
	resourceID, err := storage.GenerateClusterResourceID()
	if err != nil {
		logger.Errorf("generate resourceID: %s", err)
		ctrl.Response.Update(model.CodeInternalError, "generate resourceID")
		return
	}
	req := &ctrl.Request
	cluster := &storage.Cluster{
		Name:        req.Name,
		Description: req.Description,
		ResourceID:  resourceID.ResourceID,
		Type:        req.Type,
		Version:     req.Version,
		Runtime:     req.Runtime,
		Status:      storage.ClusterStatusCreating,
	}
	// 超过150分钟未Ready判定为异常
	limit := uint16(150)
	timtoutLimit := os.Getenv("TIMEOUT_LIMIT")
	if len(timtoutLimit) > 0 {
		t, _ := strconv.Atoi(timtoutLimit)
		limit = uint16(t)
	}
	task := &storage.Task{
		ResourceID: cluster.ResourceID,
		RequestID:  req.RequestID,
		Status:     storage.TaskStatusEnqueued,
		RetryType:  storage.TaskRetryTypeFixed,
		RetryDelay: 10,
		RetryCount: 0,
		RetryLimit: limit,
		Action:     storage.ClusterActionCreate,
	}
	// 在事务中创建集群、提交任务
	if err := storage.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(cluster).Error; err != nil {
			return errors.Wrap(err, "create cluster")
		}
		if err := tx.Create(task).Error; err != nil {
			return errors.Wrap(err, "create task")
		}
		return nil
	}); err != nil {
		logger.Errorf("transaction: %s", err)
		ctrl.Response.Update(model.CodeInternalError, "commit request")
		return
	}
	logger.WithFields(log.Fields{
		"clusterID": task.ResourceID,
		"taskID":    task.ID,
	}).Info("create cluster task committed")
	// 返回响应
	ctrl.Response.Data = resourceID.ResourceID
}

type DeleteClusterCtrl struct {
	model.BaseController[model.BaseRequest, model.BaseResponse]
}

// @Summary     删除集群
// @Description 根据集群ID自动删除集群
// @Tags        Cluster
// @Param       clusterId path     string             true "集群资源ID" extensions(x-example=cluster-sedqqz7ka)
// @Response    200       {object} model.BaseResponse "响应"
// @Router      /clusters/{clusterId} [delete]
func (ctrl *DeleteClusterCtrl) Serve(g *gin.Context) {
	logger := log.GetLogger(g)
	// 获取资源ID
	clusterID := g.Param("clusterId")
	// 获取集群
	cluster, err := storage.GetClusterByResourceID(clusterID)
	if err != nil {
		logger.WithField("clusterID", clusterID).Errorf("get cluster: %s", err)
		if err == storage.ErrDoesNotExist {
			ctrl.Response.Update(model.CodeNotExists, "cluster not found")
		} else {
			ctrl.Response.Update(model.CodeInternalError, "get cluster")
		}
		return
	}
	// 检查状态，稳态才可以提交任务
	if cluster.Status < storage.ClusterStatusRunning {
		logger.WithField("clusterID", clusterID).Error("cluster status pending")
		ctrl.Response.Update(model.CodeForbidOperate, "cluster status pending")
		return
	}
	// 超过30分钟未删除判定为异常
	task := &storage.Task{
		ResourceID: cluster.ResourceID,
		RequestID:  ctrl.Request.RequestID,
		Status:     storage.TaskStatusEnqueued,
		RetryType:  storage.TaskRetryTypeFixed,
		RetryDelay: 10,
		RetryCount: 0,
		RetryLimit: 10,
		Action:     storage.ClusterActionDelete,
	}
	// 在事务中更改集群状态、提交任务
	cluster.Status = storage.ClusterStatusDeleting
	if err := storage.DB().Transaction(func(tx *gorm.DB) error {
		if err := tx.Updates(cluster).Error; err != nil {
			return errors.Wrap(err, "update cluster")
		}
		if err := tx.Create(task).Error; err != nil {
			return errors.Wrap(err, "create task")
		}
		return nil
	}); err != nil {
		logger.Errorf("transaction: %s", err)
		ctrl.Response.Update(model.CodeInternalError, "commit request")
		return
	}
	logger.WithFields(log.Fields{
		"clusterID": task.ResourceID,
		"taskID":    task.ID,
	}).Info("delete cluster task committed")
}

type GetClusterCtrl struct {
	model.BaseController[model.GetClusterReq, model.GetClusterResp]
}

// @Summary     集群详情
// @Description 根据集群ID获取集群详细信息
// @Tags        Cluster
// @Param       clusterId path     string               true "集群资源ID" extensions(x-example=cluster-sedqqz7ka)
// @Response    200       {object} model.GetClusterResp "响应"
// @Router      /clusters/{clusterId} [get]
func (ctrl *GetClusterCtrl) Serve(g *gin.Context) {
	logger := log.GetLogger(g)
	// 获取资源ID
	resourceID := g.Param("clusterId")
	// 获取集群
	cluster, err := storage.GetClusterByResourceID(resourceID)
	if err != nil {
		logger.WithField("clusterID", resourceID).Errorf("get cluster: %s", err)
		if err == storage.ErrDoesNotExist {
			ctrl.Response.Update(model.CodeNotExists, "cluster not found")
		} else {
			ctrl.Response.Update(model.CodeInternalError, "get cluster")
		}
		return
	}
	ctrl.Response.Data = &model.ClusterDetail{
		Name:        cluster.Name,
		Description: cluster.Description,
		ResourceID:  cluster.ResourceID,
		Type:        cluster.Type,
		Version:     cluster.Version,
		Runtime:     cluster.Runtime,
		Status:      cluster.Status.String(),
		CreateTime:  utils.FormatTime(cluster.CreatedAt),
	}
}

type ListClusterCtrl struct {
	model.BaseController[model.ListClusterReq, model.ListClusterResp]
}

// @Summary     集群列表
// @Description 分页获取集群列表
// @Tags        Cluster
// @Param       pageNo      query    int                   true  "分页号，默认为1"   extensions(x-example=1)
// @Param       pageSize    query    int                   true  "分页大小，默认为10" extensions(x-example=10)
// @Param       filterKey   query    string                false "查询条件，默认为空"
// @Param       filterValue query    string                false "查询值，默认为空"
// @Response    200         {object} model.ListClusterResp "响应"
// @Router      /clusters [get]
func (ctrl *ListClusterCtrl) Serve(g *gin.Context) {
	logger := log.GetLogger(g)
	req := ctrl.Request
	filter, err := buildClusterFilter(req.FilterKey, req.FilterValue)
	if err != nil {
		logger.Errorf("build cluster filter: %s", err)
		ctrl.Response.Update(model.CodeParamError, "invalid filterKey or filterValue")
		return
	}

	items, err := storage.ListCluster((req.PageNo-1)*req.PageSize, req.PageSize, filter)
	if err != nil {
		logger.Errorf("list cluster: %s", err)
		ctrl.Response.Update(model.CodeInternalError, "list cluster")
		return
	}
	count, err := storage.CountCluster(filter)
	if err != nil {
		logger.Errorf("count cluster: %s", err)
		ctrl.Response.Update(model.CodeInternalError, "count cluster")
		return
	}
	ctrl.Response.Data = make([]model.ClusterSummary, 0)
	for _, cluster := range items {
		ctrl.Response.Data = append(ctrl.Response.Data, model.ClusterSummary{
			Name:        cluster.Name,
			Description: cluster.Description,
			ResourceID:  cluster.ResourceID,
			Type:        cluster.Type,
			Version:     cluster.Version,
			Runtime:     cluster.Runtime,
			Status:      cluster.Status.String(),
			CreateTime:  utils.FormatTime(cluster.CreatedAt),
		})
	}
	ctrl.Response.TotalCount = int(count)
}

func buildClusterFilter(filterKey string, filterValue string) (*storage.Filter, error) {
	if len(filterKey) == 0 {
		return nil, nil
	}
	if len(filterKey) > 0 && len(filterValue) == 0 {
		return nil, errors.New("empty filterValue")
	}
	var filter *storage.Filter
	switch filterKey {
	case "name":
		// 模糊查询
		filter = &storage.Filter{
			Query: "name LIKE ?",
			Args:  []interface{}{"%" + filterValue + "%"},
		}
	case "resourceID":
		// 文本匹配
		filter = &storage.Filter{
			Query: "resource_id = ?",
			Args:  []interface{}{filterValue},
		}
	case "type":
		// 多选
		items := strings.Split(filterValue, ",")
		if len(items) == 0 {
			return nil, errors.New("invalid type")
		}
		filter = &storage.Filter{
			Query: "type IN ?",
			Args:  []interface{}{},
		}
		filter.Args = append(filter.Args, items)
	case "version":
		// 多选
		items := strings.Split(filterValue, ",")
		if len(items) == 0 {
			return nil, errors.New("invalid version")
		}
		filter = &storage.Filter{
			Query: "version IN ?",
			Args:  []interface{}{},
		}
		filter.Args = append(filter.Args, items)
	case "status":
		// 多选
		items := strings.Split(filterValue, ",")
		if len(items) == 0 {
			return nil, errors.Errorf("invalid status: %s", filterValue)
		}
		for i := range items {
			if _, err := strconv.Atoi(items[i]); err != nil {
				return nil, errors.Errorf("invalid status: %s", filterValue)
			}
		}
		filter = &storage.Filter{
			Query: "status IN ?",
			Args:  []interface{}{},
		}
		filter.Args = append(filter.Args, items)
	case "createTime":
		// 多选
		items := strings.Split(filterValue, ",")
		if len(items) != 2 {
			return nil, errors.New("invalid createTime")
		}
		start, err := strconv.Atoi(items[0])
		if err != nil {
			return nil, errors.New("invalid createTime: startTime")
		}
		end, err := strconv.Atoi(items[1])
		if err != nil {
			return nil, errors.New("invalid createTime: endTime")
		}
		filter = &storage.Filter{
			Query: "created_at BETWEEN ? AND ?",
			Args: []interface{}{
				time.Unix(int64(start), 0),
				time.Unix(int64(end), 0),
			},
		}
	default:
		return nil, errors.New("unsupported filterKey")
	}
	return filter, nil
}
