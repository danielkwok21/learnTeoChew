package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

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

type Card struct {
	ID    int
	Front string
	Back  string
	Ease  *Ease
}

type Ease int

const (
	VeryHard Ease = 1
	Hard     Ease = 2
	Good     Ease = 3
	Easy     Ease = 4
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
	var cards []Card
	rows, err := db.Query("SELECT id, front, back, ease from cards ORDER BY RANDOM()")
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
		row := db.QueryRow("SELECT SUM(ease) FROM cards WHERE ease IS NOT NULL")
		var medianScore int
		_ = row.Scan(&medianScore)

		row = db.QueryRow("SELECT count(*) FROM cards")
		var totalNumberOfCards int
		_ = row.Scan(&totalNumberOfCards)

		c.HTML(http.StatusOK, "terms.html", gin.H{
			"Score": fmt.Sprintf("%d/%d", medianScore, totalNumberOfCards*int(Easy)),
		})
	})

	r.POST("/terms/:mode/:id/ease", func(c *gin.Context) {
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
		if err != nil || ease2 < int(VeryHard) || ease2 > int(Easy) {
			c.String(http.StatusBadRequest, "invalid ease=", ease)
			return
		}

		mode := c.Param("mode")
		switch mode {
		case string(TermsModeFrontToBack), string(TermsModeBackToFront):
			// do nothing
		default:
			c.String(http.StatusBadRequest, "Invalid mode: %s", mode)
			return
		}

		_, err = db.Exec("UPDATE cards SET ease = ? WHERE id = ?", ease2, cards[id2].ID)
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error: %v", err)
			return
		}

		var nextHardCards []Card
		rows, err := db.Query("SELECT id, ease FROM cards WHERE ease ORDER BY ease ASC LIMIT 3")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var card Card
			if err := rows.Scan(&card.ID, &card.Ease); err != nil {
				log.Fatal(err)
			}
			nextHardCards = append(nextHardCards, card)
		}
		if len(nextHardCards) == 0 {
			log.Println("no more hard cards found, redirect to home page")
			c.Redirect(http.StatusFound, "/terms")
			return
		}
		fmt.Println(nextHardCards)
		randIndex := rand.Intn(len(nextHardCards))
		nextCard := nextHardCards[randIndex]

		// i.e. even the most difficult card is now very easy
		// redirect to home page
		if nextCard.Ease != nil && *nextCard.Ease == Easy {
			log.Println("all cards are easy now, redirect to home page")
			c.Redirect(http.StatusFound, "/terms")
			return
		}

		for index, card := range cards {
			if card.ID == nextCard.ID {
				c.Redirect(http.StatusFound, fmt.Sprintf("/terms/%s/%d/front", mode, index))
				return
			}
		}
		c.String(http.StatusInternalServerError, "unable to find next card of id: %d", nextCard.ID)
	})

	r.POST("/terms/reset", func(c *gin.Context) {
		_, err := db.Exec("UPDATE cards SET ease = 1")
		if err != nil {
			c.String(http.StatusInternalServerError, "DB error: %v", err)
			return
		}
		for i := range cards {
			cards[i].Ease = nil
		}
		c.Redirect(http.StatusFound, "/terms")
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

	r.POST("/answer", func(c *gin.Context) {
		index := c.PostForm("index")
		if index == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "index required"})
			return
		}

		index2, err := strconv.Atoi(index)
		if err != nil || index2 < 0 || index2 >= len(cards) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid index"})
			return
		}

		file, err := c.FormFile("file")
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "file required"})
			return
		}

		savePath := "./" + file.Filename
		if err := c.SaveUploadedFile(file, savePath); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot save file"})
			return
		}
		//defer os.Remove(savePath)

		card := cards[index2]

		// Call Whisper API directly
		spoken, err := transcribeWithWhisper(savePath)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		expected := card.Back
		isMatch := strings.TrimSpace(spoken) == expected

		c.JSON(http.StatusOK, gin.H{
			"spoken":   spoken,
			"expected": expected,
			"isMatch":  isMatch,
		})
	})

	// Health check endpoint
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, nil)
	})

	r.Run(":8085")
}

func transcribeWithWhisper(filePath string) (string, error) {
	type WhisperResp struct {
		Text string `json:"text"`
	}
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", fmt.Errorf("OPENAI_API_KEY not set")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	var b bytes.Buffer
	w := multipart.NewWriter(&b)

	// File field
	fw, err := w.CreateFormFile("file", filePath)
	if err != nil {
		return "", err
	}
	if _, err = io.Copy(fw, file); err != nil {
		return "", err
	}

	// Model field
	if err = w.WriteField("model", "whisper-1"); err != nil {
		return "", err
	}

	// Language field
	if err = w.WriteField("language", "zh"); err != nil {
		return "", err
	}

	w.Close()

	req, err := http.NewRequest("POST", "https://api.openai.com/v1/audio/transcriptions", &b)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", w.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper API error: %s", string(body))
	}

	var wr WhisperResp
	if err := json.NewDecoder(resp.Body).Decode(&wr); err != nil {
		return "", err
	}

	fmt.Println(wr)

	return wr.Text, nil
}
