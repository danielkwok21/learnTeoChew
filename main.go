package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	_ "github.com/mattn/go-sqlite3"
)

type Database struct {
	Conversations []Conversation `json:"conversations"`
}

type Conversation struct {
	ID       int    `json:"id"`
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

type TermsMode string

const (
	TermsModeFrontToBack TermsMode = "f2b"
	TermsModeBackToFront TermsMode = "b2f"
)

func main() {
	// Create Gin router
	r := gin.Default()

	r.LoadHTMLGlob("templates/*")

	// Serve static files (CSS, JS, images)
	r.Static("/static", "./static")

	// Serve MP3 files from audio directory
	r.Static("/audio", "./audio")

	// SQLite connection
	log.Println("Connecting to SQLite database...")
	db, dbErr := sql.Open("sqlite3", "./db/learn_teochew.db")
	if dbErr != nil {
		log.Fatal(dbErr)
	}
	defer db.Close()

	bytes, bytesErr := os.ReadFile("database.json")
	if bytesErr != nil {
		fmt.Println("Error at reading database.json: ", bytesErr)
		panic(bytesErr)
	}
	database := &Database{}
	jsonErr := json.Unmarshal(bytes, database)
	if jsonErr != nil {
		fmt.Println("Error at unmarshalling database.json: ", jsonErr)
		panic(jsonErr)
	}

	// cache
	type Card struct {
		ID    int
		Front string
		Back  string
		Ease  *int
	}
	var cards []Card
	rows, err := db.Query("SELECT id, front, back, ease from cards ORDER BY ease ASC")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var card Card
		if err := rows.Scan(&card.ID, &card.Front, &card.Back, &card.Ease); err != nil {
			log.Fatal(err)
		}
		cards = append(cards, card)
	}

	r.GET("/", func(c *gin.Context) {
		type PageData struct {
			Conversations []Conversation
		}

		// sort descending
		sort.Slice(database.Conversations, func(i, j int) bool {
			return database.Conversations[i].ID > database.Conversations[j].ID
		})

		pageData := PageData{
			Conversations: database.Conversations,
		}

		c.HTML(http.StatusOK, "index.html", pageData)
	})

	r.GET("/terms/:mode/:id/front", func(c *gin.Context) {
		id := c.Param("id")

		index, err := strconv.Atoi(id)
		if err != nil {
			index = 1
		}

		if index >= len(cards) {
			c.String(http.StatusBadRequest, "Invalid id: %d", index)
			return
		}
		card := cards[index]

		mode := c.Param("mode")
		term := ""
		switch mode {
		case string(TermsModeFrontToBack):
			term = card.Front
		case string(TermsModeBackToFront):
			term = card.Back
		default:
			c.String(http.StatusBadRequest, "Invalid mode: %s", mode)
			return
		}

		c.HTML(http.StatusOK, "termsFront.html", gin.H{
			"Front": term,
			"ID":    index,
			"Mode":  mode,
		})
	})

	r.GET("/terms/:mode/:id/back", func(c *gin.Context) {
		id := c.Param("id")

		index, err := strconv.Atoi(id)
		if err != nil {
			index = 1
		}

		if index > len(cards) {
			c.String(http.StatusBadRequest, "Invalid id: %d", index)
			return
		}
		card := cards[index]

		mode := c.Param("mode")
		term := ""
		switch mode {
		case string(TermsModeFrontToBack):
			term = card.Back
		case string(TermsModeBackToFront):
			term = card.Front
		default:
			c.String(http.StatusBadRequest, "Invalid mode: %s", mode)
			return
		}

		c.HTML(http.StatusOK, "termsBack.html", gin.H{
			"Back": term,
			"ID":   index,
			"Mode": mode,
		})
	})

	r.GET("/terms/:mode/:id/ease", func(c *gin.Context) {
		id := c.Param("id")

		index, err := strconv.Atoi(id)
		if err != nil {
			index = 1
		}

		if index >= len(cards) {
			c.String(http.StatusBadRequest, "Invalid id: %d", index)
			return
		}

		card := cards[index]

		mode := c.Param("mode")
		front := ""
		back := ""
		switch mode {
		case string(TermsModeFrontToBack):
			front = card.Front
			back = card.Back
		case string(TermsModeBackToFront):
			front = card.Back
			back = card.Front
		default:
			c.String(http.StatusBadRequest, "Invalid mode: %s", mode)
			return
		}

		c.HTML(http.StatusOK, "termsEase.html", gin.H{
			"Front": front,
			"Back":  back,
			"ID":    index,
			"Mode":  mode,
		})
	})

	r.GET("/terms", func(c *gin.Context) {

		row := db.QueryRow("SELECT AVG(ease) FROM cards WHERE ease IS NOT NULL")
		var medianScore float64
		_ = row.Scan(&medianScore)

		c.HTML(http.StatusOK, "terms.html", gin.H{
			"Median": medianScore,
		})
	})

	r.POST("/terms/:id/ease", func(c *gin.Context) {
		id := c.Param("id")

		id2, err := strconv.Atoi(id)
		if err != nil {
			c.String(http.StatusBadRequest, "invalid id=", id)
			return
		}
		if id2 >= len(cards) {
			c.String(http.StatusBadRequest, "invalid id=", id)
			return
		}

		ease := c.PostForm("ease")
		ease2, err := strconv.Atoi(ease)
		if err != nil || ease2 < 1 || ease2 > 4 {
			c.String(http.StatusBadRequest, "invalid ease=", ease)
			return
		}

		_, err = db.Exec("UPDATE cards SET ease = ? WHERE id = ?", ease2, cards[id2].ID)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error: %v", err)
			return
		}

		var nextCard Card
		err = db.QueryRow("SELECT id, ease from cards ORDER BY ease DESC LIMIT 1").Scan(&nextCard.ID, &nextCard.Ease)
		if err != nil {
			log.Fatal(err)
		}

		// i.e. even the most difficult card is now very easy
		// redirect to home page
		if *nextCard.Ease == 1 {
			c.Redirect(http.StatusFound, "/terms")
			return
		}

		for i, card := range cards {
			if card.ID == nextCard.ID {
				c.Redirect(http.StatusFound, fmt.Sprintf("/terms/%d/front", i))
				return
			}
		}
		c.String(http.StatusInternalServerError, "unable to find next card of id: %d", nextCard.ID)
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
