package main

import (
	"context"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	_ "time"
)

const storagePath = "./storage"

type FileInfo struct {
	ID        string `bson:"_id,omitempty"`
	FileName  string `bson:"filename"`
	UploadURL string `bson:"upload_url"`
}

func connectMongo() (*mongo.Client, *mongo.Collection) {
	connect, err := mongo.Connect(context.Background(), options.Client().
		ApplyURI("mongodb://user:password@mongodb:27017/"))
	if err != nil {
		panic(err)
	}
	err = connect.Ping(context.Background(), nil)
	if err != nil {
		panic(err)
	}
	client := connect.Database("S3").Collection("files")
	return connect, client
}

func generateFileName() string {
	return uuid.New().String()
}

func uploadFile(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		return
	}
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

	downloadURL := fmt.Sprintf("http://92.53.105.243:8081/files/%s", fileID)

	client, collection := connectMongo()
	defer client.Disconnect(context.TODO())

	fileInfo := FileInfo{
		ID:        fileID,
		FileName:  handler.Filename,
		UploadURL: downloadURL,
	}

	_, err = collection.InsertOne(context.TODO(), fileInfo)
	if err != nil {
		http.Error(w, "Не удалось сохранить информацию о файле в базу данных", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "%s", downloadURL)
}

func getFileLink(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	client, collection := connectMongo()
	defer client.Disconnect(context.TODO())

	var fileInfo FileInfo
	err := collection.FindOne(context.TODO(), bson.M{"_id": fileID}).Decode(&fileInfo)
	if err != nil {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	// Возвращаем ссылку на файл
	fmt.Fprintf(w, "Ссылка на файл: %s\n", fileInfo.UploadURL)
}

// Скачивание файла по ID
func downloadFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	filePath := filepath.Join(storagePath, fileID)

	// Проверяем, существует ли файл
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		http.Error(w, "Файл не найден", http.StatusNotFound)
		return
	}

	// Отправляем файл клиенту
	w.Header().Set("Content-Disposition", "attachment; filename="+fileID)
	w.Header().Set("Content-Type", "application/octet-stream")
	http.ServeFile(w, r, filePath)
}

// Удаление файла по ID из MongoDB и файловой системы
func deleteFile(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	fileID := vars["id"]

	client, collection := connectMongo()
	defer client.Disconnect(context.TODO())

	// Удаляем информацию о файле из MongoDB
	_, err := collection.DeleteOne(context.TODO(), bson.M{"_id": fileID})
	if err != nil {
		http.Error(w, "Ошибка при удалении информации о файле из базы данных", http.StatusInternalServerError)
		return
	}

	filePath := filepath.Join(storagePath, fileID)

	// Удаляем файл с файловой системы
	err = os.Remove(filePath)
	if err != nil {
		http.Error(w, "Ошибка при удалении файла", http.StatusInternalServerError)
		return
	}

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

	log.Println("Сервер запущен на порту :8081")
	err = http.ListenAndServe(":8081", r)
	if err != nil {
		return
	}
}
