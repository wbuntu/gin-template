package tools

import (
	"context"
	"net"

	"gitbub.com/wbuntu/gin-template/internal/model"
	"gitbub.com/wbuntu/gin-template/internal/pkg/log"
	"gitbub.com/wbuntu/gin-template/internal/pkg/utils"
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type CheckCIDRCtrl struct {
	model.BaseController[model.CheckCIDRReq, model.CheckCIDRResp]
}

// @Summary     CIDR网段检查
// @Description 检查集群CIDR是否存在网段冲突
// @Tags        Tools
// @Param       CheckCIDRReq body     model.CheckCIDRReq  true "请求"
// @Response    200          {object} model.CheckCIDRResp "响应"
// @Router      /tools/check-cidr [post]
func (ctrl *CheckCIDRCtrl) Serve(g *gin.Context) {
	logger := log.GetLogger(g)
	if err := checkCIDR(g, &ctrl.Request); err != nil {
		logger.Errorf("check cidr: %s", err)
		ctrl.Response.Update(model.CodeParamError, err.Error())
		return
	}
}

func checkCIDR(ctx context.Context, req *model.CheckCIDRReq) error {

	if err := utils.CheckK8SPodCIDR(req.PodCIDR, net.IPv4len); err != nil {
		return errors.Wrapf(err, "podCidr: %s", req.PodCIDR)
	}
	if err := utils.CheckK8SServiceCIDR(req.ServiceCIDR, net.IPv4len); err != nil {
		return errors.Wrapf(err, "serviceCidr: %s", req.ServiceCIDR)
	}
	if err := utils.CheckCIDROverlap(req.PodCIDR, req.ServiceCIDR); err != nil {
		return errors.Wrapf(err, "podCidr and serviceCidr: %s vs %s", req.PodCIDR, req.ServiceCIDR)
	}

	if req.IPv6 {
		if err := utils.CheckK8SPodCIDR(req.PodCIDRIPv6, net.IPv6len); err != nil {
			return errors.Wrapf(err, "podCidrIpv6: %s", req.PodCIDRIPv6)
		}
		if err := utils.CheckK8SServiceCIDR(req.ServiceCIDRIPv6, net.IPv6len); err != nil {
			return errors.Wrapf(err, "serviceCidrIpv6: %s", req.ServiceCIDRIPv6)
		}
		if err := utils.CheckCIDROverlap(req.PodCIDRIPv6, req.ServiceCIDRIPv6); err != nil {
			return errors.Wrapf(err, "podCidrIpv6 and serviceCidrIpv6: %s vs %s", req.PodCIDRIPv6, req.ServiceCIDRIPv6)
		}
	}
	return nil
}
