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

// Kategori Hiyerarşisi Oluşturma
func CreateCategoryHierarchy(w http.ResponseWriter, r *http.Request) {
	var req models.CategoryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// En az bir alt kategori kontrolü
	if len(req.SubCategories) == 0 {
		utils.SendError(w, http.StatusBadRequest, "At least one sub category is required")
		return
	}

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// Ana kategori oluştur
	var mainCatID int
	err = tx.QueryRow(context.Background(),
		"INSERT INTO main_categories (name) VALUES ($1) RETURNING id",
		req.MainCategory).Scan(&mainCatID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error creating main category")
		return
	}

	// Alt kategoriler ve kategorileri oluştur
	for _, sub := range req.SubCategories {
		// Alt kategori kontrolü
		if sub.SubCategory == "" {
			utils.SendError(w, http.StatusBadRequest, "Sub category name is required")
			return
		}
		if len(sub.Categories) == 0 {
			utils.SendError(w, http.StatusBadRequest, "At least one category is required for each sub category")
			return
		}

		// Alt kategori oluştur
		var subCatID int
		err = tx.QueryRow(context.Background(),
			"INSERT INTO sub_categories (name) VALUES ($1) RETURNING id",
			sub.SubCategory).Scan(&subCatID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error creating sub category")
			return
		}

		// İlişki tablosuna ekle
		_, err = tx.Exec(context.Background(),
			"INSERT INTO main_category_sub_category (main_category_id, sub_category_id) VALUES ($1, $2)",
			mainCatID, subCatID)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error creating relation")
			return
		}

		// Kategorileri ekle
		for _, catName := range sub.Categories {
			_, err = tx.Exec(context.Background(),
				"INSERT INTO categories (sub_category_id, name) VALUES ($1, $2)",
				subCatID, catName)
			if err != nil {
				utils.SendError(w, http.StatusInternalServerError, "Error creating category")
				return
			}
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "Category hierarchy created successfully", req)
}

// Alt Kategori Ekleme
func AddSubCategory(w http.ResponseWriter, r *http.Request) {
	mainCategoryID := chi.URLParam(r, "mainId")
	var subCat models.SubCategoryGroup
	if err := json.NewDecoder(r.Body).Decode(&subCat); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// Alt kategori oluştur
	var subCatID int
	err = tx.QueryRow(context.Background(),
		"INSERT INTO sub_categories (name) VALUES ($1) RETURNING id",
		subCat.SubCategory).Scan(&subCatID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error creating sub category")
		return
	}

	// İlişki tablosuna ekle
	_, err = tx.Exec(context.Background(),
		"INSERT INTO main_category_sub_category (main_category_id, sub_category_id) VALUES ($1, $2)",
		mainCategoryID, subCatID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error creating relation")
		return
	}

	// Kategorileri ekle
	for _, catName := range subCat.Categories {
		_, err = tx.Exec(context.Background(),
			"INSERT INTO categories (sub_category_id, name) VALUES ($1, $2)",
			subCatID, catName)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error creating category")
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "Sub category added successfully", subCat)
}

// Kategori Hiyerarşisini Getir
func GetCategoryHierarchy(w http.ResponseWriter, r *http.Request) {
	pool := db.GetPool()
	rows, err := pool.Query(context.Background(), `
        WITH RECURSIVE CategoryData AS (
            SELECT 
                mc.id as main_id,
                mc.name as main_name,
                sc.id as sub_id,
                sc.name as sub_name,
                c.id as category_id,
                c.name as category_name
            FROM main_categories mc
            LEFT JOIN main_category_sub_category mcsc ON mc.id = mcsc.main_category_id
            LEFT JOIN sub_categories sc ON mcsc.sub_category_id = sc.id
            LEFT JOIN categories c ON sc.id = c.sub_category_id
        ),
        SubCategories AS (
            SELECT 
                main_id,
                main_name,
                sub_id,
                sub_name,
                json_agg(
                    json_build_object(
                        'id', category_id,
                        'name', category_name
                    ) ORDER BY category_id
                ) FILTER (WHERE category_id IS NOT NULL) as categories
            FROM CategoryData
            GROUP BY main_id, main_name, sub_id, sub_name
        ),
        MainCategories AS (
            SELECT 
                main_id,
                main_name,
                json_agg(
                    json_build_object(
                        'id', sub_id,
                        'name', sub_name,
                        'categories', COALESCE(categories, '[]'::json)
                    ) ORDER BY sub_id
                ) FILTER (WHERE sub_id IS NOT NULL) as sub_categories
            FROM SubCategories
            GROUP BY main_id, main_name
        )
        SELECT COALESCE(
            json_agg(
                json_build_object(
                    'id', main_id,
                    'name', main_name,
                    'sub_categories', COALESCE(sub_categories, '[]'::json)
                ) ORDER BY main_id
            ),
            '[]'::json
        ) as hierarchy
        FROM MainCategories`)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching categories: "+err.Error())
		return
	}
	defer rows.Close()

	var categoriesJSON []byte
	if rows.Next() {
		err := rows.Scan(&categoriesJSON)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning categories: "+err.Error())
			return
		}
	}

	var categories interface{}
	if categoriesJSON != nil {
		err = json.Unmarshal(categoriesJSON, &categories)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error parsing categories: "+err.Error())
			return
		}
	} else {
		categories = []interface{}{} // Boş dizi
	}

	utils.SendSuccess(w, "Categories fetched successfully", categories)
}

// Alt Kategoriye Yeni Kategoriler Ekleme
func AddCategoriesToSub(w http.ResponseWriter, r *http.Request) {
	subCategoryID := chi.URLParam(r, "subId")

	var request struct {
		Categories []string `json:"categories"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// Kategorileri ekle
	for _, catName := range request.Categories {
		_, err = tx.Exec(context.Background(),
			"INSERT INTO categories (sub_category_id, name) VALUES ($1, $2)",
			subCategoryID, catName)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error creating category")
			return
		}
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "Categories added successfully", request)
}

// Ana Kategoriye Göre Detayları Getir
func GetMainCategoryDetails(w http.ResponseWriter, r *http.Request) {
	mainCategoryName := chi.URLParam(r, "name")

	pool := db.GetPool()
	rows, err := pool.Query(context.Background(), `
        WITH CategoryGroups AS (
            SELECT 
                mc.id as main_id,
                mc.name as main_name,
                sc.id as sub_id,
                sc.name as sub_name,
                COALESCE(json_agg(c.name ORDER BY c.id) FILTER (WHERE c.id IS NOT NULL), '[]'::json) as categories
            FROM main_categories mc
            LEFT JOIN main_category_sub_category mcsc ON mc.id = mcsc.main_category_id
            LEFT JOIN sub_categories sc ON mcsc.sub_category_id = sc.id
            LEFT JOIN categories c ON sc.id = c.sub_category_id
            WHERE mc.name = $1
            GROUP BY mc.id, mc.name, sc.id, sc.name
        )
        SELECT 
            main_id,
            main_name,
            COALESCE(json_agg(
                json_build_object(
                    'sub_category', sub_name,
                    'categories', categories
                ) ORDER BY sub_id
            ) FILTER (WHERE sub_name IS NOT NULL), '[]'::json) as sub_categories
        FROM CategoryGroups
        GROUP BY main_id, main_name
        ORDER BY main_id`, mainCategoryName)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching main category details: "+err.Error())
		return
	}
	defer rows.Close()

	var result models.CategoryRequest
	var found bool

	if rows.Next() {
		var mainID int
		var subCategoriesJSON []byte

		err := rows.Scan(&mainID, &result.MainCategory, &subCategoriesJSON)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning main category: "+err.Error())
			return
		}

		err = json.Unmarshal(subCategoriesJSON, &result.SubCategories)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error parsing sub categories: "+err.Error())
			return
		}

		found = true
	}

	if !found {
		utils.SendError(w, http.StatusNotFound, "Main category not found")
		return
	}

	utils.SendSuccess(w, "Main category details fetched successfully", result)
}

// Tüm Kategorileri Getir
func GetAllCategories(w http.ResponseWriter, r *http.Request) {
	pool := db.GetPool()
	rows, err := pool.Query(context.Background(), `
        WITH CategoryGroups AS (
            SELECT 
                mc.id as main_id,
                mc.name as main_name,
                sc.id as sub_id,
                sc.name as sub_name,
                COALESCE(json_agg(
                    json_build_object(
                        'id', c.id,
                        'name', c.name
                    ) ORDER BY c.id
                ) FILTER (WHERE c.id IS NOT NULL), '[]'::json) as categories
            FROM main_categories mc
            LEFT JOIN main_category_sub_category mcsc ON mc.id = mcsc.main_category_id
            LEFT JOIN sub_categories sc ON mcsc.sub_category_id = sc.id
            LEFT JOIN categories c ON sc.id = c.sub_category_id
            GROUP BY mc.id, mc.name, sc.id, sc.name
        ),
        SubCategoryGroups AS (
            SELECT 
                main_id,
                main_name,
                json_agg(
                    json_build_object(
                        'id', sub_id,
                        'sub_category', sub_name,
                        'categories', categories
                    ) ORDER BY sub_id
                ) FILTER (WHERE sub_id IS NOT NULL) as sub_categories
            FROM CategoryGroups
            GROUP BY main_id, main_name
        )
        SELECT json_agg(
            json_build_object(
                'id', main_id,
                'main_category', main_name,
                'sub_categories', COALESCE(sub_categories, '[]'::json)
            ) ORDER BY main_id
        ) as all_categories
        FROM SubCategoryGroups`)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching categories: "+err.Error())
		return
	}
	defer rows.Close()

	var categoriesJSON []byte
	if rows.Next() {
		err := rows.Scan(&categoriesJSON)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning categories: "+err.Error())
			return
		}
	}

	// JSON'ı interface{}'e dönüştür
	var categories interface{}
	if categoriesJSON != nil {
		err = json.Unmarshal(categoriesJSON, &categories)
		if err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error parsing categories: "+err.Error())
			return
		}
	} else {
		categories = []interface{}{} // Boş dizi
	}

	utils.SendSuccess(w, "All categories fetched successfully", categories)
}

// Ana Kategori Silme
func DeleteMainCategory(w http.ResponseWriter, r *http.Request) {
	mainCategoryID := chi.URLParam(r, "mainId")

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// İlişkili alt kategorileri bul
	rows, err := tx.Query(context.Background(),
		"SELECT sub_category_id FROM main_category_sub_category WHERE main_category_id = $1",
		mainCategoryID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching related sub categories")
		return
	}
	defer rows.Close()

	// Alt kategorilerin ID'lerini topla
	var subCategoryIDs []string
	for rows.Next() {
		var subID string
		if err := rows.Scan(&subID); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning sub category ID")
			return
		}
		subCategoryIDs = append(subCategoryIDs, subID)
	}

	// Ana kategoriyi sil (cascade ile ilişkiler otomatik silinecek)
	result, err := tx.Exec(context.Background(),
		"DELETE FROM main_categories WHERE id = $1",
		mainCategoryID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error deleting main category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Main category not found")
		return
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "Main category and all related items deleted successfully", nil)
}

// Ana Kategori Güncelleme
func UpdateMainCategory(w http.ResponseWriter, r *http.Request) {
	mainCategoryID := chi.URLParam(r, "mainId")

	var request struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.Name == "" {
		utils.SendError(w, http.StatusBadRequest, "Name is required")
		return
	}

	pool := db.GetPool()
	result, err := pool.Exec(context.Background(),
		"UPDATE main_categories SET name = $1 WHERE id = $2",
		request.Name, mainCategoryID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating main category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Main category not found")
		return
	}

	utils.SendSuccess(w, "Main category updated successfully", request)
}

// Alt Kategori Silme
func DeleteSubCategory(w http.ResponseWriter, r *http.Request) {
	subCategoryID := chi.URLParam(r, "subId")

	pool := db.GetPool()
	tx, err := pool.Begin(context.Background())
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error starting transaction")
		return
	}
	defer tx.Rollback(context.Background())

	// İlişki tablosundan kaydı sil
	_, err = tx.Exec(context.Background(),
		"DELETE FROM main_category_sub_category WHERE sub_category_id = $1",
		subCategoryID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error deleting relation")
		return
	}

	// Alt kategoriyi sil (cascade ile kategoriler otomatik silinecek)
	result, err := tx.Exec(context.Background(),
		"DELETE FROM sub_categories WHERE id = $1",
		subCategoryID)
	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error deleting sub category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Sub category not found")
		return
	}

	if err := tx.Commit(context.Background()); err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error committing transaction")
		return
	}

	utils.SendSuccess(w, "Sub category and all related items deleted successfully", nil)
}

// Alt Kategori Güncelleme
func UpdateSubCategory(w http.ResponseWriter, r *http.Request) {
	subCategoryID := chi.URLParam(r, "subId")

	var request struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.Name == "" {
		utils.SendError(w, http.StatusBadRequest, "Name is required")
		return
	}

	pool := db.GetPool()
	result, err := pool.Exec(context.Background(),
		"UPDATE sub_categories SET name = $1 WHERE id = $2",
		request.Name, subCategoryID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating sub category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Sub category not found")
		return
	}

	utils.SendSuccess(w, "Sub category updated successfully", request)
}

// Kategori Silme
func DeleteCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "categoryId")
	subCategoryID := chi.URLParam(r, "subId")

	pool := db.GetPool()

	// Önce kategorinin belirtilen alt kategoriye ait olduğunu kontrol et
	var exists bool
	err := pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND sub_category_id = $2)",
		categoryID, subCategoryID).Scan(&exists)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error checking category")
		return
	}

	if !exists {
		utils.SendError(w, http.StatusNotFound, "Category not found in specified sub category")
		return
	}

	// Kategoriyi sil
	result, err := pool.Exec(context.Background(),
		"DELETE FROM categories WHERE id = $1 AND sub_category_id = $2",
		categoryID, subCategoryID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error deleting category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Category not found")
		return
	}

	utils.SendSuccess(w, "Category deleted successfully", nil)
}

// Kategori Güncelleme
func UpdateCategory(w http.ResponseWriter, r *http.Request) {
	categoryID := chi.URLParam(r, "categoryId")
	subCategoryID := chi.URLParam(r, "subId")

	var request struct {
		Name string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		utils.SendError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if request.Name == "" {
		utils.SendError(w, http.StatusBadRequest, "Name is required")
		return
	}

	pool := db.GetPool()

	// Önce kategorinin belirtilen alt kategoriye ait olduğunu kontrol et
	var exists bool
	err := pool.QueryRow(context.Background(),
		"SELECT EXISTS(SELECT 1 FROM categories WHERE id = $1 AND sub_category_id = $2)",
		categoryID, subCategoryID).Scan(&exists)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error checking category")
		return
	}

	if !exists {
		utils.SendError(w, http.StatusNotFound, "Category not found in specified sub category")
		return
	}

	// Kategoriyi güncelle
	result, err := pool.Exec(context.Background(),
		"UPDATE categories SET name = $1 WHERE id = $2 AND sub_category_id = $3",
		request.Name, categoryID, subCategoryID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error updating category")
		return
	}

	if rowsAffected := result.RowsAffected(); rowsAffected == 0 {
		utils.SendError(w, http.StatusNotFound, "Category not found")
		return
	}

	utils.SendSuccess(w, "Category updated successfully", request)
}

// Alt Kategorinin Kategorilerini Getir
func GetSubCategoryCategories(w http.ResponseWriter, r *http.Request) {
	subCategoryID := chi.URLParam(r, "subId")

	pool := db.GetPool()
	rows, err := pool.Query(context.Background(), `
        SELECT 
            c.id,
            c.name
        FROM categories c
        WHERE c.sub_category_id = $1
        ORDER BY c.id`, subCategoryID)

	if err != nil {
		utils.SendError(w, http.StatusInternalServerError, "Error fetching categories")
		return
	}
	defer rows.Close()

	var categories []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}

	for rows.Next() {
		var category struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
		}
		if err := rows.Scan(&category.ID, &category.Name); err != nil {
			utils.SendError(w, http.StatusInternalServerError, "Error scanning category")
			return
		}
		categories = append(categories, category)
	}

	utils.SendSuccess(w, "Categories fetched successfully", categories)
}
