package main

import (
	"sync"
	"time"
)

const (
	// configuration
	jwtSecret               = "replace-with-strong-secret"
	jwtCookieName           = "quiz_token"
	jwtExpiration           = 24 * time.Hour
	persistenceStateAbsPath = "./assets/state.json"
	usersFilePath           = "./assets/users.json"
	questionsFilePath       = "./assets/questions.json"

	//http server
	serverAddress = ":8080"
	claimsKey     = "claims"

	// external API
	avalaiAPIKey = "aa-Kjck4mY1yilINKUwlmzl8uB0qbyqpWOZWigPMvJEPuhSgbAJ"
	avalaiAPIURL = "https://api.avalai.ir/v1/chat/completions"
)

// System prompt mapping
var systemPrompts = map[int]string{
	1: "You are a helpful assistant for a quiz competition. Provide clear and concise answers to help users understand the questions better.",
	2: "You are an expert tutor. Explain concepts in detail and provide examples to help users learn effectively.",
	3: "You are a quiz master. Give hints and guidance without revealing the direct answer to help users think through the problem.",
}

var (
	// loaded data
	usersByUsername = map[string]User{}
	questionsByID   = map[int]Question{}

	// in-memory mutable state
	state   = &InMemoryState{Users: map[string]*UserState{}}
	stateMu sync.RWMutex
)

type User struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type Question struct {
	ID              int               `json:"id"`
	Answer          string            `json:"answer"`
	Score           int               `json:"score"`
	Penalty         int               `json:"penalty"`
	PenaltyTryCount int               `json:"penalty_try_count"`
	PerUserAnswers  map[string]string `json:"per_user_answers"`
}

func (q Question) GetCorrectAnswer(username string) string {
	answer, ok := q.PerUserAnswers[username]
	if ok {
		return answer
	}
	return q.Answer

}

type AttemptRecord struct {
	QuestionID int       `json:"question_id"`
	Answer     string    `json:"answer"`
	Correct    bool      `json:"correct"`
	At         time.Time `json:"at"`
}

type AttemptRecords []AttemptRecord

type PromptRecords []PromptRecord

type UserQuestionState struct {
	AttemptHistory AttemptRecords `json:"attempt_history"`
	PromptHistory  PromptRecords  `json:"prompt_history"`
}

type PromptRecord struct {
	UserPrompt     string    `json:"user_prompt"`
	SystemPromptID int       `json:"system_prompt_id"`
	Result         string    `json:"result"`
	At             time.Time `json:"at"`
}

func (a AttemptRecords) CountByCorrectnessState(correct bool) int {
	result := 0
	for _, record := range a {
		if record.Correct == correct {
			result++
		}
	}
	return result
}

func (a AttemptRecords) Solved() bool {
	return a.CountByCorrectnessState(true) > 0
}

type UserState struct {
	Username           string                     `json:"username"`
	TotalScore         int                        `json:"total_score"`
	LastSolvedQuestion int                        `json:"last_solved_question"`
	PerQuestion        map[int]*UserQuestionState `json:"per_question"`
}

type InMemoryState struct {
	// username -> state
	Users map[string]*UserState `json:"users"`
}

type submitAnswerRequest struct {
	QuestionID int    `json:"question_id"`
	Answer     string `json:"answer"`
}

type promptRequest struct {
	UserPrompt     string `json:"user_prompt"`
	SystemPromptID int    `json:"system_prompt_id"`
}

type promptResponse struct {
	Result string `json:"result"`
}

// External API request/response structures
type avalaiMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type avalaiRequest struct {
	Model    string          `json:"model"`
	Messages []avalaiMessage `json:"messages"`
}

type avalaiChoice struct {
	Message avalaiMessage `json:"message"`
}

type avalaiResponse struct {
	Choices []avalaiChoice `json:"choices"`
}
