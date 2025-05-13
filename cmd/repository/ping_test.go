//go:build !e2e
// +build !e2e

package main

import (
	"context"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.uber.org/fx"
)

var handler *Handler

var _ = BeforeSuite(func() {
	app := fx.New(
		fx.Provide(NewHandler),
		fx.Populate(&handler),
	)
	Expect(app.Start(context.Background())).To(Succeed())
})

var _ = Describe("Repository Handler", func() {
	It("returns pong from Ping()", func() {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)

		handler.Ping(c)

		Expect(rec.Code).To(Equal(http.StatusOK))
		Expect(rec.Body.String()).To(MatchJSON(`{"message":"pong"}`))
	})
})
