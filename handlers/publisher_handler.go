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

// Yayıncı Ekleme
func CreatePublisher(w http.ResponseWriter, r *http.Request) {
	var publisher models.Publisher
	if err := json.NewDecoder(r.Body).Decode(&publisher); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if publisher.Name == "" {
		utils.SendError(w, http.StatusBadRequest, "Name is required")
		return
	}

	pool := db.GetPool()
	err := pool.QueryRow(context.Background(),
		`INSERT INTO publishers (name, website_url) 
         VALUES ($1, $2) 
         RETURNING id`,
		publisher.Name, publisher.WebsiteURL).Scan(&publisher.ID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error creating publisher: "+err.Error())
		return
	}

	utils.SendSuccess(w, "Publisher created successfully", publisher)
}

// Tüm Yayıncıları Getir
func GetAllPublishers(w http.ResponseWriter, r *http.Request) {
	pool := db.GetPool()
	rows, err := pool.Query(context.Background(),
		`SELECT id, name, COALESCE(website_url, '') as website_url 
         FROM publishers 
         ORDER BY id`)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching publishers: "+err.Error())
		return
	}
	defer rows.Close()

	var publishers []struct {
		ID         int    `json:"id"`
		Name       string `json:"name"`
		WebsiteURL string `json:"website_url,omitempty"`
	}

	for rows.Next() {
		var p struct {
			ID         int    `json:"id"`
			Name       string `json:"name"`
			WebsiteURL string `json:"website_url,omitempty"`
		}
		if err := rows.Scan(&p.ID, &p.Name, &p.WebsiteURL); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning publisher: "+err.Error())
			return
		}
		publishers = append(publishers, p)
	}

	utils.SendSuccess(w, "Publishers fetched successfully", publishers)
}

// Yayıncı Güncelleme
func UpdatePublisher(w http.ResponseWriter, r *http.Request) {
	publisherID := chi.URLParam(r, "id")

	var publisher models.Publisher
	if err := json.NewDecoder(r.Body).Decode(&publisher); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if publisher.Name == "" {
		utils.SendError(w, http.StatusBadRequest, "Name is required")
		return
	}

	pool := db.GetPool()
	result, err := pool.Exec(context.Background(),
		`UPDATE publishers 
         SET name = $1, website_url = $2 
         WHERE id = $3`,
		publisher.Name, publisher.WebsiteURL, publisherID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating publisher")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Publisher not found")
		return
	}

	utils.SendSuccess(w, "Publisher updated successfully", publisher)
}

// Yayıncı Silme
func DeletePublisher(w http.ResponseWriter, r *http.Request) {
	publisherID := chi.URLParam(r, "id")

	pool := db.GetPool()
	result, err := pool.Exec(context.Background(),
		"DELETE FROM publishers WHERE id = $1",
		publisherID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error deleting publisher")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Publisher not found")
		return
	}

	utils.SendSuccess(w, "Publisher deleted successfully", nil)
}
