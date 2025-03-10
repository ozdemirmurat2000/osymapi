package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"osymapp/db"
	"osymapp/models"
	"osymapp/services"
	"osymapp/utils"
	"strings"

	"github.com/go-chi/chi/v5"
)

func CreateQuestion(w http.ResponseWriter, r *http.Request) {
	// Multipart form'u parse et
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		utils.SendError(w, http.StatusBadRequest, "Form parse hatası")
		return
	}

	// Form verilerini al
	var q models.Question
	if err := json.NewDecoder(strings.NewReader(r.FormValue("data"))).Decode(&q); err != nil {
		utils.SendError(w, http.StatusBadRequest, "JSON parse hatası")
		return
	}

	// Soru resmini yükle
	if file, header, err := r.FormFile("question_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.QuestionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Soru resmi yükleme hatası: "+err.Error())
			return
		}
		q.PathURL = path
	}

	// Çözüm resmini yükle
	if file, header, err := r.FormFile("solution_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.SolutionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Çözüm resmi yükleme hatası: "+err.Error())
			return
		}
		q.SolutionURL = path
	}

	pool := db.GetPool()

	// Soruyu ekle
	query := `
		INSERT INTO public.questions (
			path_url, answer, popularity, created_user_id, 
			updated_user_id, solution_url, publisher_id, 
			difficulty_level, created_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id, created_at, updated_at`

	err := pool.QueryRow(context.Background(), query,
		q.PathURL, q.Answer, q.Popularity, q.CreatedUserID,
		q.UpdatedUserID, q.SolutionURL, q.PublisherID,
		q.DifficultyLevel).Scan(&q.ID, &q.CreatedAt, &q.UpdatedAt)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru oluşturma hatası: "+err.Error())
		return
	}

	utils.SendSuccess(w, "Soru başarıyla oluşturuldu", q)
}

func GetQuestions(w http.ResponseWriter, r *http.Request) {
	// Query parametrelerini al
	subCategoryID := r.URL.Query().Get("sub_category_id")
	categoryID := r.URL.Query().Get("category_id")
	difficulty := r.URL.Query().Get("difficulty")
	search := r.URL.Query().Get("search")

	baseQuery := `
		SELECT q.id, q.path_url, q.answer, q.popularity, 
			   q.created_user_id, q.updated_user_id, q.solution_url, 
			   q.publisher_id, q.difficulty_level,
			   q.created_at, q.updated_at,
			   COALESCE(
				   (SELECT json_agg(qc.category_id)
					FROM question_categories qc
					WHERE qc.question_id = q.id), 
				   '[]'::json
			   ) as categories
		FROM public.questions q
		WHERE 1=1`

	var conditions []string
	var args []interface{}
	argCount := 1

	// Arama filtresi
	if search != "" {
		conditions = append(conditions, fmt.Sprintf("(q.path_url ILIKE $%d OR q.solution_url ILIKE $%d)", argCount, argCount))
		args = append(args, "%"+search+"%")
		argCount++
	}

	// Alt kategori filtresi
	if subCategoryID != "" {
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM question_categories qc 
				WHERE qc.question_id = q.id AND qc.category_id IN (
					SELECT id FROM categories WHERE sub_category_id = $%d
				)
			)`, argCount))
		args = append(args, subCategoryID)
		argCount++
	}

	// Kategori filtresi
	if categoryID != "" {
		conditions = append(conditions, fmt.Sprintf(`
			EXISTS (
				SELECT 1 FROM question_categories qc 
				WHERE qc.question_id = q.id AND qc.category_id = $%d
			)`, argCount))
		args = append(args, categoryID)
		argCount++
	}

	// Zorluk seviyesi filtresi
	if difficulty != "" {
		conditions = append(conditions, fmt.Sprintf("q.difficulty_level = $%d", argCount))
		args = append(args, difficulty)
		argCount++
	}

	if len(conditions) > 0 {
		baseQuery += " AND " + strings.Join(conditions, " AND ")
	}

	baseQuery += " ORDER BY q.id DESC"

	// Soruları getir
	pool := db.GetPool()
	rows, err := pool.Query(context.Background(), baseQuery, args...)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching questions: "+err.Error())
		return
	}
	defer rows.Close()

	var questions []models.Question
	for rows.Next() {
		var q models.Question
		var categoriesJSON []byte
		err := rows.Scan(
			&q.ID, &q.PathURL, &q.Answer, &q.Popularity,
			&q.CreatedUserID, &q.UpdatedUserID, &q.SolutionURL,
			&q.PublisherID, &q.DifficultyLevel,
			&q.CreatedAt, &q.UpdatedAt,
			&categoriesJSON)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning question: "+err.Error())
			return
		}

		// JSON'dan kategori ID'lerini çöz
		if err := json.Unmarshal(categoriesJSON, &q.Categories); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error parsing categories: "+err.Error())
			return
		}

		questions = append(questions, q)
	}

	utils.SendSuccess(w, "Questions fetched successfully", questions)
}

func UpdateQuestion(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var q models.Question
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pool := db.GetPool()
	query := `
		UPDATE public.questions 
		SET path_url=$1, answer=$2, popularity=$3, 
			created_user_id=$4, updated_user_id=$5, 
			solution_url=$6, publisher_id=$7, 
			difficulty_level=$8,
			updated_at=CURRENT_TIMESTAMP 
		WHERE id=$9`

	_, err := pool.Exec(context.Background(), query,
		q.PathURL, q.Answer, q.Popularity,
		q.CreatedUserID, q.UpdatedUserID, q.SolutionURL,
		q.PublisherID, q.DifficultyLevel,
		id)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating question: "+err.Error())
		return
	}

	utils.SendSuccess(w, "Question updated successfully", nil)
}

// Kategoriye Göre Soru Ekleme
func CreateQuestionWithCategories(w http.ResponseWriter, r *http.Request) {
	// Multipart form'u parse et
	if err := r.ParseMultipartForm(10 << 20); err != nil { // 10 MB limit
		utils.SendError(w, http.StatusBadRequest, "Form parse hatası")
		return
	}

	// Form verilerini al
	var req models.QuestionRequest
	if err := json.NewDecoder(strings.NewReader(r.FormValue("data"))).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "JSON parse hatası")
		return
	}

	// Kullanıcı ID'sini al
	userID := r.Context().Value("userID").(int)

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction başlatma hatası")
		return
	}
	defer tx.Rollback(context.Background())

	// Soru resmini yükle
	if file, header, err := r.FormFile("question_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.QuestionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Soru resmi yükleme hatası: "+err.Error())
			return
		}
		req.PathURL = path
	}

	// Çözüm resmini yükle
	if file, header, err := r.FormFile("solution_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.SolutionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Çözüm resmi yükleme hatası: "+err.Error())
			return
		}
		req.SolutionURL = path
	}

	// Soruyu ekle
	var questionID int
	err = tx.QueryRow(context.Background(),
		`INSERT INTO questions (
			path_url, answer, popularity, created_user_id, 
			updated_user_id, solution_url, publisher_id, 
			difficulty_level, created_at, updated_at
		) VALUES ($1, $2, 0, $3, $3, $4, $5, $6, CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
		RETURNING id`,
		req.PathURL, req.Answer, userID, req.SolutionURL,
		req.PublisherID, req.DifficultyLevel).Scan(&questionID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru ekleme hatası: "+err.Error())
		return
	}

	// Kategori ilişkilerini ekle
	for _, categoryID := range req.CategoryIDs {
		_, err = tx.Exec(context.Background(),
			"INSERT INTO question_categories (question_id, category_id) VALUES ($1, $2)",
			questionID, categoryID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Kategori ilişkisi ekleme hatası: "+err.Error())
			return
		}
	}

	// Soruyu getir
	var question models.Question
	err = tx.QueryRow(context.Background(), `
		SELECT id, path_url, answer, popularity, created_user_id, 
			   updated_user_id, solution_url, publisher_id, 
			   difficulty_level, created_at, updated_at
		FROM questions WHERE id = $1`, questionID).Scan(
		&question.ID, &question.PathURL, &question.Answer, &question.Popularity,
		&question.CreatedUserID, &question.UpdatedUserID, &question.SolutionURL,
		&question.PublisherID, &question.DifficultyLevel, &question.CreatedAt, &question.UpdatedAt)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru bilgilerini getirme hatası: "+err.Error())
		return
	}

	question.Categories = req.CategoryIDs
	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction commit hatası")
		return
	}

	utils.SendSuccess(w, "Soru başarıyla oluşturuldu", question)
}

// Kategoriye Göre Soru Güncelleme
func UpdateQuestionWithCategories(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "id")
	userID := r.Context().Value("userID").(int)

	if err := r.ParseMultipartForm(10 << 20); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Form parse hatası")
		return
	}

	var req models.QuestionRequest
	if err := json.NewDecoder(strings.NewReader(r.FormValue("data"))).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "JSON parse hatası")
		return
	}

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction başlatma hatası")
		return
	}
	defer tx.Rollback(context.Background())

	// Yeni resimler varsa güncelle
	if file, header, err := r.FormFile("question_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.QuestionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Soru resmi yükleme hatası")
			return
		}
		req.PathURL = path
	}

	if file, header, err := r.FormFile("solution_image"); err == nil {
		defer file.Close()
		path, err := services.UploadImage(file, header, services.SolutionImagesPath)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Çözüm resmi yükleme hatası")
			return
		}
		req.SolutionURL = path
	}

	// Soruyu güncelle
	result, err := tx.Exec(context.Background(),
		`UPDATE questions SET 
			path_url = COALESCE($1, path_url),
			answer = COALESCE($2, answer),
			updated_user_id = $3,
			solution_url = COALESCE($4, solution_url),
			publisher_id = COALESCE($5, publisher_id),
			difficulty_level = COALESCE($6, difficulty_level),
			updated_at = CURRENT_TIMESTAMP
		WHERE id = $7`,
		req.PathURL, req.Answer, userID, req.SolutionURL,
		req.PublisherID, req.DifficultyLevel, questionID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru güncelleme hatası")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Soru bulunamadı")
		return
	}

	// Kategori ilişkilerini güncelle
	_, err = tx.Exec(context.Background(),
		"DELETE FROM question_categories WHERE question_id = $1",
		questionID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Kategori ilişkilerini silme hatası")
		return
	}

	for _, categoryID := range req.CategoryIDs {
		_, err = tx.Exec(context.Background(),
			"INSERT INTO question_categories (question_id, category_id) VALUES ($1, $2)",
			questionID, categoryID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Kategori ilişkisi ekleme hatası")
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction commit hatası")
		return
	}

	utils.SendSuccess(w, "Soru başarıyla güncellendi", map[string]interface{}{
		"question_id": questionID,
		"categories":  req.CategoryIDs,
	})
}

// Soru Silme
func DeleteQuestion(w http.ResponseWriter, r *http.Request) {
	questionID := chi.URLParam(r, "id")

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction başlatma hatası")
		return
	}
	defer tx.Rollback(context.Background())

	// Önce resim yollarını al
	var pathURL, solutionURL string
	err = tx.QueryRow(context.Background(),
		"SELECT path_url, solution_url FROM questions WHERE id = $1",
		questionID).Scan(&pathURL, &solutionURL)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru bilgilerini alma hatası")
		return
	}

	// Kategori ilişkilerini sil
	_, err = tx.Exec(context.Background(),
		"DELETE FROM question_categories WHERE question_id = $1",
		questionID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Kategori ilişkilerini silme hatası")
		return
	}

	// Soruyu sil
	result, err := tx.Exec(context.Background(),
		"DELETE FROM questions WHERE id = $1",
		questionID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Soru silme hatası")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Soru bulunamadı")
		return
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Transaction commit hatası")
		return
	}

	// Resimleri dosya sisteminden sil
	if pathURL != "" {
		filePath := strings.TrimPrefix(pathURL, "/")
		if err := os.Remove(filePath); err != nil {
			log.Printf("Soru resmi silinirken hata: %v", err)
		}
	}

	if solutionURL != "" {
		filePath := strings.TrimPrefix(solutionURL, "/")
		if err := os.Remove(filePath); err != nil {
			log.Printf("Çözüm resmi silinirken hata: %v", err)
		}
	}

	utils.SendSuccess(w, "Soru ve ilgili resimler başarıyla silindi", nil)
}
