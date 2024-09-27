package main

import (
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	_ "time"
)

const storagePath = "./storage"

type FileInfo struct {
	ID        string
	FileName  string
	UploadURL string
}

var fileStorage = map[string]FileInfo{}

func generateFileName() string {
	return uuid.New().String()
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return
	}
	fmt.Println(r.MultipartForm)
	file, handler, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "Не удалось получить файл", http.StatusBadRequest)
		return
	}
	defer file.Close()

	fileID := fmt.Sprintf("%s.jpg", generateFileName())
	filePath := filepath.Join(storagePath, fileID)

	dst, err := os.Create(filePath)
	if err != nil {
		http.Error(w, "Не удалось создать файл на сервере", http.StatusInternalServerError)
		return
	}
	defer dst.Close()

	if _, err = io.Copy(dst, file); err != nil {
		http.Error(w, "Ошибка при копировании файла", http.StatusInternalServerError)
		return
	}

	downloadURL := fmt.Sprintf("http://localhost:8080/files/%s", fileID)

	fileStorage[fileID] = FileInfo{
		ID:        fileID,
		FileName:  handler.Filename,
		UploadURL: downloadURL,
	}

	fmt.Fprintf(w, "Файл загружен: %s\n", downloadURL)
}

func getFileLink(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	fileInfo, exists := fileStorage[fileID]
	if !exists {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	fmt.Fprintf(w, "Ссылка на файл: %s\n", fileInfo.UploadURL)
}

func downloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	filePath := filepath.Join(storagePath, fileID)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename="+fileID)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

// Удаление фото по ID
func deleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	filePath := filepath.Join(storagePath, fileID)

	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	err := os.Remove(filePath)
	if err != nil {
		http.Error(w, "Ошибка при удалении файла", http.StatusInternalServerError)
		return
	}

	delete(fileStorage, fileID)

	fmt.Fprintf(w, "Файл удален: %s\n", fileID)
}

func main() {
	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		log.Fatalf("Ошибка создания директории для хранения файлов: %v", err)
	}

	r := mux.NewRouter()
	r.PathPrefix("/files/").Handler(http.StripPrefix("/files/", http.FileServer(http.Dir("storage"))))
	r.HandleFunc("/upload", uploadFile).Methods("POST")
	r.HandleFunc("/files/{id}", downloadFile).Methods("GET")
	r.HandleFunc("/link/{id}", getFileLink).Methods("GET")
	r.HandleFunc("/files/{id}", deleteFile).Methods("DELETE")

	log.Println("Сервер запущен на порту :8080")
	err = http.ListenAndServe(":8080", r)
	if err != nil {
		return
	}
}
