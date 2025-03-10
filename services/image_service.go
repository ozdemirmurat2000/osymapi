package services

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

const (
	QuestionImagesPath = "images/questions"
	SolutionImagesPath = "images/solutions"
)

// Resim yükleme servisi
func UploadImage(file multipart.File, header *multipart.FileHeader, directory string) (string, error) {
	// Dizin yoksa oluştur
	if err := os.MkdirAll(directory, 0755); err != nil {
		return "", fmt.Errorf("dizin oluşturma hatası: %v", err)
	}

	// Benzersiz dosya adı oluştur
	extension := filepath.Ext(header.Filename)
	filename := fmt.Sprintf("%d%s", time.Now().UnixNano(), extension)
	fullPath := filepath.Join(directory, filename)

	// Dosyayı oluştur
	dst, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("dosya oluşturma hatası: %v", err)
	}
	defer dst.Close()

	// Dosyayı kopyala
	if _, err := io.Copy(dst, file); err != nil {
		return "", fmt.Errorf("dosya kopyalama hatası: %v", err)
	}

	// URL yolunu döndür
	return "/" + fullPath, nil
}
