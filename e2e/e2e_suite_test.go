package e2e

import (
	"testing"

	"gitbub.com/wbuntu/gin-template/internal/pkg/utils"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var (
	baseURL = "http://127.0.0.1:8080"
)

var httpClient *utils.HTTPClient

var _ = BeforeSuite(func() {
	httpClient = utils.NewHTTPClient(baseURL)
})

var _ = AfterSuite(func() {
})
