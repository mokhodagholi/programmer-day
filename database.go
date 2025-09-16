package main

import (
	"bytes"
	"encoding/json"
	"log"
	"os"
)

func loadUsersFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var list []User
	if err := json.NewDecoder(f).Decode(&list); err != nil {
		return err
	}
	tmp := make(map[string]User, len(list))
	for _, u := range list {
		if u.Username == "" {
			continue
		}
		tmp[u.Username] = u
	}
	usersByUsername = tmp
	return nil
}

func loadQuestionsFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var list []Question
	if err := json.NewDecoder(f).Decode(&list); err != nil {
		return err
	}
	tmp := make(map[int]Question, len(list))
	for _, q := range list {
		if q.ID == 0 {
			continue
		}
		tmp[q.ID] = q
	}
	questionsByID = tmp
	return nil
}

// -------- Persistence --------
func loadStateFromFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	var loaded InMemoryState
	if err := json.NewDecoder(f).Decode(&loaded); err != nil {
		return err
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	if loaded.Users == nil {
		loaded.Users = map[string]*UserState{}
	}
	state = &loaded
	return nil
}

func persistStateToFile(path string) error {
	stateMu.RLock()
	defer stateMu.RUnlock()

	var writer bytes.Buffer

	enc := json.NewEncoder(&writer)
	enc.SetIndent("", "  ")

	err := enc.Encode(state)
	if err != nil {
		return err
	}

	err = os.WriteFile(path, writer.Bytes(), 0644)
	if err != nil {
		return err
	}
	return nil
}

func ensureUserState(username string) *UserState {
	stateMu.Lock()
	defer stateMu.Unlock()
	us, ok := state.Users[username]
	if !ok {
		us = &UserState{
			Username:    username,
			PerQuestion: map[int]*UserQuestionState{},
		}
		state.Users[username] = us
	}
	return us
}

func LoadData() {
	// Initialize stores and load data files
	if err := loadUsersFromFile(usersFilePath); err != nil {
		log.Fatalf("failed to load users: %v", err)
	}
	if err := loadQuestionsFromFile(questionsFilePath); err != nil {
		log.Fatalf("failed to load questions: %v", err)
	}
	// Load persisted state if present
	if err := loadStateFromFile(persistenceStateAbsPath); err != nil {
		log.Printf("no existing state to load: %v", err)
	}
}
