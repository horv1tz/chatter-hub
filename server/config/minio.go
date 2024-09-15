package config

import (
    "log"

    "github.com/minio/minio-go/v7"
    "github.com/minio/minio-go/v7/pkg/credentials"
)

var MinioClient *minio.Client

// InitMinio инициализирует соединение с MinIO
func InitMinio(cfg *Config) {
    var err error
    MinioClient, err = minio.New(cfg.Minio.Endpoint, &minio.Options{
        Creds:  credentials.NewStaticV4(cfg.Minio.AccessKey, cfg.Minio.SecretKey, ""),
        Secure: cfg.Minio.UseSSL,
    })
    if err != nil {
        log.Fatalf("Ошибка подключения к MinIO: %v", err)
    }

    // Создаем бакет, если он не существует
    bucketName := "voice-messages"
    location := "us-east-1"

    exists, err := MinioClient.BucketExists(Ctx, bucketName)
    if err != nil {
        minioErr, ok := err.(minio.ErrorResponse)
        if !ok || minioErr.Code != "NoSuchBucket" {
            log.Fatalf("Ошибка проверки существования бакета: %v", err)
        }
    }
    if !exists {
        err = MinioClient.MakeBucket(Ctx, bucketName, minio.MakeBucketOptions{Region: location})
        if err != nil {
            log.Fatalf("Ошибка создания бакета: %v", err)
        }
        log.Printf("Бакет %s успешно создан", bucketName)
    }
}
