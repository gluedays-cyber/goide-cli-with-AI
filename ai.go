package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type openaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openaiRequest struct {
	Model    string          `json:"model"`
	Messages []openaiMessage `json:"messages"`
}

type openaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

// AskAI routes the request based on selectedModel.Provider
func AskAI(currentCode, instruction string) (string, error) {
	if selectedModel.Provider == "" {
		return "", errors.New("No model selected. Press F12 to select a model.")
	}

	sysPrompt := `You are a Go language code editor. Update the given Go code according to the instruction.
Return ONLY the final, complete, runnable Go source code.
Do NOT include any explanations, introduction, markdown syntax like ` + "`" + `" + ` + "`" + `" + ` + "`" + ` or code block decorators like ` + "`" + `" + ` + "`" + `go.`
	
	userPrompt := fmt.Sprintf(`[Instruction]
%s

[Current Code]
%s`, instruction, currentCode)

	var resultText string
	var err error

	switch selectedModel.Provider {
	case "google":
		resultText, err = askGoogle(selectedModel.Name, sysPrompt, userPrompt)
	case "groq":
		resultText, err = askOpenAICompatible(selectedModel.Name, groqKey, "https://api.groq.com/openai/v1/chat/completions", sysPrompt, userPrompt)
	case "upstage":
		resultText, err = askOpenAICompatible(selectedModel.Name, upstageKey, "https://api.upstage.ai/v1/chat/completions", sysPrompt, userPrompt)
	case "ollama":
		resultText, err = askOllama(selectedModel.Name, ollamaKey, sysPrompt, userPrompt)
	default:
		return "", fmt.Errorf("Unsupported Provider: %s", selectedModel.Provider)
	}

	if err != nil {
		return "", err
	}

	// Clean up markdown backticks and code guide strings
	resultText = strings.TrimSpace(resultText)
	resultText = strings.TrimPrefix(resultText, "```go")
	resultText = strings.TrimPrefix(resultText, "```")
	resultText = strings.TrimSuffix(resultText, "```")
	resultText = strings.TrimSpace(resultText)

	return resultText, nil
}

func askGoogle(model, sysPrompt, userPrompt string) (string, error) {
	if googleKey == "" {
		return "", errors.New("Missing Google API Key.")
	}
	url := fmt.Sprintf("https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s", model, googleKey)

	type geminiPart struct {
		Text string `json:"text"`
	}
	type geminiContent struct {
		Parts []geminiPart `json:"parts"`
	}
	type geminiRequest struct {
		SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
		Contents          []geminiContent `json:"contents"`
	}
	
	reqBody := geminiRequest{
		SystemInstruction: &geminiContent{
			Parts: []geminiPart{{Text: sysPrompt}},
		},
		Contents: []geminiContent{
			{Parts: []geminiPart{{Text: userPrompt}}},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed (status %d): %s", resp.StatusCode, string(respBody))
	}

	var geminiResp struct {
		Candidates []struct {
			Content struct {
				Parts []struct {
					Text string `json:"text"`
				} `json:"parts"`
			} `json:"content"`
		} `json:"candidates"`
	}
	if err := json.Unmarshal(respBody, &geminiResp); err != nil {
		return "", err
	}

	if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
		return "", errors.New("empty response content from AI")
	}

	return geminiResp.Candidates[0].Content.Parts[0].Text, nil
}

func askOpenAICompatible(model, apiKey, url, sysPrompt, userPrompt string) (string, error) {
	if apiKey == "" {
		return "", errors.New("Missing API Key.")
	}
	reqBody := openaiRequest{
		Model: model,
		Messages: []openaiMessage{
			{Role: "system", Content: sysPrompt},
			{Role: "user", Content: userPrompt},
		},
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	
	var apiResp openaiResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", err
	}
	
	if len(apiResp.Choices) == 0 {
		return "", errors.New("empty response content from AI")
	}
	
	return apiResp.Choices[0].Message.Content, nil
}

func askOllama(model, endpoint, sysPrompt, userPrompt string) (string, error) {
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	url := strings.TrimSuffix(endpoint, "/") + "/api/chat"
	
	reqBody := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": sysPrompt},
			{"role": "user", "content": userPrompt},
		},
		"stream": false,
	}
	
	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}
	
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("Ollama request failed (status %d): %s", resp.StatusCode, string(respBody))
	}
	
	var apiResp struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	}
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return "", err
	}
	
	return apiResp.Message.Content, nil
}
