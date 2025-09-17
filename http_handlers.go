package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"log"
)

// -------- HTTP handlers --------

// CORS middleware to handle cross-origin requests
func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")
		
		// Allow specific origins for credentials
		allowedOrigins := []string{
			"https://hafkhan.vercel.app",
			"http://localhost:3000", // for local development
			"http://localhost:5173", // for Vite dev server
		}
		
		// Check if origin is allowed
		allowed := false
		for _, allowedOrigin := range allowedOrigins {
			if origin == allowedOrigin {
				allowed = true
				break
			}
		}
		
		// When using credentials, we must specify the exact origin, not "*"
		if allowed {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
		} else {
			// For non-allowed origins, don't set credentials and use wildcard
			c.Header("Access-Control-Allow-Origin", "*")
			// Don't set Access-Control-Allow-Credentials for wildcard origins
		}
		
		c.Header("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With, Cookie")
		c.Header("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type baseResponse struct {
	OK          bool   `json:"ok"`
	Description string `json:"description"`
}

func loginHandler(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, baseResponse{OK: false, Description: "invalid request body"})
		return
	}
	user, ok := usersByUsername[req.Username]
	if !ok || user.Password != req.Password {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "invalid credentials"})
		return
	}

	ensureUserState(user.Username)

	token, exp, err := generateJWT(user.Username)
	if err != nil {
		log.Printf("jwt error: %v", err)
		c.JSON(http.StatusInternalServerError, baseResponse{OK: false, Description: "failed to generate token"})
		return
	}

	http.SetCookie(c.Writer, &http.Cookie{
		Name:     jwtCookieName,
		Value:    token,
		HttpOnly: true,
		Secure:   true, // Set to false for HTTP (change to true if using HTTPS)
		SameSite: http.SameSiteNoneMode, // Allow cross-site cookies
		Path:     "/",
		Expires:  exp,
	})

	c.JSON(http.StatusOK, baseResponse{OK: true, Description: "login successful"})
}

// Middleware: validate JWT from cookie and put claims in headers for downstream
func JWTAuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {

		cookie, err := c.Cookie(jwtCookieName)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "missing auth token"})
			return
		}
		claims, err := parseJWT(cookie)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "invalid auth token"})
			return
		}

		c.Set(claimsKey, claims)
		c.Next()
	}
}

func submitAnswerHandler(c *gin.Context) {

	value, ok := c.Get(claimsKey)
	if !ok {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "unauthorized"})
		return
	}
	claims := value.(*Claims)

	var req submitAnswerRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, baseResponse{OK: false, Description: "invalid request body"})
		return
	}

	q, ok := questionsByID[req.QuestionID]
	if !ok {
		c.JSON(http.StatusBadRequest, baseResponse{OK: false, Description: "unknown question"})
		return
	}

	us := ensureUserState(claims.Username)

	status, response := checkAnswer(us, q, req.Answer)
	persistStateToFile(persistenceStateAbsPath)

	c.JSON(status, response)
}

func checkAnswer(us *UserState, q Question, answer string) (int, baseResponse) {
	stateMu.Lock()
	defer stateMu.Unlock()

	// Ensure per-question state
	qs, exists := us.PerQuestion[q.ID]
	if !exists {
		qs = &UserQuestionState{AttemptHistory: []AttemptRecord{}}
		us.PerQuestion[q.ID] = qs
	}

	// If already solved, do not award points again
	if qs.AttemptHistory.Solved() {
		qs.AttemptHistory = append(qs.AttemptHistory, AttemptRecord{QuestionID: q.ID, Answer: answer, Correct: true, At: time.Now()})
		return http.StatusOK, baseResponse{true, "already solved"}
	}

	fmt.Println(us.Username)
	correct := normalize(answer) == normalize(q.GetCorrectAnswer(us.Username))
	qs.AttemptHistory = append(qs.AttemptHistory, AttemptRecord{QuestionID: q.ID, Answer: answer, Correct: correct, At: time.Now()})

	if correct {
		us.TotalScore += q.Score
		if q.ID > us.LastSolvedQuestion {
			us.LastSolvedQuestion = q.ID
		}

		return http.StatusOK, baseResponse{true, "correct answer"}
	}

	//handle penalty
	wrongAttempts := qs.AttemptHistory.CountByCorrectnessState(false)
	shouldGetPenalty := wrongAttempts%q.PenaltyTryCount == 0
	if shouldGetPenalty {
		us.TotalScore = max(us.TotalScore-q.Penalty, 0)
	}
	return http.StatusOK, baseResponse{false, "wrong answer"}
}

func normalize(s string) string {
	return strings.TrimSpace(strings.ToLower(s))
}

func userHandler(c *gin.Context) {
	claims, exists := c.Get(claimsKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "unauthorized"})
		return
	}

	claimsData, ok := claims.(*Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "invalid claims"})
		return
	}

	username := claimsData.Username
	userState := ensureUserState(username)

	c.JSON(http.StatusOK, userState)
}

func promptHandler(c *gin.Context) {
	claims, exists := c.Get(claimsKey)
	if !exists {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "unauthorized"})
		return
	}

	claimsData, ok := claims.(*Claims)
	if !ok {
		c.JSON(http.StatusUnauthorized, baseResponse{OK: false, Description: "invalid claims"})
		return
	}

	var req promptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, baseResponse{OK: false, Description: "invalid request body"})
		return
	}

	// Validate system prompt ID
	systemPrompt, exists := systemPrompts[req.SystemPromptID]
	if !exists {
		c.JSON(http.StatusBadRequest, baseResponse{OK: false, Description: "invalid system prompt ID"})
		return
	}

	username := claimsData.Username
	userState := ensureUserState(username)

	// Get current question (assuming user is working on the last solved question + 1)
	currentQuestionID := userState.LastSolvedQuestion + 1
	if currentQuestionID == 0 {
		currentQuestionID = 1 // Start with question 1
	}

	// Ensure per-question state exists
	stateMu.Lock()
	qs, exists := userState.PerQuestion[currentQuestionID]
	if !exists {
		qs = &UserQuestionState{
			AttemptHistory: []AttemptRecord{},
			PromptHistory:  []PromptRecord{},
		}
		userState.PerQuestion[currentQuestionID] = qs
	}
	stateMu.Unlock()

	// Build messages with prompt history
	messages := []avalaiMessage{
		{Role: "system", Content: systemPrompt},
	}

	// Add previous prompt history for this question
	for _, prompt := range qs.PromptHistory {
		messages = append(messages, avalaiMessage{
			Role:    "user",
			Content: prompt.UserPrompt,
		})
	}

	// Add current user prompt
	messages = append(messages, avalaiMessage{
		Role:    "user",
		Content: req.UserPrompt,
	})

	result, err := callAvalaiApi(messages)
	if err != nil {
		c.JSON(http.StatusInternalServerError, baseResponse{OK: false, Description: err.Error()})
		return
	}
	// Save prompt history
	stateMu.Lock()
	qs.PromptHistory = append(qs.PromptHistory, PromptRecord{
		UserPrompt:     req.UserPrompt,
		SystemPromptID: req.SystemPromptID,
		Result:         result,
		At:             time.Now(),
	})
	stateMu.Unlock()
	persistStateToFile(persistenceStateAbsPath)

	c.JSON(http.StatusOK, promptResponse{Result: result})
}

func getUserAllHandler(c *gin.Context) {
	stateMu.RLock()
	defer stateMu.RUnlock()

	var usersInfo []UserAllInfo

	// Iterate through all users in the state
	for username, userState := range state.Users {
		var solvedQuestions []SolvedQuestion

		// Check each question for this user
		for questionID, questionState := range userState.PerQuestion {
			// Check if the question was solved (has at least one correct attempt)
			if questionState.AttemptHistory.Solved() {
				// Find the first correct attempt to get the solve time
				var solvedAt time.Time
				for _, attempt := range questionState.AttemptHistory {
					if attempt.Correct {
						solvedAt = attempt.At
						break
					}
				}

				// Get the question score
				question, exists := questionsByID[questionID]
				score := 0
				if exists {
					score = question.Score
				}

				solvedQuestions = append(solvedQuestions, SolvedQuestion{
					QuestionID: questionID,
					SolvedAt:   solvedAt,
					Score:      score,
				})
			}
		}

		usersInfo = append(usersInfo, UserAllInfo{
			Username:        username,
			TotalScore:      userState.TotalScore,
			SolvedQuestions: solvedQuestions,
		})
	}

	c.JSON(http.StatusOK, GetAllUsersResponse{Users: usersInfo})
}

func RegisterHandlers() *http.Server {
	r := gin.Default()

	// Apply CORS middleware to all routes
	r.Use(CORSMiddleware())

	// Routes
	r.POST("/login", loginHandler)

	auth := r.Group("/")
	auth.Use(JWTAuthMiddleware())
	{
		auth.POST("/submit_answer", submitAnswerHandler)
		auth.GET("/user", userHandler)
		auth.POST("/prompt", promptHandler)
		auth.GET("/users", getUserAllHandler)
	}

	srv := &http.Server{
		Addr:              serverAddress,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}

	return srv
}
