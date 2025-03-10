package models

import "time"

type QuestionRequest struct {
	PathURL         string `json:"path_url"`
	Answer          string `json:"answer"`
	SolutionURL     string `json:"solution_url"`
	PublisherID     int    `json:"publisher_id"`
	DifficultyLevel string `json:"difficulty_level"`
	CategoryIDs     []int  `json:"category_ids"`
}

type Question struct {
	ID              int       `json:"id"`
	PathURL         string    `json:"path_url"`
	Answer          string    `json:"answer"`
	Popularity      int       `json:"popularity"`
	CreatedUserID   int       `json:"created_user_id"`
	UpdatedUserID   int       `json:"updated_user_id"`
	SolutionURL     string    `json:"solution_url"`
	PublisherID     int       `json:"publisher_id"`
	DifficultyLevel string    `json:"difficulty_level"`
	Categories      []int     `json:"categories"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}
