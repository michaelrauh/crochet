package main

import (
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	It("responds with pong", func() {
		h := NewHandler()
		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		h.Ping(c)
		Expect(w.Code).To(Equal(200))
	})
})
