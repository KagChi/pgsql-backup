package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	"github.com/go-co-op/gocron"
	"github.com/joho/godotenv"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func GenerateRandomHex(length int) (string, error) {
	if length%2 != 0 {
		return "", fmt.Errorf("length must be an even number")
	}

	bytes := make([]byte, length/2)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(bytes), nil
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	logger := log.New(os.Stdout, "", log.Ldate|log.Ltime)

	endpoint := os.Getenv("S3_ENDPOINT")
	accessKeyID := os.Getenv("S3_ACCESS")
	secretAccessKey := os.Getenv("S3_SECRET")
	useSSL := os.Getenv("S3_USE_SSL") == "true"

	client, err := minio.New(endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(accessKeyID, secretAccessKey, ""),
		Secure: useSSL,
	})

	if err != nil {
		log.Fatalln(err)
	}

	c := gocron.NewScheduler(time.UTC)

	Cron := os.Getenv("CRON_JOB")

	logger.Printf("pgsql will be dumped every %s", Cron)

	c.Cron(Cron).Do(
		func() {
			logger.Println("Starting to dump pgsql...")
			cmd := "pg_dump"
			args := []string{"-U", os.Getenv("PGUSERNAME"), "-h", os.Getenv("PGHOST"), "-p", os.Getenv("PGPORT")}

			out, err := exec.Command(cmd, args...).Output()
			if err != nil {
				log.Println("Error executing command:", err)
				return
			}

			zipBuffer := new(bytes.Buffer)
			zipWriter := zip.NewWriter(zipBuffer)

			zipEntry, err := zipWriter.Create("backup.sql")
			if err != nil {
				log.Printf("Error creating zip entry: %v\n", err)
				return
			}

			_, err = zipEntry.Write(out)
			if err != nil {
				log.Printf("Error writing to zip entry: %v\n", err)
				return
			}

			logger.Println("pgsql dumped successfully...")

			zipWriter.Close()
			zipWriter.Flush()

			randHex, err := GenerateRandomHex(32)

			if err != nil {
				log.Printf("Error creating hex: %v\n", err)
				return
			}

			currentTime := time.Now()
			year, month, day := currentTime.Date()
			date := fmt.Sprintf("%d-%02d-%02d", year, month, day)
			fileName := fmt.Sprintf("Database/%d/%d/%d/%s (%s).zip", year, month, day, date, randHex)

			log.Printf("File name: %s", fileName)

			_, err = client.PutObject(context.Background(), os.Getenv("S3_BUCKET"), fileName, bytes.NewReader(zipBuffer.Bytes()), int64(zipBuffer.Len()), minio.PutObjectOptions{ContentType: "application/zip"})
			if err != nil {
				log.Printf("Error uploading file to S3: %v\n", err)
				return
			}

			log.Printf("Uploaded the file to S3")
		},
	)

	c.StartAsync()

	select {}
}
