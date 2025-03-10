package main

import (
	"log"
	"net/http"
	"osymapp/auth"
	"osymapp/db"
	"osymapp/handlers"
	appmiddleware "osymapp/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

func main() {
	// Veritabanı bağlantı havuzunu başlat
	if err := db.Connect(); err != nil {
		log.Fatal("Could not initialize database connection pool:", err)
	}
	defer db.GetPool().Close()

	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// CORS middleware'ini ekle
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000"}, // React uygulamanızın adresi
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Merhaba, Dünya!"))
	})

	r.Post("/login", handlers.Login)
	r.Post("/register", handlers.Register)
	r.Post("/logout", auth.Logout)
	r.Post("/guest-login", handlers.GuestLogin)

	r.Group(func(r chi.Router) {
		r.Use(auth.JwtVerify)

		// Profil endpoint'leri
		r.Get("/profile", handlers.GetProfile)
		r.Put("/profile", handlers.UpdateProfile)
		r.Put("/profile/password", handlers.ChangePassword)

		// Normal kullanıcı işlemleri
		r.Post("/questions", handlers.CreateQuestion)
		r.Get("/questions", handlers.GetQuestions)
		r.Put("/questions/{id}", handlers.UpdateQuestion)
		r.Delete("/questions/{id}", handlers.DeleteQuestion)

		// Sadece Admin rolüne sahip kullanıcılar için rol yönetimi
		r.Group(func(r chi.Router) {
			r.Use(appmiddleware.RequireAdmin)
			r.Post("/users/{userId}/roles", handlers.AssignRole)
			r.Delete("/users/{userId}/roles", handlers.RemoveRole)
		})

		// Admin endpoint'leri
		r.Group(func(r chi.Router) {
			r.Use(auth.JwtVerify)
			r.Use(appmiddleware.RequireAdmin)

			// Kullanıcı ve rol yönetimi
			r.Get("/admin/users", handlers.GetAllUsers)
			r.Get("/admin/roles", handlers.GetAllRoles)
			r.Post("/users/{userId}/roles", handlers.AssignRole)
			r.Delete("/users/{userId}/roles", handlers.RemoveRole)
			r.Put("/admin/users/{userId}/roles", handlers.UpdateUserRoles)
			r.Delete("/admin/users/{userId}/roles", handlers.RemoveUserRole)

			// Kategori işlemleri
			r.Post("/admin/categories", handlers.CreateCategoryHierarchy)
			r.Get("/admin/categories", handlers.GetCategoryHierarchy)
			r.Post("/admin/categories/{mainId}/sub", handlers.AddSubCategory)
			r.Post("/admin/categories/sub/{subId}/categories", handlers.AddCategoriesToSub)
			r.Get("/admin/categories/main/{name}", handlers.GetMainCategoryDetails)
			r.Get("/admin/categories/all", handlers.GetAllCategories)
			r.Delete("/admin/categories/main/{mainId}", handlers.DeleteMainCategory)
			r.Put("/admin/categories/main/{mainId}", handlers.UpdateMainCategory)
			r.Delete("/admin/categories/sub/{subId}", handlers.DeleteSubCategory)
			r.Put("/admin/categories/sub/{subId}", handlers.UpdateSubCategory)
			r.Delete("/admin/categories/sub/{subId}/category/{categoryId}", handlers.DeleteCategory)
			r.Put("/admin/categories/sub/{subId}/category/{categoryId}", handlers.UpdateCategory)

			// Soru işlemleri
			r.Post("/admin/questions", handlers.CreateQuestionWithCategories)
			r.Put("/admin/questions/{id}", handlers.UpdateQuestionWithCategories)
			r.Delete("/admin/questions/{id}", handlers.DeleteQuestion)

			// Yayıncı işlemleri
			r.Post("/admin/publishers", handlers.CreatePublisher)
			r.Get("/admin/publishers", handlers.GetAllPublishers)
			r.Put("/admin/publishers/{id}", handlers.UpdatePublisher)
			r.Delete("/admin/publishers/{id}", handlers.DeletePublisher)
		})
	})

	// Statik dosya sunucusu
	r.Handle("/images/*", http.StripPrefix("/images/", http.FileServer(http.Dir("images"))))

	log.Println("Server starting on port 8080...")
	http.ListenAndServe(":8080", r)
}
