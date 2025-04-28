package types

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
)

func ProcessIncomingCorpus(c *gin.Context, serviceName string) (Corpus, error) {
	var corpus Corpus
	if err := c.ShouldBindJSON(&corpus); err != nil {
		log.Printf("[ERROR] %s: Invalid JSON in request body: %v", serviceName, err)
		c.JSON(http.StatusBadRequest, gin.H{
			"status": "error",
			"error":  "Invalid JSON in request body",
		})
		return corpus, err
	}
	return corpus, nil
}

func ExtractPairsFromSubphrases(subphrases [][]string) [][]string {
	var pairs [][]string
	for _, subphrase := range subphrases {
		if len(subphrase) == 2 {
			pairs = append(pairs, subphrase)
		}
	}
	return pairs
}
