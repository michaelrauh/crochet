//go:build e2e
// +build e2e

package e2e

import (
	"io"
	"net/http"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = Describe("Ping Endpoint", func() {
	It("returns pong", func() {
		resp, err := http.Get("http://localhost:8080/ping")
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
		Expect(string(body)).To(Equal(`{"message":"pong"}`))
	})
})
