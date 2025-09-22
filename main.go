package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"
)

// Character 定义
type Character struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Persona string `json:"persona"`
}

// 内存角色示例
var characters = []Character{
	{ID: "socrates", Name: "Socrates", Persona: "philosopher: ask probing questions."},
	{ID: "harry", Name: "Harry (inspired)", Persona: "wizard: curious and brave."},
}

// WebSocket 升级器（本地测试允许跨源）
var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

func main() {
	// 静态文件（前端）
	http.Handle("/", http.FileServer(http.Dir("./static")))

	// API: 列出角色
	http.HandleFunc("/api/characters", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(characters)
	})

	// WebSocket 聊天端点
	http.HandleFunc("/ws", wsHandler)

	log.Println("Server running at http://localhost:8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		log.Fatal(err)
	}
}

// wsHandler: 每个连接处理一个会话
func wsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "failed websocket upgrade", http.StatusBadRequest)
		return
	}
	defer conn.Close()

	// 通过查询参数选择角色
	charID := r.URL.Query().Get("character_id")
	char := findCharacter(charID)
	if char == nil {
		// 默认第一个
		char = &characters[0]
	}

	// 读/写循环
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			log.Println("read error:", err)
			return
		}
		// 前端消息格式 { type: "user_message", text: "..." }
		var clientMsg struct {
			Type string `json:"type"`
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &clientMsg); err != nil {
			log.Println("invalid client message:", err)
			continue
		}

		if clientMsg.Type == "user_message" {
			userText := clientMsg.Text
			reply := generateReply(*char, userText)

			// 发送 assistant 消息回前端
			serverMsg := map[string]string{
				"type": "assistant_message",
				"role": "assistant",
				"text": reply,
			}
			if err := conn.WriteJSON(serverMsg); err != nil {
				log.Println("write error:", err)
				return
			}
		}
	}
}

func findCharacter(id string) *Character {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil
	}
	for _, c := range characters {
		if c.ID == id {
			return &c
		}
	}
	return nil
}

// 一个非常简单的 "mock LLM"：根据 persona 做小变换
func generateReply(ch Character, userText string) string {
	userText = strings.TrimSpace(userText)
	if userText == "" {
		return fmt.Sprintf("%s: ...", ch.Name)
	}
	p := strings.ToLower(ch.Persona)
	if strings.Contains(p, "philosopher") {
		return fmt.Sprintf("%s: Why do you say \"%s\"?", ch.Name, userText)
	}
	if strings.Contains(p, "wizard") {
		return fmt.Sprintf("%s: That sounds magical — tell me more about \"%s\".", ch.Name, userText)
	}
	// 默认 echo 风格
	return fmt.Sprintf("%s: %s", ch.Name, userText)
}
s