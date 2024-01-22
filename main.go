package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

var collection *mongo.Collection

func init() {
	// Replace these values with your MongoDB connection details
	mongoURI := "mongodb+srv://guru:guru@banking.sy1piq8.mongodb.net/?retryWrites=true&w=majority"
	dbName := "Vedio_Streaming"
	collectionName := "fs.files"

	// Create a MongoDB client
	client, err := mongo.NewClient(options.Client().ApplyURI(mongoURI))
	if err != nil {
		log.Fatal(err)
	}

	// Connect to MongoDB
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Second)
	defer cancel()
	err = client.Connect(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// Set up the MongoDB collection for videos
	db := client.Database(dbName)
	collection = db.Collection(collectionName)

}

func main() {
	r := gin.Default()
	r.Static("/upload", "./frontend/upload")
	r.Static("/play", "./frontend/play/public")
	r.Static("/delete", "./frontend/delete")

	r.POST("/upload", func(c *gin.Context) {
		file, header, err := c.Request.FormFile("video")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		defer file.Close()

		videoID, err := saveVideo(file, header.Filename)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"video_id": videoID})
	})

	r.GET("/video/:id", func(c *gin.Context) {
		fmt.Println("called Here play")
		videoID := c.Param("id")
		video, err := getVideo(videoID)
		if err != nil {
			log.Println("Error retrieving video:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}

		c.Data(http.StatusOK, "video/mp4", video)
	})

	r.DELETE("/video/:id", func(c *gin.Context) {
		videoID := c.Param("id")
		err := deleteVideo(videoID)
		if err != nil {
			log.Println("Error deleting video:", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal Server Error"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"message": "Video deleted successfully"})
	})

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080" // Default to 8080 if PORT is not set
	}
	log.Fatal(r.Run("0.0.0.0:" + port))
}

func saveVideo(file io.Reader, filename string) (interface{}, error) {
	// Create a GridFS bucket
	bucket, err := gridfs.NewBucket(
		collection.Database(),
	)
	if err != nil {
		return "", err
	}

	// Open a new GridFS upload stream
	uploadStream, err := bucket.OpenUploadStream(filename)
	if err != nil {
		return "", err
	}
	defer uploadStream.Close()

	// Copy the video content to the GridFS stream
	_, err = io.Copy(uploadStream, file)
	if err != nil {
		return "", err
	}

	return uploadStream.FileID, nil
}

func getVideo(videoID string) ([]byte, error) {
	// Convert the video ID to an ObjectId
	objectID, err := primitive.ObjectIDFromHex(videoID)
	if err != nil {
		return nil, err
	}
	fmt.Println("Objectid :", objectID)

	// Find the document in fs.files by _id
	var fileInfo gridfs.File
	err = collection.FindOne(context.TODO(), bson.M{"_id": objectID}).Decode(&fileInfo)
	if err != nil {
		log.Println("Error finding file information:", err)
		return nil, err
	}

	// Create a GridFS bucket
	bucket, err := gridfs.NewBucket(
		collection.Database(),
	)
	if err != nil {
		return nil, err
	}

	// Open a GridFS download stream using the found file information
	downloadStream, err := bucket.OpenDownloadStreamByName(fileInfo.Name)
	if err != nil {
		log.Println("Error opening download stream:", err)
		return nil, err
	}
	defer downloadStream.Close()

	// Read the video content from the stream
	videoData, err := io.ReadAll(downloadStream)
	if err != nil {
		log.Println("Error reading video content:", err)
		return nil, err
	}

	return videoData, nil
}

func deleteVideo(videoID string) error {
	objectID, err := primitive.ObjectIDFromHex(videoID)
	if err != nil {
		return err
	}
	fmt.Println("Objectid :", objectID)
	// Create a GridFS bucket

	bucket, err := gridfs.NewBucket(
		collection.Database(),
	)
	if err != nil {
		return err
	}

	_, err = collection.DeleteOne(context.TODO(), bson.M{"_id": objectID})
	if err != nil {
		log.Println("Error finding file information:", err)
		return err
	}

	// Delete the video file from GridFS using its ID
	err = bucket.Delete(bson.M{"_id": objectID})
	if err != nil {
		return err
	}

	return nil
}
