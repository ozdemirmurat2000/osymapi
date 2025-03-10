package utils

import (
	"encoding/json"
	"net/http"
	"osymapp/models"
)

func SendResponse(w http.ResponseWriter, statusCode int, success bool, message string, data interface{}, err string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	response := models.Response{
		Success: success,
		Message: message,
		Data:    data,
		Error:   err,
	}

	json.NewEncoder(w).Encode(response)
}

func SendSuccess(w http.ResponseWriter, message string, data interface{}) {
	SendResponse(w, http.StatusOK, true, message, data, "")
}

func SendError(w http.ResponseWriter, statusCode int, message string) {
	SendResponse(w, statusCode, false, "", nil, message)
}
