package models

type MainCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type SubCategory struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Category struct {
	ID            int    `json:"id"`
	SubCategoryID int    `json:"sub_category_id"`
	Name          string `json:"name"`
}

type MainCategorySubCategory struct {
	MainCategoryID int `json:"main_category_id"`
	SubCategoryID  int `json:"sub_category_id"`
}

type CategoryRequest struct {
	MainCategory  string             `json:"main_category"`
	SubCategories []SubCategoryGroup `json:"sub_categories"`
}

type SubCategoryGroup struct {
	SubCategory string   `json:"sub_category"`
	Categories  []string `json:"categories"`
}
