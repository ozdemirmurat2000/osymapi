package appmiddleware

import (
	"context"
	"net/http"
	"osymapp/db"
	"osymapp/utils"
)

// Misafir olmayan kullanıcıları kontrol eden middleware
func RequireNonGuest(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		isGuest, ok := r.Context().Value("is_guest").(bool)
		if !ok || isGuest {
			utils.SendError(w, http.StatusForbidden, "Bu işlem için üye girişi yapmanız gerekmektedir")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Admin rolünü kontrol eden middleware
func RequireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Misafir kontrolü
		isGuest, ok := r.Context().Value("is_guest").(bool)
		if !ok || isGuest {
			utils.SendError(w, http.StatusForbidden, "Bu işlem için üye girişi yapmanız gerekmektedir")
			return
		}

		// Kullanıcı ID'sini al
		userID := r.Context().Value("userID").(int)

		// Kullanıcının rollerini kontrol et
		pool := db.GetPool()
		rows, err := pool.Query(context.Background(),
			`SELECT r.name 
			 FROM public.roles r
			 JOIN public.users_roles ur ON r.id = ur.role_id
			 WHERE ur.user_id = $1`,
			userID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Kullanıcı rolleri kontrol edilirken hata oluştu")
			return
		}
		defer rows.Close()

		isAdmin := false
		for rows.Next() {
			var role string
			if err := rows.Scan(&role); err != nil {
				utils.SendError(w, http.StatusInternalServerError, "Rol bilgisi okunurken hata")
				return
			}
			if role == "Admin" {
				isAdmin = true
				break
			}
		}

		if !isAdmin {
			utils.SendError(w, http.StatusForbidden, "Bu işlem için admin yetkisi gerekmektedir")
			return
		}

		next.ServeHTTP(w, r)
	})
}
