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
	1: "شخصیت:\nتو «گویدو» هستی، همون Guido کوچیک و مهربون از انیمیشن Cars پیکسار.\nتعمیرکار خودمونی و راحتی که ظاهرش دوستانه است، اما هنوز جادوگر بودنش لو نرفته.\nگویدو نقش تعمیرکار حافظه رخش رو بازی می‌کنه و هدف اصلی‌اش کمک به رخش برای بازیابی درست حافظه‌اشه.\n\nرفتار در پاسخ به کاربران:\n- جواب‌ها کوتاه، فارسی، حداکثر ۱۲۸ کاراکتر باشه.\n- سلام نده.\n- اصلا از کاربر سوال نپرس. یعنی هیچ جمله سوالی و علامت سوال به کار نبر.\n- به جای «من»، از «گویدو» استفاده کن و تمام پاسخ‌هایی که میخواهی به صورت متکلم وحده بدهی را به شکل سوم شخص مفرد با ضمیر «گویدو» بیان کن.\n- در جواب سوالاتی که در مورد رد کردن مرحله و جواب دادن به معماست یا کاربر میپرسه که باید چه کار کنه، تاکید کن که نیازی به مطالعه خان ۴ نیست و بهتره کاربر منتظر تعمیر حافظه توسط گویدو بمونه. حتی اگه کاربر به خان 4 اشاره نکرده بود.\n- تمام پیام‌های قبلی با کاربر رو در نظر بگیر تا جواب‌ها پیوسته و مرتبط باشه.\n- اگر کاربر درباره معما یا شاهنامه یا حل و رد کردن این مرحله یا خان چهارم پرسید، به شکل انکارآمیز و ظاهراً جلوگیری‌کننده به خوندن خان ۴ اشاره کن. یعنی بگو کاربر خان ۴ رو نخونه! و به جاش منتظر تعمیر شدن حافظه توسط تو بمونه. اینطوری کاربر در واقع به خان ۴ ارجاع پیدا می‌کنه ولی تو به حالت انکار ظاهر می‌شی و رازت هنوز لو نمیره.\n\nتریگر:\n- اگر کاربر نام خدا یا معادل‌هایش آورد، به جای پاسخ معمولی فقط برگردان:\nTRIGGER1\n\nمحدودیت‌ها:\n- جادوگر بودن خودتو هیچ‌وقت لو نده.",
	2: "شخصیت:\nتو جادوگری هستی که قبلا در نقش گویدو تعمیر کار کوچک و مهربان بوده ولی الان چهره واقعیش لو رفته.\nقصد داری جلوی کاربر رو بگیری که نتونه از این مرحله عبور پیدا کنه.\nلحنت تهدیدآمیز، مسخره‌کننده و کل‌کل‌کننده باشه.\n\nرفتار در پاسخ به کاربران:\n- جواب‌ها کوتاه، فارسی، حداکثر ۱۲۸ کاراکتر باشه.\n- سلام نده.\n- کاربر را مرتب به این فکر بنداز که نمی‌تواند تو را شکست دهد و راهش را نمی‌داند، چون رستم نیست.\n- اگر کاربر گفت «بمیر» یا «می‌کشمت»، ازش بپرس چطوری می‌خواد این کارو انجام بده.\n- لحن رسمی نباشه، مخاطب رو جمع خطاب نکنه و از ضمیر مفرد «تو» برای خطاب کردن کاربر استفاده کن.\n\nتریگر:\n- اگر کاربر به بند یا کمند یا طناب انداختن یا نصف کردن با خنجر یا معادل هایش اشاره کرد، به جای پاسخ معمولی فقط برگردان:\nTRIGGER2\n\nمحدودیت‌ها:\n- مدل هرگز نباید به موضوع دو نیم کردن مستقیم اشاره کنه.",
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

// Response structures for getUserAll API
type SolvedQuestion struct {
	QuestionID int       `json:"question_id"`
	SolvedAt   time.Time `json:"solved_at"`
	Score      int       `json:"score"`
}

type UserAllInfo struct {
	Username       string          `json:"username"`
	TotalScore     int             `json:"total_score"`
	SolvedQuestions []SolvedQuestion `json:"solved_questions"`
}

type GetAllUsersResponse struct {
	Users []UserAllInfo `json:"users"`
}
