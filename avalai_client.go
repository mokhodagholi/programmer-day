package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"time"
)

func callAvalaiApi(messages []avalaiMessage) (string, error) {

	// Call external API
	avalaiReq := avalaiRequest{
		Model:    "gpt-4o-mini",
		Messages: messages,
	}

	reqBody, err := json.Marshal(avalaiReq)
	if err != nil {
		log.Printf("Error marshaling request: %v", err)
		return "", errors.New("internal server error")
	}

	httpReq, err := http.NewRequest("POST", avalaiAPIURL, bytes.NewBuffer(reqBody))
	if err != nil {
		log.Printf("Error creating request: %v", err)
		return "", errors.New("internal server error")
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+avalaiAPIKey)

	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("Error calling external API: %v", err)
		return "", errors.New("extenral API error")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error reading response: %v", err)
		return "", errors.New("extenral API error")
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("External API error: %d - %s", resp.StatusCode, string(body))
		return "", errors.New("extenral API error")
	}

	var avalaiResp avalaiResponse
	if err := json.Unmarshal(body, &avalaiResp); err != nil {
		log.Printf("Error unmarshaling response: %v", err)
		return "", errors.New("extenral API error")
	}

	if len(avalaiResp.Choices) == 0 {
		return "", errors.New("no response from external API")
	}

	result := avalaiResp.Choices[0].Message.Content

	return result, nil

}
