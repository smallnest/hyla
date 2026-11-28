package feishu

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"

	"hyla/internal/proto"
)

// BuildApprovalCardJSON builds a Feishu Schema 2.0 interactive card with an
// optional verification screenshot and optional Approve/Reject buttons when
// laneID is non-empty.
func BuildApprovalCardJSON(title, summary, laneID, imgKey, state string) (string, error) {
	color := cardState[state]
	if color == "" {
		if laneID != "" {
			color = "orange"
		} else {
			color = "green"
		}
	}
	var elems []any
	if summary != "" {
		elems = append(elems, map[string]any{
			"tag":     "markdown",
			"content": summary,
		})
	}
	if imgKey != "" {
		elems = append(elems, map[string]any{
			"tag":     "img",
			"img_key": imgKey,
			"alt":     map[string]any{"tag": "plain_text", "content": title},
		})
	}
	if laneID != "" {
		elems = append(elems, map[string]any{
			"tag":     "button",
			"type":    "primary",
			"text":    map[string]any{"tag": "plain_text", "content": "Approve & Requeue"},
			"value":   map[string]any{"hyla": "mq.approve", "lane": laneID},
		})
		elems = append(elems, map[string]any{
			"tag":   "button",
			"type":  "danger",
			"text":  map[string]any{"tag": "plain_text", "content": "Reject"},
			"value": map[string]any{"hyla": "mq.reject", "lane": laneID},
		})
	}
	card := map[string]any{
		"schema": "2.0",
		"config": map[string]any{"update_multi": true, "wide_screen_mode": true},
		"header": map[string]any{
			"title":    map[string]any{"tag": "plain_text", "content": title},
			"template": color,
		},
	}
	if len(elems) > 0 {
		card["body"] = map[string]any{"direction": "vertical", "elements": elems}
	}
	data, err := json.Marshal(card)
	return string(data), err
}

// UploadImage uploads a local PNG file to Feishu and returns its image_key.
func (b *Bot) UploadImage(pngPath string) (string, error) {
	if b == nil || b.lark == nil {
		return "", errors.New("feishu client not initialized")
	}
	f, err := os.Open(pngPath)
	if err != nil {
		return "", err
	}
	defer f.Close()
	req := larkim.NewCreateImageReqBuilder().
		Body(larkim.NewCreateImageReqBodyBuilder().
			ImageType("message").
			Image(f).
			Build()).
		Build()
	res, err := b.lark.Im.V1.Image.Create(context.Background(), req)
	if err != nil {
		return "", err
	}
	if !res.Success() {
		return "", fmt.Errorf("Feishu answered %d %s", res.Code, res.Msg)
	}
	if res.Data == nil || res.Data.ImageKey == nil {
		return "", errors.New("Feishu returned no image_key")
	}
	return *res.Data.ImageKey, nil
}

// SendApprovalCard uploads pngPath if provided and sends an interactive
// visual approval card to chatID.
func (b *Bot) SendApprovalCard(chatID, title, summary, laneID, pngPath string) (string, error) {
	if b == nil || b.msgs == nil {
		return "", errors.New("feishu bot not ready")
	}
	imgKey := ""
	if pngPath != "" {
		if k, err := b.UploadImage(pngPath); err == nil {
			imgKey = k
		} else {
			b.logf("feishu: upload image %s: %v", pngPath, err)
		}
	}
	state := "done"
	if laneID != "" {
		state = "asks"
	}
	card, err := BuildApprovalCardJSON(title, summary, laneID, imgKey, state)
	if err != nil {
		return "", err
	}
	return b.sendCard(chatID, "", card)
}

func (b *Bot) onMergeQueueAction(action string, e *callback.CardActionTriggerEvent) (*callback.CardActionTriggerResponse, error) {
	lane, _ := cardValue(e.Event.Action.Value, "lane")
	if lane == "" {
		return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "warning", Content: "Missing merge lane ID"}}, nil
	}
	if b.api == nil {
		return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "error", Content: "Daemon API unavailable"}}, nil
	}
	method := "mq.approve"
	msg := "Approved & requeued " + lane
	if action == "mq.reject" {
		method = "mq.reject"
		msg = "Rejected " + lane
	}
	if err := b.api.Call(method, proto.MergeQueueLaneParams{ID: lane}, nil); err != nil {
		return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "error", Content: err.Error()}}, nil
	}
	return &callback.CardActionTriggerResponse{Toast: &callback.Toast{Type: "success", Content: msg}}, nil
}
