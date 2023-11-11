package main

import (
	"archive/zip"
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
)

func main() {
	// Create 'dist' directory
	if _, err := os.Stat("dist"); os.IsNotExist(err) {
		err = os.Mkdir("dist", 0755)
		if err != nil {
			panic(err)
		}
	}

	// Create a zip file
	newZipFile, err := os.Create("dist/deploy.zip")
	if err != nil {
		panic(err)
	}
	defer newZipFile.Close()

	zipWriter := zip.NewWriter(newZipFile)

	log.Println("Adding files to zip")
	addFilesToZip(zipWriter, ".", "src", []string{"*"})
	addFilesToZip(zipWriter, ".venv/lib/python3.11/site-packages/", ".",
		[]string{"*"})
	zipWriter.Close()

	// Create a new S3 service client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("configuration error, " + err.Error())
	}
	client := s3.NewFromConfig(cfg)

	// Upload the file using the AWS SDK
	log.Println("Upload")
	_, err = client.PutObject(context.TODO(), &s3.PutObjectInput{
		Bucket: aws.String("rag-predev-serverlessdeploymentbucket-1o174lbx4zh68"),
		Key:    aws.String("deploy.zip"),
		// .... You will need to upload by chunks if the file is big....
	})
	if err != nil {
		panic(err)
	}

	// Deploy to the Lambda functions
	// In AWS lambda, you can simply update the function code to "deploy" your zip file

	log.Println("Deploy code")
	svc := lambda.New(session.New())
	input := &lambda.UpdateFunctionCodeInput{
		FunctionName: aws.String("rag-predev-ImportDocument"),
		S3Bucket:     aws.String("rag-predev-serverlessdeploymentbucket-1o174lbx4zh68"),
		S3Key:        aws.String("deploy.zip"),
	}

	_, err = svc.UpdateFunctionCode(input)

	// Do the same for the other function..

	if err != nil {
		panic(err)
	}
}

func addFilesToZip(zipWriter *zip.Writer, basePath string, relativePath string, filters []string) {
	filepath.Walk(filepath.Join(basePath, relativePath), func(path string, info os.FileInfo, err error) error {
		if info.Mode().IsRegular() && includeFile(filters, info.Name()) {

			archivePath := strings.TrimPrefix(path, basePath)
			f, err := zipWriter.Create(archivePath)
			if err != nil {
				log.Fatal(err)
			}

			fileData, err := os.Open(path)
			if err != nil {
				log.Fatal(err)
			}
			defer fileData.Close()

			_, err = io.Copy(f, fileData)
			if err != nil {
				log.Fatal(err)
			}
		}
		return nil
	})
}

// includeFile checks if a file should be included based on its name and a list of filters
func includeFile(filters []string, filePath string) bool {
	for _, filter := range filters {
		matched, _ := filepath.Match(filter, filePath)
		if matched {
			return true
		}
	}
	return false
}
