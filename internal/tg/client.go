// Package tg implements a Telegram Bot API client (long polling, no SDK)
// and the SecretDock bot service (add/query flows).
package tg

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ---- Bot API types ----

type Update struct {
	UpdateID      int64         `json:"update_id"`
	Message       *Message      `json:"message"`
	CallbackQuery *CallbackQuery `json:"callback_query"`
}

type Message struct {
	MessageID int64       `json:"message_id"`
	Chat      Chat        `json:"chat"`
	Text      string      `json:"text"`
	Caption   string      `json:"caption"`
	Photo     []PhotoSize `json:"photo"`
	Document  *Document   `json:"document"`
}

type Document struct {
	FileID   string `json:"file_id"`
	FileName string `json:"file_name"`
	MimeType string `json:"mime_type"`
	FileSize int64  `json:"file_size"`
}

type PhotoSize struct {
	FileID   string `json:"file_id"`
	Width    int    `json:"width"`
	Height   int    `json:"height"`
	FileSize int64  `json:"file_size"`
}

type Chat struct {
	ID int64 `json:"id"`
}

type CallbackQuery struct {
	ID      string   `json:"id"`
	Message *Message `json:"message"`
	Data    string   `json:"data"`
}

type InlineKeyboardButton struct {
	Text         string `json:"text"`
	CallbackData string `json:"callback_data"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

// BtnRow builds one keyboard row.
func BtnRow(btns ...InlineKeyboardButton) []InlineKeyboardButton { return btns }

// Btn is a shorthand for a callback button.
func Btn(text, data string) InlineKeyboardButton {
	return InlineKeyboardButton{Text: text, CallbackData: data}
}

// ---- Client ----

// Client is a minimal Bot API client.
type Client struct {
	token string
	http  *http.Client
}

// NewClient creates a client for the given bot token.
func NewClient(token string) *Client {
	return &Client{
		token: token,
		http:  &http.Client{Timeout: 70 * time.Second},
	}
}

func (c *Client) call(method string, payload interface{}) ([]byte, error) {
	var body io.Reader
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequest("POST", "https://api.telegram.org/bot"+c.token+"/"+method, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	var env struct {
		OK          bool            `json:"ok"`
		Result      json.RawMessage `json:"result"`
		Description string          `json:"description"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		return nil, err
	}
	if !env.OK {
		return nil, fmt.Errorf("tg: %s: %s", method, env.Description)
	}
	return env.Result, nil
}

// SetMyCommands installs the bot command menu.
func (c *Client) SetMyCommands(cmds []map[string]string) error {
	_, err := c.call("setMyCommands", map[string]interface{}{"commands": cmds})
	return err
}

// GetUpdates long-polls for updates.
func (c *Client) GetUpdates(offset int64, timeoutSec int) ([]Update, error) {
	raw, err := c.call("getUpdates", map[string]interface{}{
		"offset":          offset,
		"timeout":         timeoutSec,
		"allowed_updates": []string{"message", "callback_query"},
	})
	if err != nil {
		return nil, err
	}
	var updates []Update
	if err := json.Unmarshal(raw, &updates); err != nil {
		return nil, err
	}
	return updates, nil
}

// SendMessage sends text; returns the new message id.
func (c *Client) SendMessage(chatID int64, text, parseMode string, markup *InlineKeyboardMarkup) (int64, error) {
	p := map[string]interface{}{
		"chat_id":                chatID,
		"text":                   text,
		"disable_web_page_preview": true,
	}
	if parseMode != "" {
		p["parse_mode"] = parseMode
	}
	if markup != nil {
		p["reply_markup"] = markup
	}
	raw, err := c.call("sendMessage", p)
	if err != nil {
		return 0, err
	}
	var m Message
	if err := json.Unmarshal(raw, &m); err != nil {
		return 0, err
	}
	return m.MessageID, nil
}

// DeleteMessage deletes a message; errors are returned to the caller
// (callers usually ignore "already deleted").
func (c *Client) DeleteMessage(chatID, messageID int64) error {
	_, err := c.call("deleteMessage", map[string]interface{}{
		"chat_id":    chatID,
		"message_id": messageID,
	})
	return err
}

// AnswerCallbackQuery acknowledges a button tap.
func (c *Client) AnswerCallbackQuery(id, text string) {
	_, _ = c.call("answerCallbackQuery", map[string]interface{}{
		"callback_query_id": id,
		"text":              text,
	})
}

// GetMe verifies the token and returns the bot username.
// GetFile returns the file_path for a file_id.
func (c *Client) GetFile(fileID string) (string, error) {
	raw, err := c.call("getFile", map[string]interface{}{"file_id": fileID})
	if err != nil {
		return "", err
	}
	var res struct {
		FileID   string `json:"file_id"`
		FilePath string `json:"file_path"`
	}
	if err := json.Unmarshal(raw, &res); err != nil {
		return "", err
	}
	if res.FilePath == "" {
		return "", fmt.Errorf("getFile failed")
	}
	return res.FilePath, nil
}

// DownloadFile downloads a file by its file_path.
func (c *Client) DownloadFile(filePath string) ([]byte, error) {
	u := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", c.token, filePath)
	resp, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("download status %d", resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func (c *Client) GetMe() (string, error) {
	raw, err := c.call("getMe", nil)
	if err != nil {
		return "", err
	}
	var me struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(raw, &me); err != nil {
		return "", err
	}
	return me.Username, nil
}
