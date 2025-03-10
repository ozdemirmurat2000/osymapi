package models

import "time"

type User struct {
	ID       int       `json:"id"`
	Username string    `json:"username"`
	Email    string    `json:"email"`
	Name     string    `json:"name"`
	Surname  string    `json:"surname"`
	Age      time.Time `json:"age"`
	Roles    []string  `json:"roles"`
}
