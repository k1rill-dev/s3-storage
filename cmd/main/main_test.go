package main

import (
	"bytes"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestUploadFile(t *testing.T) {
	// Создаем директорию storage перед тестом
	err := os.MkdirAll(storagePath, os.ModePerm)
	if err != nil {
		t.Fatalf("Не удалось создать директорию storage: %v", err)
	}
	// Удаляем директорию storage после теста
	defer os.RemoveAll(storagePath)

	// Создаем временный файл для загрузки
	tempFile, err := os.CreateTemp("", "testfile-*.jpg")
	if err != nil {
		t.Fatalf("Не удалось создать временный файл: %v", err)
	}
	defer os.Remove(tempFile.Name())

	// Записываем данные в временный файл
	_, err = tempFile.WriteString("фейковые данные файла")
	if err != nil {
		t.Fatalf("Не удалось записать данные во временный файл: %v", err)
	}
	tempFile.Close()

	// Создаем форму для загрузки файла
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(tempFile.Name()))
	if err != nil {
		t.Fatalf("Ошибка создания формы: %v", err)
	}

	file, err := os.Open(tempFile.Name())
	if err != nil {
		t.Fatalf("Ошибка открытия временного файла: %v", err)
	}
	defer file.Close()

	_, err = io.Copy(part, file)
	if err != nil {
		t.Fatalf("Ошибка копирования данных файла в форму: %v", err)
	}

	err = writer.Close()
	if err != nil {
		t.Fatalf("Ошибка закрытия writer: %v", err)
	}

	// Создаем тестовый HTTP-запрос
	req := httptest.NewRequest("POST", "/upload", body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// Создаем ResponseRecorder для записи ответа
	rr := httptest.NewRecorder()

	// Инициализация mtest для MongoDB
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))

	// Подменяем функцию connectMongoFunc заглушкой
	connectMongoFunc = func() (*mongo.Client, *mongo.Collection) {
		client := mt.Client // Используем mtest.Client для тестирования
		collection := client.Database("S3").Collection("files")
		return client, collection
	}

	// Вызов функции загрузки файла
	handler := http.HandlerFunc(uploadFile)
	handler.ServeHTTP(rr, req)

	// Проверяем код ответа
	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Ожидался код ответа %v, получен %v", http.StatusOK, status)
	}

	// Проверяем содержимое ответа
	if !strings.Contains(rr.Body.String(), "http://localhost:8080/files/") {
		t.Errorf("Ожидался URL загрузки файла, получен %v", rr.Body.String())
	}
}
