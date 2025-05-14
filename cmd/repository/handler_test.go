package main

import (
	"crochet/mocks"
	"crochet/pkg/db"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	"github.com/gin-gonic/gin"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Handler", func() {
	var (
		mockQueries *mocks.QueriesInterface
		handler     *Handler
		recorder    *httptest.ResponseRecorder
		ginContext  *gin.Context
	)

	BeforeEach(func() {
		mockQueries = mocks.NewQueriesInterface(GinkgoT())
		handler = NewHandler(mockQueries)
		recorder = httptest.NewRecorder()

		req, _ := http.NewRequest("GET", "/ping", nil)
		ginContext, _ = gin.CreateTestContext(recorder)
		ginContext.Request = req
	})

	It("responds with pong and calls DB methods with correct args", func() {
		emptyItem := db.Item{}
		ctx := ginContext.Request.Context()
		mockQueries.On("CreateItem", ctx, "exampleItem").Return(emptyItem, nil).Once()
		mockQueries.On("GetItemByID", ctx, int32(1)).Return(emptyItem, nil).Once()

		handler.Ping(ginContext)

		Expect(recorder.Code).To(Equal(200))
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		Expect(err).NotTo(HaveOccurred())
		Expect(response["message"]).To(Equal("pong"))
		mockQueries.AssertExpectations(GinkgoT())
	})
})
