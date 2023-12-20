package main

import (
	"archive/zip"
	"context"
	"errors"
	"flag"
	"io"
	"lapd"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/aws/aws-sdk-go-v2/service/cloudwatchlogs"
)

const (
	singleFileByteLimit = 107374182400 // 1 GB
	chunkSize           = 4096         // 4 KB
)

type BucketBasics struct {
	S3Client *s3.Client
}

func main() {
	// Define string flag for function
	functionName := flag.String("function", "", "Function name")
	purge := flag.Bool("purge", false, "Set to true to purge cloudwatch logs")

	// Parse the flags
	flag.Parse()

	// Adding a check if function flag is not provided
	if *functionName == "" {
		log.Fatal("please provide the function name using the -function  flag")
	}

	configuration := lapd.Config{}
	dependency_config, err := configuration.GetConfig()

	// Create 'dist' directory
	newDistDir(dependency_config.LocalPackageName)

	currentLambdaFunction := "default"
	if functionName != nil {
		currentLambdaFunction = *functionName
	}
	// Create a zip file
	newZipFile, err := os.Create(dependency_config.LocalPackageName)
	if err != nil {
		panic(err)
	}
	zipWriter := zip.NewWriter(newZipFile)
	if err != nil {
		panic("Invalid config file 'lapd.yml'")
	}
	log.Println("Adding files to zip")
	for _, function := range dependency_config.Functions {
		if function.Name == currentLambdaFunction {
			for _, filter := range function.Filters {
				addFilesToZip(zipWriter, filter)
				log.Println("Added: ", filter)
			}
		}
	}

	zipWriter.Close()
	newZipFile.Close()

	// Create a new S3 service client
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		panic("configuration error, " + err.Error())
	}
	s3Client := s3.NewFromConfig(cfg)

	// Upload the file using the AWS SDK
	log.Println("Upload")
	uploader := BucketBasics{}
	uploader.S3Client = s3Client

	uploader.UploadFile(
		dependency_config.S3Bucket,
		dependency_config.Package,
		dependency_config.LocalPackageName,
	)

	if err != nil {
		log.Fatalf("Failed to upload file to S3: %v\n", err)
	}

	// Deploy to the Lambda functions
	// In AWS lambda, you can simply update the function code to "deploy" your zip file

	log.Println("Deploy code")
	lambdaClient := lambda.NewFromConfig(cfg)

	cloudWatchLogsClient := cloudwatchlogs.NewFromConfig(cfg)
	log.Printf("Deploying function %s\n", *functionName)
	input := &lambda.UpdateFunctionCodeInput{
		FunctionName: functionName,
		S3Bucket:     aws.String(dependency_config.S3Bucket),
		S3Key:        aws.String(dependency_config.Package),
	}
	_, err = lambdaClient.UpdateFunctionCode(context.TODO(), input)

	if err != nil {
		panic(err)
	}

	if *purge {
		log.Println("Purge logs")
		// Get all log streams in the "function" log group
		streamInput := &cloudwatchlogs.DescribeLogStreamsInput{
			LogGroupName: aws.String("/aws/lambda/" + *functionName),
		}

		resp, err := cloudWatchLogsClient.DescribeLogStreams(context.Background(), streamInput)
		if err != nil {
			log.Println("Got error getting log streams:")
			log.Println(err)
			return
		}

		// Delete each log stream
		for _, stream := range resp.LogStreams {
			_, err := cloudWatchLogsClient.DeleteLogStream(context.Background(), &cloudwatchlogs.DeleteLogStreamInput{
				LogGroupName:  aws.String("/aws/lambda/" + *functionName),
				LogStreamName: stream.LogStreamName,
			})

			if err != nil {
				log.Println("Got error deleting log stream:")
				log.Println(err)
				return
			}

			log.Println("Deleted log stream: " + *stream.LogStreamName)

			log.Println("Successfully delete all log entries in all streams from function log group.")
		}
	}

}

// Create missing local directory
func newDistDir(filePath string) {
	dirPath := filepath.Dir(filePath)

	if _, err := os.Stat("dist"); os.IsNotExist(err) {
		err = os.Mkdir(dirPath, 0755)
		if err != nil {
			panic(err)
		}
	}
}

// Create zip to deploy
func addFilesToZip(zipWriter *zip.Writer, filter lapd.Filter) {
	basePath := filter.BasePath
	relativePath := filter.RelativePath
	include := filter.Include
	exclude := filter.Exclude
	filepath.Walk(filepath.Join(basePath, relativePath), func(path string, info os.FileInfo, err error) error {
		if info == nil {
			log.Printf("nil warning for path %v\n", path)
			return filepath.SkipDir
		}
		if info.IsDir() && info.Name() == "__pycache__" {
			return filepath.SkipDir
		}

		if info.Mode().IsRegular() &&
			includeFile(include, info.Name()) &&
			excludeFile(exclude, info.Name()) {

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

			err = copyContents(fileData, f)
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

func excludeFile(filters []string, filePath string) bool {
	for _, filter := range filters {
		matched, _ := filepath.Match(filter, filePath)
		if matched {
			return false
		}
	}
	return true
}

// copyContents copies the contents of the files to the zip using a buffer
func copyContents(r io.Reader, w io.Writer) error {
	var size int64
	b := make([]byte, chunkSize)
	for {
		// we limit the size to avoid zip bombs
		size += chunkSize
		if size > singleFileByteLimit {
			return errors.New("file too large, please contact us for assistance")
		}
		// read chunk into memory
		length, err := r.Read(b[:cap(b)])
		if err != nil {
			if err != io.EOF {
				return err
			}
			if length == 0 {
				break
			}
		}
		// write chunk to zip file
		_, err = w.Write(b[:length])
		if err != nil {
			return err
		}
	}
	return nil
}

// UploadFile uploads a file to an S3 bucket
func (basics BucketBasics) UploadFile(bucketName string, objectKey string, fileName string) error {
	file, err := os.Open(fileName)
	if err != nil {
		log.Printf("Couldn't open file %v to upload. Here's why: %v\n", fileName, err)
	} else {
		defer file.Close()
		_, err = basics.S3Client.PutObject(context.TODO(), &s3.PutObjectInput{
			Bucket: aws.String(bucketName),
			Key:    aws.String(objectKey),
			Body:   file,
		})
		if err != nil {
			log.Printf("Couldn't upload file %v to %v:%v. Here's why: %v\n",
				fileName, bucketName, objectKey, err)
		}
	}
	return err
}
