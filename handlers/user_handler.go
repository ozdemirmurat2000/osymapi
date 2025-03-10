package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"osymapp/db"
	"osymapp/models"
	"osymapp/utils"

	"github.com/go-chi/chi/v5"
)

func GetAllUsers(w http.ResponseWriter, r *http.Request) {
	pool := db.GetPool()

	// Tüm kullanıcıları getir
	rows, err := pool.Query(context.Background(), `
		SELECT id, username, email, name, surname, age 
		FROM public.users 
		ORDER BY id`)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching users")
		return
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(
			&user.ID,
			&user.Username,
			&user.Email,
			&user.Name,
			&user.Surname,
			&user.Age,
		)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning user")
			return
		}

		// Her kullanıcı için rolleri getir
		roleRows, err := pool.Query(context.Background(), `
			SELECT r.name 
			FROM public.roles r
			JOIN public.users_roles ur ON r.id = ur.role_id
			WHERE ur.user_id = $1`,
			user.ID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error fetching user roles")
			return
		}
		defer roleRows.Close()

		var roles []string
		for roleRows.Next() {
			var role string
			if err := roleRows.Scan(&role); err != nil {
				utils.SendError(w, http.StatusInternalServerError, "Error scanning role")
				return
			}
			roles = append(roles, role)
		}
		user.Roles = roles

		users = append(users, user)
	}

	utils.SendSuccess(w, "Users fetched successfully", users)
}

// Kullanıcının rollerini güncelle
func UpdateUserRoles(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	var roleData struct {
		Roles []string `json:"roles"` // Yeni rol listesi
	}

	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// "User" rolünün değiştirilmesini engelle
	for _, role := range roleData.Roles {
		if role == "User" {
			utils.SendError(w, http.StatusForbidden, "Cannot modify User role")
			return
		}
	}

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// Mevcut rolleri sil (User rolü hariç)
	_, err = tx.Exec(context.Background(), `
		DELETE FROM public.users_roles 
		WHERE user_id = $1 
		AND role_id IN (
			SELECT id FROM public.roles 
			WHERE name != 'User'
		)`, userID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error removing existing roles")
		return
	}

	// Yeni rolleri ekle
	for _, roleName := range roleData.Roles {
		var roleID int
		err := tx.QueryRow(context.Background(),
			"SELECT id FROM public.roles WHERE name = $1", roleName).Scan(&roleID)
		if err != nil {
			utils.SendError(w, http.StatusBadRequest, "Invalid role: "+roleName)
			return
		}

		_, err = tx.Exec(context.Background(),
			"INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)",
			userID, roleID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error assigning role: "+roleName)
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "User roles updated successfully", nil)
}

// Kullanıcıya yeni rol ekle
func AddUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	var roleData struct {
		RoleName string `json:"role_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// User rolünün eklenmesini engelle
	if roleData.RoleName == "User" {
		utils.SendError(w, http.StatusForbidden, "Cannot modify User role")
		return
	}

	pool := db.GetPool()

	// Rol ID'sini bul
	var roleID int
	err := pool.QueryRow(context.Background(),
		"SELECT id FROM public.roles WHERE name = $1", roleData.RoleName).Scan(&roleID)
	if err != nil {
		utils.SendError(w, http.StatusNotFound, "Role not found")
		return
	}

	// Rolü ekle
	_, err = pool.Exec(context.Background(),
		"INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)",
		userID, roleID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error adding role")
		return
	}

	utils.SendSuccess(w, "Role added successfully", nil)
}

// Kullanıcıdan rol sil
func RemoveUserRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	var roleData struct {
		RoleName string `json:"role_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// User rolünün silinmesini engelle
	if roleData.RoleName == "User" {
		utils.SendError(w, http.StatusForbidden, "Cannot remove User role")
		return
	}

	pool := db.GetPool()

	// Rol ID'sini bul
	var roleID int
	err := pool.QueryRow(context.Background(),
		"SELECT id FROM public.roles WHERE name = $1", roleData.RoleName).Scan(&roleID)
	if err != nil {
		utils.SendError(w, http.StatusNotFound, "Role not found")
		return
	}

	// Rolü sil
	_, err = pool.Exec(context.Background(),
		"DELETE FROM public.users_roles WHERE user_id = $1 AND role_id = $2",
		userID, roleID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error removing role")
		return
	}

	utils.SendSuccess(w, "Role removed successfully", nil)
}

// Tüm rolleri getir
func GetAllRoles(w http.ResponseWriter, r *http.Request) {
	pool := db.GetPool()

	rows, err := pool.Query(context.Background(), `
		SELECT id, name 
		FROM public.roles 
		ORDER BY id`)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching roles")
		return
	}
	defer rows.Close()

	type Role struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	var roles []Role
	for rows.Next() {
		var role Role
		if err := rows.Scan(&role.ID, &role.Name); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning role")
			return
		}
		roles = append(roles, role)
	}

	utils.SendSuccess(w, "Roles fetched successfully", roles)
}
