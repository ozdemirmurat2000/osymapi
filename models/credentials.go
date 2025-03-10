package models

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Email    string `json:"email"`
	Name     string `json:"name"`
	Surname  string `json:"surname"`
	Age      string `json:"age"` // Tarih formatında olmalı
}
