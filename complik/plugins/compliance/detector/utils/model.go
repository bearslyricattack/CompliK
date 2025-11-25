package utils

type APIResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

type ComplianceResult struct {
	IsIllegal   string `json:"is_illegal"`
	Explanation string `json:"explanation"`
}

type ResultDict struct {
	Description string           `json:"description"`
	Keywords    any              `json:"keywords"`
	Compliance  ComplianceResult `json:"compliance"`
}
