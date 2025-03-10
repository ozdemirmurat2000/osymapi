package handlers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"osymapp/auth"
	"osymapp/db"
	"osymapp/models"
	"osymapp/utils"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/bcrypt"
)

func Login(w http.ResponseWriter, r *http.Request) {
	var creds models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Geçersiz istek formatı")
		return
	}

	pool := db.GetPool()
	var user models.User

	// Önce kullanıcının temel bilgilerini al
	err := pool.QueryRow(context.Background(),
		`SELECT id, username, email, name, surname, age
		 FROM public.users 
		 WHERE username = $1`,
		creds.Username).Scan(
		&user.ID, &user.Username, &user.Email, &user.Name, &user.Surname, &user.Age)

	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, "Geçersiz kullanıcı bilgileri")
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
		utils.SendError(w, http.StatusInternalServerError, "Kullanıcı rolleri alınırken hata oluştu")
		return
	}
	defer rows.Close()

	// Rolleri diziye ekle
	var roles []string
	for rows.Next() {
		var role string
		if err := rows.Scan(&role); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Rol bilgisi okunurken hata")
			return
		}
		roles = append(roles, role)
	}
	user.Roles = roles

	// Şifre kontrolü için sorgu
	var hashedPassword string
	err = pool.QueryRow(context.Background(), "SELECT password FROM public.users WHERE username=$1", creds.Username).Scan(&hashedPassword)
	if err != nil {
		utils.SendError(w, http.StatusUnauthorized, "Geçersiz kullanıcı bilgileri")
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(creds.Password)); err != nil {
		utils.SendError(w, http.StatusUnauthorized, "Geçersiz kullanıcı bilgileri")
		return
	}

	token, err := auth.GenerateJWT(user.ID, user.Username)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Token oluşturulurken hata")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
	})

	response := map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       user.ID,
			"username": user.Username,
			"email":    user.Email,
			"name":     user.Name,
			"surname":  user.Surname,
			"age":      user.Age,
			"roles":    user.Roles,
		},
	}

	utils.SendSuccess(w, "Giriş başarılı", response)
}

func Register(w http.ResponseWriter, r *http.Request) {
	var creds models.Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Geçersiz istek formatı")
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Şifre işleme hatası")
		return
	}

	pool := db.GetPool()

	// Transaction başlat
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction başlatma hatası")
		return
	}
	defer tx.Rollback(context.Background())

	// Kullanıcıyı oluştur
	var userID int
	err = tx.QueryRow(context.Background(),
		"INSERT INTO public.users (username, password, email, name, surname, age) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id",
		creds.Username, string(hashedPassword), creds.Email, creds.Name, creds.Surname, creds.Age).Scan(&userID)

	if err != nil {
		if pgErr, ok := err.(*pgconn.PgError); ok {
			switch pgErr.Code {
			case "23505": // unique_violation
				if strings.Contains(pgErr.Message, "uk_email") {
					utils.SendError(w, http.StatusConflict, "Bu e-posta adresi zaten kullanımda")
					return
				}
				if strings.Contains(pgErr.Message, "uk_username") {
					utils.SendError(w, http.StatusConflict, "Bu kullanıcı adı zaten kullanımda")
					return
				}
			case "23502": // not_null_violation
				utils.SendError(w, http.StatusBadRequest, "Tüm zorunlu alanları doldurunuz")
				return
			default:
				utils.SendError(w, http.StatusInternalServerError, "Kullanıcı kaydı oluşturulurken bir hata oluştu")
				return
			}
		}
		utils.SendError(w, http.StatusInternalServerError, "Kullanıcı kaydı oluşturulurken bir hata oluştu")
		return
	}

	// User rolünü bul
	var roleID int
	err = tx.QueryRow(context.Background(), "SELECT id FROM public.roles WHERE name = 'User'").Scan(&roleID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Kullanıcı rolü bulunamadı")
		return
	}

	// Kullanıcıya User rolünü ata
	_, err = tx.Exec(context.Background(),
		"INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)",
		userID, roleID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Kullanıcı rolü atanırken bir hata oluştu")
		return
	}

	// Transaction'ı commit et
	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction commit hatası")
		return
	}

	utils.SendSuccess(w, "Kullanıcı başarıyla kaydedildi", map[string]interface{}{
		"user_id": userID,
		"email":   creds.Email,
	})
}

// Rol yönetimi için yeni fonksiyonlar
func AssignRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	log.Printf("Attempting to assign role for user ID: %s", userID)

	var roleData struct {
		RoleName string `json:"role_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		log.Printf("Error decoding request body: %v", err)
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	log.Printf("Requested role name: %s", roleData.RoleName)

	pool := db.GetPool()

	// Rol ID'sini bul
	var roleID int
	err := pool.QueryRow(context.Background(),
		"SELECT id FROM public.roles WHERE name = $1", roleData.RoleName).Scan(&roleID)
	if err != nil {
		log.Printf("Error finding role: %v", err)
		utils.SendError(w, http.StatusNotFound, "Role not found")
		return
	}

	log.Printf("Found role ID: %d", roleID)

	// Rolü ata
	_, err = pool.Exec(context.Background(),
		"INSERT INTO public.users_roles (user_id, role_id) VALUES ($1, $2)",
		userID, roleID)
	if err != nil {
		log.Printf("Error assigning role: %v", err)
		utils.SendError(w, http.StatusInternalServerError, "Error assigning role")
		return
	}

	utils.SendSuccess(w, "Role assigned successfully", nil)
}

func RemoveRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")
	var roleData struct {
		RoleName string `json:"role_name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&roleData); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
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

	// Rolü kaldır
	_, err = pool.Exec(context.Background(),
		"DELETE FROM public.users_roles WHERE user_id = $1 AND role_id = $2",
		userID, roleID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error removing role")
		return
	}

	utils.SendSuccess(w, "Role removed successfully", nil)
}

// Misafir girişi için handler
func GuestLogin(w http.ResponseWriter, r *http.Request) {
	token, err := auth.GenerateGuestJWT()
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Misafir token oluşturulurken hata")
		return
	}

	// Cookie olarak token'ı ayarla
	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    token,
		HttpOnly: true,
		Path:     "/",
	})

	response := map[string]interface{}{
		"token": token,
		"user": map[string]interface{}{
			"id":       "guest",
			"username": "guest",
			"is_guest": true,
		},
	}

	utils.SendSuccess(w, "Misafir girişi başarılı", response)
}
