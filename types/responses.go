package types

// OrthosSaveResponse is the response from saving orthos
type OrthosSaveResponse struct {
	Status  string   `json:"status"`
	Message string   `json:"message"`
	Count   int      `json:"count"`
	NewIDs  []string `json:"newIDs"` // Field to store the IDs of newly saved orthos
}

// Note: This is the only declaration of OrthosSaveResponse.
// Other response types are declared in models.go and should not be duplicated here.
