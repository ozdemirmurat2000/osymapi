package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"osymapp/db"
	"osymapp/models"
	"osymapp/utils"

	"golang.org/x/crypto/bcrypt"
)

// Profil bilgilerini getirme
func GetProfile(w http.ResponseWriter, r *http.Request) {
	// Token'dan username'i al
	username := r.Context().Value("username").(string)

	pool := db.GetPool()
	var user models.User

	// Kullanıcı bilgilerini al
	err := pool.QueryRow(context.Background(),
		`SELECT id, username, email, name, surname, age
		 FROM public.users 
		 WHERE username = $1`,
		username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Name, &user.Surname, &user.Age)

	if err != nil {
		utils.SendError(w, http.StatusNotFound, "User not found")
		return
	}

	// Kullanıcının rollerini al
	rows, err := pool.Query(context.Background(),
		`SELECT r.name 
		 FROM public.roles r
		 JOIN public.users_roles ur ON r.id = ur.role_id
		 WHERE ur.user_id = $1`,
		user.ID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching user roles")
		return
	}
	defer rows.Close()

	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning roles")
			return
		}
		roles = append(roles, role)
	}
	user.Roles = roles

	utils.SendSuccess(w, "Profile fetched successfully", user)
}

// Profil güncelleme
func UpdateProfile(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value("username").(string)

	var updateData struct {
		Email   string `json:"email"`
		Name    string `json:"name"`
		Surname string `json:"surname"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pool := db.GetPool()
	_, err := pool.Exec(context.Background(),
		`UPDATE public.users 
		 SET email = $1, name = $2, surname = $3
		 WHERE username = $4`,
		updateData.Email, updateData.Name, updateData.Surname, username)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating profile")
		return
	}

	utils.SendSuccess(w, "Profile updated successfully", nil)
}

// Şifre değiştirme
func ChangePassword(w http.ResponseWriter, r *http.Request) {
	username := r.Context().Value("username").(string)

	var passwordData struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&passwordData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pool := db.GetPool()

	// Mevcut şifreyi kontrol et
	var currentHashedPassword string
	err := pool.QueryRow(context.Background(),
		"SELECT password FROM public.users WHERE username = $1",
		username).Scan(&currentHashedPassword)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching user")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(currentHashedPassword), []byte(passwordData.CurrentPassword)); err != nil {
		utils.SendError(w, http.StatusUnauthorized, "Current password is incorrect")
		return
	}

	// Yeni şifreyi hashle
	hashedNewPassword, err := bcrypt.GenerateFromPassword([]byte(passwordData.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error hashing new password")
		return
	}

	// Şifreyi güncelle
	_, err = pool.Exec(context.Background(),
		"UPDATE public.users SET password = $1 WHERE username = $2",
		string(hashedNewPassword), username)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating password")
		return
	}

	utils.SendSuccess(w, "Password changed successfully", nil)
}
