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
		mockQueue   *mocks.Queue
		handler     *Handler
		recorder    *httptest.ResponseRecorder
		ginContext  *gin.Context
	)

	BeforeEach(func() {
		mockQueries = mocks.NewQueriesInterface(GinkgoT())
		mockQueue = mocks.NewQueue(GinkgoT())
		handler = NewHandler(mockQueries, mockQueue)
		recorder = httptest.NewRecorder()

		req, _ := http.NewRequest("GET", "/ping", nil)
		ginContext, _ = gin.CreateTestContext(recorder)
		ginContext.Request = req
	})

	It("responds with pong and calls DB and RabbitMQ methods with correct args", func() {
		emptyItem := db.Item{}
		ctx := ginContext.Request.Context()
		mockQueries.On("CreateItem", ctx, "exampleItem").Return(emptyItem, nil).Once()
		mockQueries.On("GetItemByID", ctx, int32(1)).Return(emptyItem, nil).Once()
		mockQueue.On("Publish", ctx, "ping-queue", []byte("ping-message")).Return(nil).Once()
		mockQueue.On("ConsumeOne", ctx, "ping-queue").Return([]byte("test"), nil).Once()

		handler.Ping(ginContext)

		Expect(recorder.Code).To(Equal(200))
		var response map[string]interface{}
		err := json.Unmarshal(recorder.Body.Bytes(), &response)
		Expect(err).NotTo(HaveOccurred())
		Expect(response["message"]).To(Equal("pong"))
		mockQueries.AssertExpectations(GinkgoT())
		mockQueue.AssertExpectations(GinkgoT())
	})
})
