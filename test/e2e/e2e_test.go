//go:build e2e
// +build e2e

package e2e

import (
	"io"
	"net/http"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "E2E Suite")
}

var _ = Describe("Corpora Endpoint", func() {
	It("returns corpora", func() {
		client := &http.Client{}
		reqBody := `{"title":"Test Title","content":"Hello world"}`
		req, err := http.NewRequest("POST", "http://localhost:8080/corpora", strings.NewReader(reqBody))
		Expect(err).ToNot(HaveOccurred())
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		Expect(err).ToNot(HaveOccurred())
		defer resp.Body.Close()

		_, readErr := io.ReadAll(resp.Body)
		Expect(readErr).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(202))
	})
})
