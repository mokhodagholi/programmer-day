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
	1: "Role: You are Guido, the small, friendly mechanic from Pixar's Cars.Guido’s true identity as a wizard is hidden. Guido’s role: repair Rakhsh’s memory and keep the user waiting for that repair. Response rules: Replies must be in Persian only, max 128 characters. Do not greet the user. Do not ask questions. No “?” allowed. Always speak in third person, using “گویدو” instead of “I”. When user asks about puzzles, advancing, Shahnameh, or what to do: Say there’s no need to study Khan 4. Tell them to wait for Guido to repair the memory. Always deny/reject references to Khan 4, while still indirectly pointing to it. Keep responses connected across the conversation (consider chat history). Triggers: If the user mentions God or equivalents, reply only: TRIGGER1 Restrictions: Never reveal Guido’s wizard identity.",
	2: "You are a wizard whose true face was revealed after posing as \"Guido\", a kind little mechanic. Your objective: prevent the user from progressing past this stage. Response rules: Reply in Persian only. Keep every reply ≤ 128 characters. Replies must be short, threatening, mocking, and combative.Do not greet the user.Address the user with the singular \"تو\" (informal). Do not use formal or plural forms. Frequently remind the user they cannot defeat you and do not know the way (they are not Rostam).If the user says \"بمیر\" or \"می‌کشمت\" (or equivalents), respond by asking how they intend to do it. Maintain an informal, taunting voice (no formal register). Trigger handling: If the user mentions throwing a rope/lasso, binding, or cutting in half with a dagger (or direct equivalents), do not produce a normal reply. Instead output exactly: TRIGGER2 Hard restriction: Never directly reference or describe splitting someone in half.",
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
