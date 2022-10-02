package e2e_test

import (
	"net/http"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gexec"
)

var _ = Describe("healthz check should pass", func() {
	Context("start server with health check", func() {
		It("should work properly", func() {
			cmd := exec.Command(keplerBin)
			keplerSession, err = gexec.Start(cmd, nil, nil)
			Expect(err).NotTo(HaveOccurred())
			time.Sleep(5 * time.Second) // wait for server start up
			resp, err := http.Get("http://localhost:8888/healthz")
			Expect(err).NotTo(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})
	})
})
