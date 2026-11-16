package handler

import (
	"encoding/json"
	"net/http"

	"com.xsguyu.accounting/backend/internal/model"
)

func sendJSON(w http.ResponseWriter, code int, statusCode int, message string, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(statusCode)
	resp := model.ApiResponse{
		Code:    code,
		Message: message,
		Data:    data,
	}
	_ = json.NewEncoder(w).Encode(resp)
}

func sendError(w http.ResponseWriter, statusCode int, message string) {
	sendJSON(w, statusCode, statusCode, message, nil)
}

func sendBytes(w http.ResponseWriter, data []byte, contentType string, headers map[string]string, statusCode int) {
	w.Header().Set("Content-Type", contentType)
	for k, v := range headers {
		w.Header().Set(k, v)
	}
	w.WriteHeader(statusCode)
	_, _ = w.Write(data)
}
