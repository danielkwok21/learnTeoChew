package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Database struct {
	Conversations []Conversation `json:"conversations"`
}

type Conversation struct {
	ID       int    `json:"id"`
	Title    string `json:"title"`
	Subtitle string `json:"subtitle"`
	Audio    string `json:"audio"`
	Lines    []Line `json:"lines"`
}

type Line struct {
	Timestamp string `json:"timestamp"`
	Chinese   string `json:"chinese"`
	Teochew   string `json:"teochew"`
	English   string `json:"english"`
	Pinyin    string `json:"pinyin"`
}

func main() {
	// Create Gin router
	r := gin.Default()

	// Load HTML templates
	r.LoadHTMLGlob("templates/*")

	// Serve static files (CSS, JS, images)
	r.Static("/static", "./static")

	// Serve MP3 files from audio directory
	r.Static("/audio", "./audio")

	bytes, err := os.ReadFile("database.json")
	if err != nil {
		fmt.Println("Error at reading database.json: ", err)
		panic(err)
	}
	database := &Database{}
	err = json.Unmarshal(bytes, database)
	if err != nil {
		fmt.Println("Error at unmarshalling database.json: ", err)
		panic(err)
	}

	r.GET("/", func(c *gin.Context) {
		type PageData struct {
			Conversations []Conversation
		}

		pageData := PageData{
			Conversations: database.Conversations,
		}

		c.HTML(http.StatusOK, "index.html", pageData)
	})

	r.GET("/conversation/:ID", func(c *gin.Context) {
		_id := c.Param("ID")
		id, err := strconv.Atoi(_id)
		if err != nil {
			c.HTML(http.StatusOK, "error.html", gin.H{
				"message": fmt.Sprintf("No file found with id=%s", _id),
			})
			return
		}

		// Get conversation data
		var conversation *Conversation

		for _, c := range database.Conversations {
			if c.ID == id {
				conversation = &c
				break
			}
		}

		fmt.Printf("%+v", conversation)

		// Render template with data
		c.HTML(http.StatusOK, "conversation.html", gin.H{
			"conversation": conversation,
		})
	})
	r.POST("/conversation/:direction/:ID", func(c *gin.Context) {
		direction := c.Param("direction")
		if direction != "prev" && direction != "next" {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid direction parameter. Use 'prev' or 'next'.",
			})
			return
		}

		_id := c.Param("ID")
		id, err := strconv.Atoi(_id)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid conversation ID",
			})
			return
		}

		var targetConversation *Conversation
		for i, conv := range database.Conversations {
			if conv.ID == id {
				if direction == "prev" && i > 0 {
					targetConversation = &database.Conversations[i-1]
				} else if direction == "next" && i < len(database.Conversations)-1 {
					targetConversation = &database.Conversations[i+1]
				}
				break
			}
		}

		if targetConversation == nil {
			c.Redirect(http.StatusFound, fmt.Sprintf("/conversation/%d", id))
			return
		}

		c.Redirect(http.StatusFound, fmt.Sprintf("/conversation/%d", targetConversation.ID))
	})

	// API endpoint to get available audio files
	r.GET("/api/audio-files", func(c *gin.Context) {
		files, err := filepath.Glob("./audio/*.mp3")
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read audio files"})
			return
		}

		// Extract just filenames
		var audioFiles []string
		for _, file := range files {
			audioFiles = append(audioFiles, filepath.Base(file))
		}

		c.JSON(http.StatusOK, gin.H{
			"files": audioFiles,
		})
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	r.Run(":8085")
}
